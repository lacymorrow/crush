package config

// LashOptions defines extensions specific to the Lash fork
// and is embedded into Crush's existing configuration as the
// "lash" top-level JSON key to preserve backward compatibility.
type LashOptions struct {
    // DefaultMode selects the initial mode at startup.
    // Valid values: "shell", "agent", "auto". Defaults to "shell".
    DefaultMode string `json:"default_mode,omitempty"`
    // RealShell is the path to the real shell to spawn for Shell mode.
    // Defaults to the user's $SHELL or "/bin/sh".
    RealShell string `json:"real_shell,omitempty"`
    // AutoModeEnabled toggles simple heuristics for Auto mode.
    AutoModeEnabled bool `json:"auto_mode_enabled,omitempty"`
}

// GetLash returns a non-nil LashOptions pointer with defaults applied.
func (c *Config) GetLash() *LashOptions {
    if c == nil {
        return &LashOptions{DefaultMode: "shell"}
    }
    if c.Lash == nil {
        c.Lash = &LashOptions{DefaultMode: "shell"}
    }
    if c.Lash.DefaultMode == "" {
        c.Lash.DefaultMode = "shell"
    }
    return c.Lash
}


