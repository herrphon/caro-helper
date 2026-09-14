// Package config persists carohelper's settings in %APPDATA%\CaroHelper\config.json
// (or the OS equivalent). The Smartsheet token is stored encrypted with a
// platform-specific scheme (DPAPI on Windows) so the file is useless on
// another machine or user account.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const (
	appDirName = "CaroHelper"
	fileName   = "config.json"
)

// errNotDecryptable is returned when the stored token was written by a
// different platform/user and cannot be recovered; the user must re-enter it.
var errNotDecryptable = errors.New("stored token cannot be decrypted on this machine; please enter it again")

// SheetAlias maps a human name to a Smartsheet sheet or report id.
type SheetAlias struct {
	Alias   string `json:"alias"`
	Kind    string `json:"kind"` // "sheet" (default) or "report"
	SheetID int64  `json:"sheetId"`
}

// Config is the on-disk shape. TokenEnc is the protected token, base64.
type Config struct {
	TokenEnc string       `json:"tokenEnc,omitempty"`
	Sheets   []SheetAlias `json:"sheets"`
	Port     int          `json:"port,omitempty"`
}

type Store struct {
	path string
	mu   sync.Mutex
	cfg  Config
}

// Path returns the config file location without touching the disk.
func Path() (string, error) {
	base, err := os.UserConfigDir() // %APPDATA% on Windows
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName, fileName), nil
}

// Load reads the config, creating an empty one if it does not exist.
// An empty path means the default location (see Path).
func Load(path string) (*Store, error) {
	p := path
	if p == "" {
		var err error
		if p, err = Path(); err != nil {
			return nil, err
		}
	}
	s := &Store{path: p, cfg: Config{Sheets: []SheetAlias{}, Port: 8765}}
	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return s, s.save()
	case err != nil:
		return nil, err
	}
	if err := json.Unmarshal(data, &s.cfg); err != nil {
		return nil, err
	}
	if s.cfg.Sheets == nil {
		s.cfg.Sheets = []SheetAlias{}
	}
	for i := range s.cfg.Sheets {
		if s.cfg.Sheets[i].Kind == "" {
			s.cfg.Sheets[i].Kind = "sheet"
		}
	}
	if s.cfg.Port == 0 {
		s.cfg.Port = 8765
	}
	return s, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *Store) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Port
}

func (s *Store) Sheets() []SheetAlias {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SheetAlias, len(s.cfg.Sheets))
	copy(out, s.cfg.Sheets)
	return out
}

func (s *Store) SetSheets(sheets []SheetAlias) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Sheets = sheets
	return s.save()
}

// HasToken reports whether a token is stored, without decrypting it.
func (s *Store) HasToken() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.TokenEnc != ""
}

func (s *Store) Token() (string, error) {
	s.mu.Lock()
	enc := s.cfg.TokenEnc
	s.mu.Unlock()
	if enc == "" {
		return "", nil
	}
	return unprotect(enc)
}

func (s *Store) SetToken(token string) error {
	enc, err := protect(token)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.TokenEnc = enc
	return s.save()
}
