package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lacymorrow/lash/internal/config"
)

// OauthInfo stores OAuth tokens for a provider.
type OauthInfo struct {
	Type    string `json:"type"`
	Refresh string `json:"refresh"`
	Access  string `json:"access"`
	Expires int64  `json:"expires"`
}

// file path: ~/.local/share/lash/auth.json (XDG data dir)
func authFilepath() string {
	base := config.XDGDataDir()
	dir := filepath.Join(base, config.AppName)
	return filepath.Join(dir, "auth.json")
}

func ensureDir() error {
	path := authFilepath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create auth directory: %w", err)
	}
	return nil
}

func readAll() (map[string]OauthInfo, error) {
	path := authFilepath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]OauthInfo{}, nil
		}
		return nil, err
	}
	var m map[string]OauthInfo
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]OauthInfo{}, nil
	}
	return m, nil
}

func writeAll(m map[string]OauthInfo) error {
	if err := ensureDir(); err != nil {
		return err
	}
	path := authFilepath()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return nil
}

// Get returns stored OAuth info for a provider.
func Get(providerID string) (*OauthInfo, error) {
	all, err := readAll()
	if err != nil {
		return nil, err
	}
	info, ok := all[providerID]
	if !ok {
		return nil, nil
	}
	return &info, nil
}

// Set stores OAuth info for a provider.
func Set(providerID string, info OauthInfo) error {
	all, err := readAll()
	if err != nil {
		return err
	}
	all[providerID] = info
	return writeAll(all)
}

// Remove deletes stored OAuth info for a provider.
func Remove(providerID string) error {
	all, err := readAll()
	if err != nil {
		return err
	}
	delete(all, providerID)
	return writeAll(all)
}
