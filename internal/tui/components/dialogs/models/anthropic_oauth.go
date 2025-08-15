package models

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacymorrow/lash/internal/auth"
	"github.com/lacymorrow/lash/internal/config"
	"github.com/lacymorrow/lash/internal/tui/components/dialogs"
	"github.com/lacymorrow/lash/internal/tui/styles"
	"github.com/lacymorrow/lash/internal/tui/util"
)

// AnthropicOAuthDialogID identifies the OAuth dialog
const AnthropicOAuthDialogID dialogs.DialogID = "anthropic_oauth"

type anthropicOAuthDialog struct {
	width   int
	wWidth  int
	wHeight int

	provider  catwalk.Provider
	model     catwalk.Model
	modelType config.SelectedModelType

	url      string
	verifier string

	codeInput textinput.Model
	status    string

	keyMap keyMapOAuth
}

type keyMapOAuth struct {
	Submit key.Binding
	Open   key.Binding
	Copy   key.Binding
	Cancel key.Binding
}

func newKeyMapOAuth() keyMapOAuth {
	return keyMapOAuth{
		Submit: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit code")),
		Open:   key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open link")),
		Copy:   key.NewBinding(key.WithKeys("c", "y"), key.WithHelp("c/y", "copy link")),
		Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

func NewAnthropicOAuthDialogCmp(option *ModelOption, modelType config.SelectedModelType) dialogs.DialogModel {
	t := styles.CurrentTheme()
	ti := textinput.New()
	ti.Placeholder = "Paste code#state here"
	ti.SetVirtualCursor(false)
	ti.Prompt = "> "
	ti.SetStyles(t.S().TextInput)
	ti.Focus()

	return &anthropicOAuthDialog{
		width:     70,
		provider:  option.Provider,
		model:     option.Model,
		modelType: modelType,
		codeInput: ti,
		keyMap:    newKeyMapOAuth(),
	}
}

func (d *anthropicOAuthDialog) ID() dialogs.DialogID { return AnthropicOAuthDialogID }

func (d *anthropicOAuthDialog) Init() tea.Cmd {
	// Build URL and open browser
	return tea.Sequence(
		func() tea.Msg {
			url, verifier, err := auth.AuthorizeURL("max")
			if err != nil {
				return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("failed to start OAuth: %v", err)}
			}
			d.url = url
			d.verifier = verifier
			return nil
		},
		d.openBrowserCmd(),
	)
}

func (d *anthropicOAuthDialog) openBrowserCmd() tea.Cmd {
	if d.url == "" {
		return nil
	}
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", d.url)
		case "linux":
			cmd = exec.Command("xdg-open", d.url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", d.url)
		default:
			cmd = exec.Command("open", d.url) // best-effort
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: "Opened browser for Claude Max sign-in"}
	}
}

// oauthSuccessMsg is emitted when token exchange succeeded
type oauthSuccessMsg struct {
	providerID string
	modelID    string
	modelType  config.SelectedModelType
}

func (d *anthropicOAuthDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth = m.Width
		d.wHeight = m.Height
		return d, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(m, d.keyMap.Open):
			return d, d.openBrowserCmd()
		case key.Matches(m, d.keyMap.Copy):
			if d.url != "" {
				_ = clipboard.WriteAll(d.url)
				return d, util.ReportInfo("Auth link copied to clipboard")
			}
			return d, nil
		case key.Matches(m, d.keyMap.Cancel):
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(m, d.keyMap.Submit):
			code := strings.TrimSpace(d.codeInput.Value())
			if code == "" {
				return d, nil
			}
			d.status = "Exchanging code..."
			return d, d.exchangeCmd(code)
		}
	case oauthSuccessMsg:
		// Close and propagate selection
		return d, tea.Sequence(
			util.CmdHandler(dialogs.CloseDialogMsg{}),
			util.CmdHandler(ModelSelectedMsg{
				Model:     config.SelectedModel{Model: m.modelID, Provider: m.providerID},
				ModelType: m.modelType,
			}),
		)
	}
	var cmd tea.Cmd
	d.codeInput, cmd = d.codeInput.Update(msg)
	return d, cmd
}

func (d *anthropicOAuthDialog) exchangeCmd(code string) tea.Cmd {
	providerID := string(d.provider.ID)
	modelID := d.model.ID
	modelType := d.modelType
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = ctx // reserved if we move to http.Client with ctx
		info, err := auth.ExchangeCode(code, d.verifier)
		if err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("OAuth exchange failed: %v", err)}
		}
		if err := auth.Set("anthropic", info); err != nil {
			return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("failed to persist OAuth tokens: %v", err)}
		}
		// Configure provider API key as Bearer token for synthetic provider
		token := info.Access
		if !strings.HasPrefix(token, "Bearer ") {
			token = "Bearer " + token
		}
		cfg := config.Get()
		// Build a provider config for 'anthropic-max' mirroring real Anthropic
		providers, _ := config.Providers()
		var realAnth *catwalk.Provider
		for _, p := range providers {
			if p.ID == catwalk.InferenceProviderAnthropic {
				realAnth = &p
				break
			}
		}
		pc := config.ProviderConfig{
			ID:           providerID,
			Name:         "Anthropic Max",
			BaseURL:      "",
			Type:         catwalk.TypeAnthropic,
			APIKey:       token,
			Disable:      false,
			ExtraHeaders: map[string]string{},
			ExtraBody:    map[string]any{},
			ExtraParams:  map[string]string{},
			Models:       nil,
		}
		if realAnth != nil {
			pc.BaseURL = realAnth.APIEndpoint
			pc.Type = realAnth.Type
			pc.Models = realAnth.Models
		}
		// Persist and set in-memory
		_ = cfg.SetConfigField("providers."+providerID, pc)
		cfg.Providers.Set(providerID, pc)

		return oauthSuccessMsg{providerID: providerID, modelID: modelID, modelType: modelType}
	}
}

func (d *anthropicOAuthDialog) View() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Primary).Render("Sign in to Claude Max")
	body := []string{
		t.S().Base.Render("1. A browser window was opened to Claude. Sign in and approve."),
		t.S().Base.Render("2. Press 'o' to open link, 'c/y' to copy link."),
		t.S().Muted.Render(d.url),
		t.S().Base.Render("3. Copy the code shown (it looks like code#state)."),
		t.S().Base.Render("4. Paste below and press Enter."),
		"",
		t.S().Muted.Render("If the browser didn't open: press 'o' to open again."),
	}
	input := d.codeInput.View()
	if d.status != "" {
		body = append(body, "", t.S().Base.Render(d.status))
	}
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		strings.Join(body, "\n"),
		"",
		input,
	)
	return d.style().Render(content)
}

func (d *anthropicOAuthDialog) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(d.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (d *anthropicOAuthDialog) Position() (int, int) {
	row := d.wHeight/4 - 2
	col := d.wWidth/2 - d.width/2
	return row, col
}

func (d *anthropicOAuthDialog) Cursor() *tea.Cursor {
	cursor := d.codeInput.Cursor()
	if cursor == nil {
		return nil
	}
	row, col := d.Position()
	cursor.Y += row + 5
	cursor.X = cursor.X + col + 2
	return cursor
}
