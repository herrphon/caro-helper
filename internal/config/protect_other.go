//go:build !windows

package config

import "encoding/base64"

// Non-Windows builds exist only for development on the Mac; there is no
// per-user secret store wired up, so the token is merely base64-obfuscated.
// The file is 0600 and lives in the user's config dir.

func protect(plain string) (string, error) {
	return "plain:" + base64.StdEncoding.EncodeToString([]byte(plain)), nil
}

func unprotect(enc string) (string, error) {
	const prefix = "plain:"
	if len(enc) < len(prefix) || enc[:len(prefix)] != prefix {
		return "", errNotDecryptable
	}
	b, err := base64.StdEncoding.DecodeString(enc[len(prefix):])
	return string(b), err
}
