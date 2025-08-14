package config

import (
	"os"
	"path/filepath"
)

// findConfig searches for the configuration file, prioritizing "lash.json"
// and falling back to "crush.json" for backward compatibility.
func findConfig(appName string) string {
	base := XDGConfigDir()

	// Prioritize the new "lash" directory
	lashDir := filepath.Join(base, "lash")
	lashConfig := filepath.Join(lashDir, "lash.json")

	if _, err := os.Stat(lashConfig); err == nil {
		return lashConfig
	}

	// Fallback to the old "crush" directory
	crushDir := filepath.Join(base, "crush")
	crushConfig := filepath.Join(crushDir, "crush.json")

	if _, err := os.Stat(crushConfig); err == nil {
		return crushConfig
	}

	// If neither exists, return the path to the new config file,
	// which will be created if it doesn't exist.
	return lashConfig
}
