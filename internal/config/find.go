package config

import (
	"os"
	"path/filepath"
)

// findConfig searches for the configuration file, prioritizing the current
// application's config and falling back to the legacy config for compatibility.
func findConfig(appName string) string {
	base := XDGConfigDir()

	// Prioritize the current application directory
	lashDir := filepath.Join(base, appName)
	lashConfig := filepath.Join(lashDir, CurrentConfigFilename)

	if _, err := os.Stat(lashConfig); err == nil {
		return lashConfig
	}

	// Fallback to the legacy application directory
	crushDir := filepath.Join(base, LegacyAppName)
	crushConfig := filepath.Join(crushDir, LegacyConfigFilename)

	if _, err := os.Stat(crushConfig); err == nil {
		return crushConfig
	}

	// If neither exists, return the path to the new config file,
	// which will be created if it doesn't exist.
	return lashConfig
}
