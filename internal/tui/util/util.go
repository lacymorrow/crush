package util

import (
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
)

type Cursor interface {
	Cursor() *tea.Cursor
}

type Model interface {
	tea.Model
	tea.ViewModel
}

func CmdHandler(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

func ReportError(err error) tea.Cmd {
	slog.Error("Error reported", "error", err)
	return CmdHandler(InfoMsg{
		Type: InfoTypeError,
		Msg:  err.Error(),
	})
}

type InfoType int

const (
	InfoTypeInfo InfoType = iota
	InfoTypeWarn
	InfoTypeError
)

func ReportInfo(info string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeInfo,
		Msg:  info,
	})
}

func ReportWarn(warn string) tea.Cmd {
	return CmdHandler(InfoMsg{
		Type: InfoTypeWarn,
		Msg:  warn,
	})
}

type (
	InfoMsg struct {
		Type InfoType
		Msg  string
		TTL  time.Duration
	}
	ClearStatusMsg struct{}
)

func Clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}

// AppModelAccessor exposes the active mode without importing UI internals
type AppModelAccessor interface {
	ActiveMode() string
}

var appModel AppModelAccessor

// RegisterAppModel allows the TUI root to register a minimal accessor
func RegisterAppModel(m AppModelAccessor) {
	appModel = m
}

// TryGetAppModel returns the accessor if present
func TryGetAppModel() (AppModelAccessor, bool) {
	if appModel == nil {
		return nil, false
	}
	return appModel, true
}

// ActiveModeFromApp returns current mode with fallback to app.App.Mode
func ActiveModeFromApp(a *app.App) string {
	if m, ok := TryGetAppModel(); ok {
		return m.ActiveMode()
	}
	return a.Mode
}
