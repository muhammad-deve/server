package config

import (
	"os"
	"path/filepath"
	"strings"
)

var (
	ServerAddr string
	Domain     string
	Token      string
)

func Load() {
	ServerAddr = os.Getenv("GOPORT_SERVER_ADDR")
	if ServerAddr == "" {
		ServerAddr = "goport.uz:7000"
	}

	Domain = os.Getenv("GOPORT_DOMAIN")
	if Domain == "" {
		Domain = "goport.uz"
	}

	// A token from the environment wins; otherwise fall back to the saved one.
	Token = strings.TrimSpace(os.Getenv("GOPORT_TOKEN"))
	if Token == "" {
		if b, err := os.ReadFile(TokenPath()); err == nil {
			Token = strings.TrimSpace(string(b))
		}
	}
}

// TokenPath returns the on-disk location of the saved account token.
func TokenPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "goport", "token")
}

// SaveToken persists the account token to disk and updates the in-memory value.
func SaveToken(token string) error {
	token = strings.TrimSpace(token)
	path := TokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return err
	}
	Token = token
	return nil
}

// ClearToken removes the saved account token.
func ClearToken() error {
	Token = ""
	err := os.Remove(TokenPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
