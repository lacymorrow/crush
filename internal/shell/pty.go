package shell

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	ptypkg "github.com/creack/pty"
)

// PTYShell manages a long-lived interactive shell attached to a PTY.
// It is intended for real user shells to preserve history, completions,
// interactive full-screen apps (vim, less, fzf), and custom shell plugins.
//
// Start the shell with NewPTYShell, consume output via Output(), forward
// keystrokes with Write(), and call Resize() on window changes.
// Close() terminates the shell process gracefully.
type PTYShell struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	ptmx     *os.File
	outputCh chan []byte
	doneCh   chan error
	started  bool
}

// DetectUserShell returns the user shell binary and default args.
// It checks LASH_SHELL, then SHELL, and falls back to a sensible default.
func DetectUserShell() (bin string, args []string) {
	// Allow explicit override
	if v := strings.TrimSpace(os.Getenv("LASH_SHELL")); v != "" {
		parts := strings.Fields(v)
		if len(parts) > 0 {
			return parts[0], parts[1:]
		}
	}

	// Use the user's login shell
	if sh := strings.TrimSpace(os.Getenv("SHELL")); sh != "" {
		return sh, []string{"-i", "-l"}
	}

	// Common fallbacks
	candidates := []string{"/bin/zsh", "/bin/bash", "/bin/sh"}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, []string{"-i", "-l"}
		}
	}
	// Last resort
	return "/bin/sh", []string{"-i"}
}

// NewPTYShell spawns a user shell attached to a new PTY. The environment and
// working directory can be customized; if env is nil, os.Environ() is used.
func NewPTYShell(shellBin string, shellArgs []string, env []string, cwd string) (*PTYShell, error) {
	if shellBin == "" {
		bin, args := DetectUserShell()
		shellBin, shellArgs = bin, args
	}
	if env == nil {
		env = os.Environ()
	}

	cmd := exec.Command(shellBin, shellArgs...)
	cmd.Env = env
	if cwd != "" {
		cmd.Dir = cwd
	}

	ptmx, err := ptypkg.Start(cmd)
	if err != nil {
		return nil, err
	}

	s := &PTYShell{
		cmd:      cmd,
		ptmx:     ptmx,
		outputCh: make(chan []byte, 64),
		doneCh:   make(chan error, 1),
		started:  true,
	}

	go s.pumpOutput()
	go s.wait()

	// Forward child SIGHUP on parent SIGHUP/TERM to encourage clean shutdowns
	go s.forwardSignals()
	return s, nil
}

func (s *PTYShell) pumpOutput() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			s.outputCh <- chunk
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				close(s.outputCh)
			}
			return
		}
	}
}

func (s *PTYShell) wait() {
	err := s.cmd.Wait()
	s.doneCh <- err
	close(s.doneCh)
}

func (s *PTYShell) forwardSignals() {
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	for sig := range sigCh {
		_ = s.Signal(sig)
	}
}

// Output returns a channel of raw PTY bytes (ANSI included). Consumers should
// render these bytes appropriately (e.g., in a terminal widget).
func (s *PTYShell) Output() <-chan []byte {
	return s.outputCh
}

// Done returns a channel that is closed with the process exit error once the
// shell terminates.
func (s *PTYShell) Done() <-chan error {
	return s.doneCh
}

// Write sends bytes to the shell's stdin (through the PTY).
func (s *PTYShell) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started || s.ptmx == nil {
		return 0, io.ErrClosedPipe
	}
	return s.ptmx.Write(p)
}

// WriteString is a convenience wrapper over Write.
func (s *PTYShell) WriteString(str string) (int, error) {
	return s.Write([]byte(str))
}

// Resize sets the PTY window size.
func (s *PTYShell) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started || s.ptmx == nil {
		return io.ErrClosedPipe
	}
	ws := &ptypkg.Winsize{Cols: uint16(cols), Rows: uint16(rows)}
	return ptypkg.Setsize(s.ptmx, ws)
}

// Signal forwards a signal to the shell process.
func (s *PTYShell) Signal(sig os.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Signal(sig)
}

// Close attempts a graceful shutdown and then force-kills if needed.
func (s *PTYShell) Close() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	ptmx := s.ptmx
	cmd := s.cmd
	s.ptmx = nil
	s.mu.Unlock()

	// Try to signal a hangup (like terminal closure)
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(syscall.SIGHUP)
	}

	// Give it a moment to exit gracefully
	select {
	case <-time.After(300 * time.Millisecond):
	case <-s.doneCh:
	}

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if ptmx != nil {
		_ = ptmx.Close()
	}
	return nil
}

// NewPTYShellContext allows canceling startup if context is done before the shell
// is fully initialized.
func NewPTYShellContext(ctx context.Context, shellBin string, shellArgs []string, env []string, cwd string) (*PTYShell, error) {
	type result struct {
		s   *PTYShell
		err error
	}
	ch := make(chan result, 1)
	go func() {
		s, err := NewPTYShell(shellBin, shellArgs, env, cwd)
		ch <- result{s: s, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.s, r.err
	}
}
