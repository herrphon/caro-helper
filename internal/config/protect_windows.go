//go:build windows

package config

import (
	"encoding/base64"
	"unsafe"

	"golang.org/x/sys/windows"
)

// protect encrypts with DPAPI bound to the current Windows user.
func protect(plain string) (string, error) {
	in := bytesBlob([]byte(plain))
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return base64.StdEncoding.EncodeToString(blobBytes(&out)), nil
}

func unprotect(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	in := bytesBlob(raw)
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return string(blobBytes(&out)), nil
}

func bytesBlob(b []byte) windows.DataBlob {
	if len(b) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

func blobBytes(b *windows.DataBlob) []byte {
	if b.Size == 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice(b.Data, b.Size)...)
}
