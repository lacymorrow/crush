package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Centralized environment variable keys
const (
	EnvXDGConfigHome = "XDG_CONFIG_HOME"
	EnvXDGDataHome   = "XDG_DATA_HOME"
	EnvLocalAppData  = "LOCALAPPDATA"
	EnvUserProfile   = "USERPROFILE"
	EnvHome          = "HOME"
)

// XDGConfigDir returns the base XDG config directory path.
// Falls back to $HOME/.config on non-Windows systems when XDG is not set.
func XDGConfigDir() string {
	if dir := os.Getenv(EnvXDGConfigHome); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		// On Windows, prefer LOCALAPPDATA for config/data style storage
		localAppData := os.Getenv(EnvLocalAppData)
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv(EnvUserProfile), "AppData", "Local")
		}
		return localAppData
	}
	return filepath.Join(HomeDir(), ".config")
}

// XDGDataDir returns the base XDG data directory path.
// Falls back to platform defaults when XDG is not set.
func XDGDataDir() string {
	if dir := os.Getenv(EnvXDGDataHome); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv(EnvLocalAppData)
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv(EnvUserProfile), "AppData", "Local")
		}
		return localAppData
	}
	return filepath.Join(HomeDir(), ".local", "share")
}
