package tui

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

type KeyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Commands key.Binding
	Suspend  key.Binding
	Sessions key.Binding

	// Modes
	ToggleMode key.Binding

	// Safety
	ToggleAutoConfirm key.Binding

	pageBindings []key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "more"),
		),
		Commands: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		),
		Suspend: key.NewBinding(
			key.WithKeys("ctrl+z"),
			key.WithHelp("ctrl+z", "suspend"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "sessions"),
		),

		// Single toggle for modes
		ToggleMode: key.NewBinding(
			key.WithKeys("ctrl+space"),
			key.WithHelp("ctrl+space", "mode"),
		),

		// Toggle auto-confirm (Shift-Tab)
		ToggleAutoConfirm: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "auto-confirm"),
		),
	}
}
