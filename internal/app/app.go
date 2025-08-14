package app

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/lacymorrow/lash/internal/config"
	"github.com/lacymorrow/lash/internal/csync"
	"github.com/lacymorrow/lash/internal/db"
	"github.com/lacymorrow/lash/internal/format"
	"github.com/lacymorrow/lash/internal/history"
	"github.com/lacymorrow/lash/internal/llm/agent"
	"github.com/lacymorrow/lash/internal/log"
	"github.com/lacymorrow/lash/internal/pubsub"

	"github.com/lacymorrow/lash/internal/lsp"
	"github.com/lacymorrow/lash/internal/message"
	"github.com/lacymorrow/lash/internal/permission"
	"github.com/lacymorrow/lash/internal/session"
)

const subscriberSendTimeout = 2 * time.Second

type App struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service

	CoderAgent agent.Service

	LSPClients map[string]*lsp.Client

	clientsMutex sync.RWMutex

	watcherCancelFuncs *csync.Slice[context.CancelFunc]
	lspWatcherWG       sync.WaitGroup

	config *config.Config

	serviceEventsWG *sync.WaitGroup
	eventsCtx       context.Context
	events          chan tea.Msg
	tuiWG           *sync.WaitGroup

	// global context and cleanup functions
	globalCtx    context.Context
	cleanupFuncs []func()

	// UI Mode: "Shell", "Agent", or "Auto"
	Mode string

	// InputHistory stores a global list of user-entered prompts across all
	// sessions for convenient navigation with Up/Down arrows, similar to a shell.
	// Most recent entries are appended at the end.
	InputHistory []string
}

// New initializes a new applcation instance.
func New(ctx context.Context, conn *sql.DB, cfg *config.Config) (*App, error) {
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
	files := history.NewService(q, conn)
	skipPermissionsRequests := cfg.Permissions != nil && cfg.Permissions.SkipRequests
	// If YOLO is enabled in config, skip all permission requests
	if cfg.Lash != nil && cfg.Lash.Yolo {
		skipPermissionsRequests = true
	}
	allowedTools := []string{}
	if cfg.Permissions != nil && cfg.Permissions.AllowedTools != nil {
		allowedTools = cfg.Permissions.AllowedTools
	}
	// If Lash safety confirm is explicitly disabled, allow bash:execute without prompts.
	if cfg.Lash != nil && cfg.Lash.Safety.ConfirmAgentExec != nil && !*cfg.Lash.Safety.ConfirmAgentExec {
		// Allow either tool-wide or specific action key depending on how permissions are checked
		// Permission service treats both "bash" and "bash:execute" as allow entries
		allowedTools = append(allowedTools, "bash", "bash:execute")
	}

	app := &App{
		Sessions:    sessions,
		Messages:    messages,
		History:     files,
		Permissions: permission.NewPermissionService(cfg.WorkingDir(), skipPermissionsRequests, allowedTools),
		LSPClients:  make(map[string]*lsp.Client),

		globalCtx: ctx,

		config: cfg,

		watcherCancelFuncs: csync.NewSlice[context.CancelFunc](),

		events:          make(chan tea.Msg, 100),
		serviceEventsWG: &sync.WaitGroup{},
		tuiWG:           &sync.WaitGroup{},

		Mode: cfg.ActiveMode(),

		InputHistory: make([]string, 0, 128),
	}

	app.setupEvents()

	// Load global input history from disk (best-effort; non-fatal on error)
	if err := app.LoadInputHistory(); err != nil {
		slog.Warn("Failed to load input history", "error", err)
	}

	// Initialize LSP clients in the background.
	app.initLSPClients(ctx)

	// TODO: remove the concept of agent config, most likely.
	if cfg.IsConfigured() {
		if err := app.InitCoderAgent(); err != nil {
			return nil, fmt.Errorf("failed to initialize coder agent: %w", err)
		}
	} else {
		slog.Warn("No agent configuration found")
	}
	return app, nil
}

// historyFilePath returns the path to the persisted input history file.
func (app *App) historyFilePath() string {
	// Store alongside the main database in the data directory
	return filepath.Join(app.config.Options.DataDirectory, "input_history.jsonl")
}

// LoadInputHistory loads the global input history from disk, if present.
// Entries are stored as JSON strings, one per line.
func (app *App) LoadInputHistory() error {
	path := app.historyFilePath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var loaded []string
	scanner := bufio.NewScanner(f)
	// Increase the buffer in case of long multi-line prompts
	buf := make([]byte, 0, 1024*64)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var s string
		if err := json.Unmarshal(line, &s); err != nil {
			// Skip malformed lines
			continue
		}
		if s == "" {
			continue
		}
		loaded = append(loaded, s)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(loaded) > 0 {
		app.InputHistory = append(app.InputHistory, loaded...)
	}
	return nil
}

// AppendInputHistory appends an entry to the in-memory and on-disk history.
// It skips consecutive duplicates. Entries are stored as JSONL for safe multiline support.
func (app *App) AppendInputHistory(entry string) error {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return nil
	}
	if n := len(app.InputHistory); n > 0 && app.InputHistory[n-1] == entry {
		return nil
	}
	app.InputHistory = append(app.InputHistory, entry)

	// Ensure data directory exists
	if err := os.MkdirAll(app.config.Options.DataDirectory, 0o700); err != nil {
		return err
	}
	// Append as JSON string + newline
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(app.historyFilePath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

// Config returns the application configuration.
func (app *App) Config() *config.Config {
	return app.config
}

// RunNonInteractive handles the execution flow when a prompt is provided via
// CLI flag.
func (app *App) RunNonInteractive(ctx context.Context, prompt string, quiet bool) error {
	slog.Info("Running in non-interactive mode")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start spinner if not in quiet mode.
	var spinner *format.Spinner
	if !quiet {
		spinner = format.NewSpinner(ctx, cancel, "Generating")
		spinner.Start()
	}

	// Helper function to stop spinner once.
	stopSpinner := func() {
		if !quiet && spinner != nil {
			spinner.Stop()
			spinner = nil
		}
	}
	defer stopSpinner()

	const maxPromptLengthForTitle = 100
	titlePrefix := "Non-interactive: "
	var titleSuffix string

	if len(prompt) > maxPromptLengthForTitle {
		titleSuffix = prompt[:maxPromptLengthForTitle] + "..."
	} else {
		titleSuffix = prompt
	}
	title := titlePrefix + titleSuffix

	sess, err := app.Sessions.Create(ctx, title)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}
	slog.Info("Created session for non-interactive run", "session_id", sess.ID)

	// Automatically approve all permission requests for this non-interactive session
	app.Permissions.AutoApproveSession(sess.ID)

	if app.CoderAgent == nil {
		return fmt.Errorf("coder agent is not initialized")
	}
	done, err := app.CoderAgent.Run(ctx, sess.ID, prompt)
	if err != nil {
		return fmt.Errorf("failed to start agent processing stream: %w", err)
	}

	messageEvents := app.Messages.Subscribe(ctx)
	readBts := 0

	for {
		select {
		case result := <-done:
			stopSpinner()

			if result.Error != nil {
				if errors.Is(result.Error, context.Canceled) || errors.Is(result.Error, agent.ErrRequestCancelled) {
					slog.Info("Non-interactive: agent processing cancelled", "session_id", sess.ID)
					return nil
				}
				return fmt.Errorf("agent processing failed: %w", result.Error)
			}

			msgContent := result.Message.Content().String()
			if len(msgContent) < readBts {
				slog.Error("Non-interactive: message content is shorter than read bytes", "message_length", len(msgContent), "read_bytes", readBts)
				return fmt.Errorf("message content is shorter than read bytes: %d < %d", len(msgContent), readBts)
			}
			fmt.Println(msgContent[readBts:])

			slog.Info("Non-interactive: run completed", "session_id", sess.ID)
			return nil

		case event := <-messageEvents:
			msg := event.Payload
			if msg.SessionID == sess.ID && msg.Role == message.Assistant && len(msg.Parts) > 0 {
				stopSpinner()
				part := msg.Content().String()[readBts:]
				fmt.Print(part)
				readBts += len(part)
			}

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}

func (app *App) UpdateAgentModel() error {
	if app.CoderAgent == nil {
		return fmt.Errorf("coder agent is not initialized")
	}
	return app.CoderAgent.UpdateModel()
}

func (app *App) setupEvents() {
	ctx, cancel := context.WithCancel(app.globalCtx)
	app.eventsCtx = ctx
	setupSubscriber(ctx, app.serviceEventsWG, "sessions", app.Sessions.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "messages", app.Messages.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions", app.Permissions.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions-notifications", app.Permissions.SubscribeNotifications, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "history", app.History.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "mcp", agent.SubscribeMCPEvents, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "lsp", SubscribeLSPEvents, app.events)
	cleanupFunc := func() {
		cancel()
		app.serviceEventsWG.Wait()
	}
	app.cleanupFuncs = append(app.cleanupFuncs, cleanupFunc)
}

func setupSubscriber[T any](
	ctx context.Context,
	wg *sync.WaitGroup,
	name string,
	subscriber func(context.Context) <-chan pubsub.Event[T],
	outputCh chan<- tea.Msg,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		subCh := subscriber(ctx)
		for {
			select {
			case event, ok := <-subCh:
				if !ok {
					slog.Debug("subscription channel closed", "name", name)
					return
				}
				var msg tea.Msg = event
				select {
				case outputCh <- msg:
				case <-time.After(subscriberSendTimeout):
					slog.Warn("message dropped due to slow consumer", "name", name)
				case <-ctx.Done():
					slog.Debug("subscription cancelled", "name", name)
					return
				}
			case <-ctx.Done():
				slog.Debug("subscription cancelled", "name", name)
				return
			}
		}
	}()
}

func (app *App) InitCoderAgent() error {
	coderAgentCfg := app.config.Agents["coder"]
	if coderAgentCfg.ID == "" {
		return fmt.Errorf("coder agent configuration is missing")
	}
	var err error
	app.CoderAgent, err = agent.NewAgent(
		app.globalCtx,
		coderAgentCfg,
		app.Permissions,
		app.Sessions,
		app.Messages,
		app.History,
		app.LSPClients,
	)
	if err != nil {
		slog.Error("Failed to create coder agent", "err", err)
		return err
	}

	// Add MCP client cleanup to shutdown process
	app.cleanupFuncs = append(app.cleanupFuncs, agent.CloseMCPClients)

	setupSubscriber(app.eventsCtx, app.serviceEventsWG, "coderAgent", app.CoderAgent.Subscribe, app.events)
	return nil
}

// Subscribe sends events to the TUI as tea.Msgs.
func (app *App) Subscribe(program *tea.Program) {
	defer log.RecoverPanic("app.Subscribe", func() {
		slog.Info("TUI subscription panic: attempting graceful shutdown")
		program.Quit()
	})

	app.tuiWG.Add(1)
	tuiCtx, tuiCancel := context.WithCancel(app.globalCtx)
	app.cleanupFuncs = append(app.cleanupFuncs, func() {
		slog.Debug("Cancelling TUI message handler")
		tuiCancel()
		app.tuiWG.Wait()
	})
	defer app.tuiWG.Done()

	for {
		select {
		case <-tuiCtx.Done():
			slog.Debug("TUI message handler shutting down")
			return
		case msg, ok := <-app.events:
			if !ok {
				slog.Debug("TUI message channel closed")
				return
			}
			program.Send(msg)
		}
	}
}

// Shutdown performs a graceful shutdown of the application.
func (app *App) Shutdown() {
	if app.CoderAgent != nil {
		app.CoderAgent.CancelAll()
	}

	for cancel := range app.watcherCancelFuncs.Seq() {
		cancel()
	}

	// Wait for all LSP watchers to finish.
	app.lspWatcherWG.Wait()

	// Get all LSP clients.
	app.clientsMutex.RLock()
	clients := make(map[string]*lsp.Client, len(app.LSPClients))
	maps.Copy(clients, app.LSPClients)
	app.clientsMutex.RUnlock()

	// Shutdown all LSP clients.
	for name, client := range clients {
		shutdownCtx, cancel := context.WithTimeout(app.globalCtx, 5*time.Second)
		if err := client.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown LSP client", "name", name, "error", err)
		}
		cancel()
	}

	// Call call cleanup functions.
	for _, cleanup := range app.cleanupFuncs {
		if cleanup != nil {
			cleanup()
		}
	}
}
