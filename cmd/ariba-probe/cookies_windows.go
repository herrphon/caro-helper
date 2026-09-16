//go:build windows

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	_ "modernc.org/sqlite"

	"golang.org/x/sys/windows"
)

type cookie struct {
	Name  string
	Value string
}

// readEdgeCookies decrypts the cookies Edge holds for hosts under realmHost.
//
// Edge (Chromium) stores cookies in a SQLite DB, each value encrypted with
// AES-256-GCM. The AES key lives in "Local State" as base64("DPAPI"+blob),
// where blob is DPAPI-encrypted under the current Windows user. Newer builds
// may wrap it with App-Bound Encryption ("APPB" prefix) instead - we detect
// that and fail loudly rather than return garbage.
func readEdgeCookies(profile, realmHost string) ([]cookie, error) {
	base, err := edgeUserDataDir()
	if err != nil {
		return nil, err
	}

	key, err := masterKey(filepath.Join(base, "Local State"))
	if err != nil {
		return nil, err
	}

	cookiesDB := filepath.Join(base, profile, "Network", "Cookies")
	if _, err := os.Stat(cookiesDB); err != nil {
		return nil, fmt.Errorf("cookies DB not found (%s): %w", cookiesDB, err)
	}

	// The DB is usually locked while Edge is running; copy it and open read-only.
	tmp, cleanup, err := copyToTemp(cookiesDB)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	db, err := sql.Open("sqlite", "file:"+tmp+"?mode=ro&immutable=1")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name, host_key, encrypted_value FROM cookies`)
	if err != nil {
		return nil, fmt.Errorf("query cookies (schema change?): %w", err)
	}
	defer rows.Close()

	var out []cookie
	for rows.Next() {
		var name, host string
		var enc []byte
		if err := rows.Scan(&name, &host, &enc); err != nil {
			return nil, err
		}
		if !hostMatches(host, realmHost) {
			continue
		}
		val, err := decryptCookie(enc, key)
		if err != nil {
			// Skip individually undecryptable cookies; report nothing secret.
			continue
		}
		out = append(out, cookie{Name: name, Value: val})
	}
	return out, rows.Err()
}

func edgeUserDataDir() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", fmt.Errorf("LOCALAPPDATA not set")
	}
	return filepath.Join(local, "Microsoft", "Edge", "User Data"), nil
}

// masterKey reads and DPAPI-decrypts the AES key from Local State.
func masterKey(localStatePath string) ([]byte, error) {
	b, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("read Local State: %w", err)
	}
	var ls struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(b, &ls); err != nil {
		return nil, fmt.Errorf("parse Local State: %w", err)
	}
	if ls.OSCrypt.EncryptedKey == "" {
		return nil, fmt.Errorf("no os_crypt.encrypted_key in Local State")
	}
	raw, err := base64.StdEncoding.DecodeString(ls.OSCrypt.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted_key: %w", err)
	}

	switch {
	case len(raw) >= 5 && string(raw[:5]) == "DPAPI":
		return dpapiDecrypt(raw[5:])
	case len(raw) >= 4 && string(raw[:4]) == "APPB":
		return nil, fmt.Errorf("App-Bound Encryption detected (APPB prefix): this Edge build " +
			"wraps the cookie key beyond user DPAPI, so pure-Go cookie import won't work. " +
			"Fall back to driving the live browser (see ADR option b).")
	default:
		return nil, fmt.Errorf("unknown encrypted_key prefix %q", prefix(raw))
	}
}

func prefix(b []byte) string {
	n := 5
	if len(b) < n {
		n = len(b)
	}
	return string(b[:n])
}

// decryptCookie handles the Chromium v10/v11 AES-256-GCM format:
// "v10"/"v11" (3 bytes) | nonce (12) | ciphertext | tag (16).
func decryptCookie(enc, key []byte) (string, error) {
	if len(enc) > 3 && (string(enc[:3]) == "v10" || string(enc[:3]) == "v11") {
		if len(enc) < 3+12+16 {
			return "", fmt.Errorf("cookie blob too short")
		}
		nonce := enc[3:15]
		ct := enc[15:]
		block, err := aes.NewCipher(key)
		if err != nil {
			return "", err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", err
		}
		pt, err := gcm.Open(nil, nonce, ct, nil)
		if err != nil {
			return "", err
		}
		return string(pt), nil
	}
	// Legacy DPAPI-per-cookie format (older profiles).
	pt, err := dpapiDecrypt(enc)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func hostMatches(hostKey, realmHost string) bool {
	h := strings.TrimPrefix(hostKey, ".")
	realmHost = strings.TrimPrefix(realmHost, ".")
	return h == realmHost || strings.HasSuffix(h, "."+baseDomain(realmHost))
}

// baseDomain returns the last two labels (e.g. ariba.com) so realm subdomain
// cookies scoped to .ariba.com are matched too.
func baseDomain(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// dpapiDecrypt runs CryptUnprotectData on raw bytes (current user scope).
func dpapiDecrypt(raw []byte) ([]byte, error) {
	in := windows.DataBlob{Size: uint32(len(raw))}
	if len(raw) > 0 {
		in.Data = &raw[0]
	}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	if out.Size == 0 {
		return nil, nil
	}
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func copyToTemp(src string) (string, func(), error) {
	b, err := os.ReadFile(src)
	if err != nil {
		return "", nil, fmt.Errorf("copy cookies DB (close Edge if locked): %w", err)
	}
	f, err := os.CreateTemp("", "ariba-cookies-*.db")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
