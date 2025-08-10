package shell

import (
	"log/slog"
	"sync"
)

// PersistentShell is a singleton shell instance that maintains state across the application
type PersistentShell struct {
	*Shell
}

var (
	once              sync.Once
	shellInstance     *PersistentShell
	userOnce          sync.Once
	userShellInstance *PersistentShell
)

// GetPersistentShell returns the singleton persistent shell instance
// This maintains backward compatibility with the existing API
func GetPersistentShell(cwd string) *PersistentShell {
	once.Do(func() {
		shellInstance = &PersistentShell{
			Shell: NewShell(&Options{
				WorkingDir: cwd,
				Logger:     &loggingAdapter{},
			}),
		}
	})
	return shellInstance
}

// GetUserPersistentShell returns a persistent shell intended for user commands.
// It does not apply tool-level block functions.
func GetUserPersistentShell(cwd string) *PersistentShell {
	userOnce.Do(func() {
		userShellInstance = &PersistentShell{
			Shell: NewShell(&Options{
				WorkingDir: cwd,
				Logger:     &loggingAdapter{},
			}),
		}
	})
	return userShellInstance
}

// slog.dapter adapts the internal slog.package to the Logger interface
type loggingAdapter struct{}

func (l *loggingAdapter) InfoPersist(msg string, keysAndValues ...any) {
	slog.Info(msg, keysAndValues...)
}
