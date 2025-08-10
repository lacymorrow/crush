package shell

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
	pty "github.com/creack/pty"
)

// Minimal Shell page: sends entered commands to the real shell using `sh -lc` as a stub.
// Follow-up: replace with a true PTY for full interactive compatibility.

type Page struct {
	app      *app.App
	width    int
	height   int
	ta       *textarea.Model
	buf      bytes.Buffer
	vp       viewport.Model
	keyRun   key.Binding
	keyCopy  key.Binding
	keyPaste key.Binding
	keyCopy  key.Binding

	ptyFile    *os.File
	shellCmd   *exec.Cmd
	promptMode string
}

func New(app *app.App) *Page {
	ta := textarea.New()
	ta.Placeholder = "Enter shell command..."
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.Focus()
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{}
	return &Page{app: app, ta: ta, vp: vp,
		keyRun:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")),
		keyCopy:  key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy output")),
		keyPaste: key.NewBinding(key.WithKeys("ctrl+v"), key.WithHelp("ctrl+v", "paste")),
	}
}

type shellTickMsg struct{}

func (p *Page) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return shellTickMsg{} })
}

func (p *Page) Init() tea.Cmd { return p.tickCmd() }

func (p *Page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		p.ta.SetWidth(p.width - 4)
		p.vp.SetWidth(p.width - 4)
		if p.height-5 > 0 {
			p.vp.SetHeight(p.height - 5)
		} else {
			p.vp.SetHeight(1)
		}
		// propagate winsize to PTY
		if p.ptyFile != nil {
			_ = pty.Setsize(p.ptyFile, &pty.Winsize{Rows: uint16(max(1, p.height)), Cols: uint16(max(1, p.width))})
		}
		return p, nil
	case shellTickMsg:
		// periodic refresh to display PTY output
		return p, p.tickCmd()
	case tea.KeyPressMsg:
		if key.Matches(msg, p.keyRun) {
			cmdline := strings.TrimSpace(p.ta.Value())
			if cmdline != "" {
				return p, p.writeToShell(cmdline + "\n")
			}
			return p, nil
		}
		if key.Matches(msg, p.keyCopy) {
			_ = clipboard.WriteAll(p.buf.String())
			return p, util.ReportInfo("Output copied")
		}
		if key.Matches(msg, p.keyPaste) {
			if clip, err := clipboard.ReadAll(); err == nil {
				p.ta.InsertString(clip)
			}
			return p, nil
		}
		if key.Matches(msg, p.keyCopy) {
			_ = clipboard.WriteAll(p.buf.String())
			return p, util.ReportInfo("Output copied")
		}
	}
	m, cmd := p.ta.Update(msg)
	p.ta = m
	return p, cmd
}

func (p *Page) View() string {
	t := styles.CurrentTheme()
	p.vp.SetContent(p.buf.String())
	// Match Crush layout: a bordered output area and an input editor with subtle border
	output := t.S().Base.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1).Render(p.vp.View())
	input := t.S().Base.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1).Render(p.ta.View())
	box := lipgloss.JoinVertical(lipgloss.Left, output, input)
	return box
}

func (p *Page) SetSize(w, h int) tea.Cmd {
	p.width, p.height = w, h
	p.ta.SetWidth(w - 4)
	p.vp.SetWidth(w - 4)
	if h-5 > 0 {
		p.vp.SetHeight(h - 5)
	} else {
		p.vp.SetHeight(1)
	}
	return nil
}

func (p *Page) GetSize() (int, int) { return p.width, p.height }

func (p *Page) Bindings() []key.Binding { return []key.Binding{p.keyRun} }

func (p *Page) ensureShell() error {
	if p.ptyFile != nil {
		return nil
	}
	shellPath := config.Get().GetLash().RealShell
	if shellPath == "" {
		shellPath = os.Getenv("SHELL")
	}
	if shellPath == "" {
		shellPath = "/bin/sh"
	}
	cmd := exec.Command(shellPath, "-l")
	// set prompt via env to avoid echoing export
	prompt := "[SHELL]$ "
	switch strings.ToLower(p.promptMode) {
	case "agent":
		prompt = "[AGENT]> "
	case "auto":
		prompt = "[AUTO]$ "
	}
	baseEnv := os.Environ()
	cmd.Env = append(baseEnv, "PS1="+prompt, "PROMPT="+prompt)
	if cwd := p.app.Config().WorkingDir(); cwd != "" {
		cmd.Dir = cwd
	}
	f, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	p.shellCmd = cmd
	p.ptyFile = f
	_ = pty.Setsize(f, &pty.Winsize{Rows: uint16(max(1, p.height)), Cols: uint16(max(1, p.width))})
	// stream output
	go func() {
		buf := make([]byte, 4096)
		for {
			n, rerr := f.Read(buf)
			if n > 0 {
				p.buf.Write(buf[:n])
			}
			if rerr != nil {
				return
			}
		}
	}()
	// handle child exit
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGCHLD)
		<-sigCh
		if p.ptyFile != nil {
			_ = p.ptyFile.Close()
		}
	}()
	if p.promptMode != "" {
		_ = p.setPrompt(p.promptMode)
	}
	return nil
}

func (p *Page) writeToShell(data string) tea.Cmd {
	return func() tea.Msg {
		if err := p.ensureShell(); err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("pty error: %v", err)}
		}
		if _, err := p.ptyFile.WriteString(data); err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("write error: %v", err)}
		}
		p.ta.SetValue("")
		return nil
	}
}

func (p *Page) setPrompt(mode string) error {
	p.promptMode = mode
	if p.ptyFile == nil {
		return nil
	}
	ps1 := "[" + strings.ToUpper(mode) + "]$ "
	_, err := p.ptyFile.WriteString("export PS1='" + ps1 + "'\n")
	return err
}

// SetPrompt is exported for the app to change prompt on mode switch
func (p *Page) SetPrompt(mode string) {
	_ = p.setPrompt(mode)
}
