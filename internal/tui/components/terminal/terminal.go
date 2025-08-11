package terminal

import (
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/shell"
)

// TerminalOutputMsg carries raw bytes from the PTY to the UI loop.
type TerminalOutputMsg struct{ Data []byte }

// TerminalClosedMsg indicates the PTY process exited.
type TerminalClosedMsg struct{ Err error }

// Terminal is a minimal Bubble Tea component that displays a PTY-backed user shell
// and forwards keystrokes to it.
type Terminal struct {
	app    *app.App
	width  int
	height int

	mu     sync.Mutex
	pty    *shell.PTYShell
	closed bool

	// naive text buffer – we accumulate and trim to a reasonable size
	// Rendering assumes ANSI bytes can be passed through as-is.
	buffer strings.Builder
}

func New(a *app.App) *Terminal { return &Terminal{app: a} }

func (t *Terminal) Init() tea.Cmd {
	if t.pty != nil {
		return t.listen()
	}
	// Start user shell in working dir
	s, err := shell.NewPTYShell("", nil, nil, t.app.Config().WorkingDir())
	if err != nil {
		// Defer reporting via returned command
		return func() tea.Msg { return TerminalClosedMsg{Err: err} }
	}
	t.pty = s
	return t.listen()
}

func (t *Terminal) listen() tea.Cmd {
	if t.pty == nil {
		return nil
	}
	output := t.pty.Output()
	return func() tea.Msg {
		if output == nil {
			return TerminalClosedMsg{Err: nil}
		}
		b, ok := <-output
		if !ok {
			return TerminalClosedMsg{Err: nil}
		}
		return TerminalOutputMsg{Data: b}
	}
}

func (t *Terminal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		_ = t.Resize(m.Width, m.Height)
		return t, nil
	case tea.KeyPressMsg:
		if t.closed || t.pty == nil {
			return t, nil
		}
		seq := keyToBytes(m)
		if len(seq) > 0 {
			_, _ = t.pty.Write(seq)
		}
		return t, nil
	case tea.PasteMsg:
		if t.closed || t.pty == nil {
			return t, nil
		}
		if len(m) > 0 {
			_, _ = t.pty.Write([]byte(m))
		}
		return t, nil
	case TerminalOutputMsg:
		t.mu.Lock()
		// Bound the buffer to avoid unbounded growth
		if t.buffer.Len() > 1_000_000 { // ~1MB
			// drop first half by recreating builder with suffix
			current := t.buffer.String()
			half := len(current) / 2
			var b strings.Builder
			b.Grow(len(current) - half + len(m.Data))
			b.WriteString(current[half:])
			t.buffer = b
		}
		t.buffer.Write(m.Data)
		t.mu.Unlock()
		// Continue listening
		return t, t.listen()
	case TerminalClosedMsg:
		t.closed = true
		return t, nil
	}
	return t, nil
}

func (t *Terminal) View() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	// For now, return the raw buffer; lipgloss canvas above will compose it.
	return t.buffer.String()
}

// SetSize allows the container to control width/height.
func (t *Terminal) SetSize(width, height int) tea.Cmd {
	t.width, t.height = width, height
	_ = t.Resize(width, height)
	return nil
}

// Resize notifies the PTY of a window size change.
func (t *Terminal) Resize(width, height int) error {
	if t.pty == nil {
		return nil
	}
	return t.pty.Resize(width, height)
}

// keyToBytes maps Bubble Tea key strings to terminal byte sequences.
func keyToBytes(k tea.KeyPressMsg) []byte {
	s := k.String()
	switch s {
	case "enter":
		return []byte{'\r'}
	case "tab":
		return []byte{'\t'}
	case "backspace":
		return []byte{0x7f}
	case "ctrl+c":
		return []byte{0x03}
	case "ctrl+d":
		return []byte{0x04}
	case "ctrl+z":
		return []byte{0x1a}
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	default:
		// If it's a single printable character, forward directly
		if len(s) == 1 {
			return []byte(s)
		}
		return nil
	}
}
