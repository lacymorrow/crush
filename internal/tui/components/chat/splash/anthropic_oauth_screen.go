package splash

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	dialogmodels "github.com/lacymorrow/lash/internal/tui/components/dialogs/models"
	"github.com/lacymorrow/lash/internal/tui/styles"
	"github.com/lacymorrow/lash/internal/tui/util"
)

// oauthSuccessMsg is emitted when token exchange succeeded
type oauthSuccessMsg struct {
	providerID string
	modelID    string
	modelType  config.SelectedModelType
}

// oauthErrorMsg is emitted when token exchange fails and we want to show the error inline
// on the OAuth screen rather than relying solely on the global status bar.
type oauthErrorMsg struct{ err string }

type anthropicOAuthScreen struct {
	provider  catwalk.Provider
	model     catwalk.Model
	modelType config.SelectedModelType

	url      string
	verifier string

	codeInput  textinput.Model
	status     string
	exchanging bool

	width int

	keyOpen   key.Binding
	keyCopy   key.Binding
	keySubmit key.Binding

	lastOpen time.Time
}

func newAnthropicOAuthScreen(option *dialogmodels.ModelOption, modelType config.SelectedModelType) *anthropicOAuthScreen {
	t := styles.CurrentTheme()
	ti := textinput.New()
	ti.Placeholder = "Paste code#state here"
	ti.SetVirtualCursor(false)
	ti.Prompt = "> "
	ti.SetStyles(t.S().TextInput)
	ti.Focus()

	return &anthropicOAuthScreen{
		provider:  option.Provider,
		model:     option.Model,
		modelType: modelType,
		codeInput: ti,
		keyOpen:   key.NewBinding(key.WithKeys("o")),
		keyCopy:   key.NewBinding(key.WithKeys("c", "y")),
		keySubmit: key.NewBinding(key.WithKeys("enter")),
	}
}

func (s *anthropicOAuthScreen) Init() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg {
			url, verifier, err := auth.AuthorizeURL("max")
			if err != nil {
				return util.InfoMsg{Type: util.InfoTypeError, Msg: fmt.Sprintf("failed to start OAuth: %v", err)}
			}
			s.url = url
			s.verifier = verifier
			return nil
		},
		s.openBrowserCmd(),
	)
}

func (s *anthropicOAuthScreen) SetWidth(width int) {
	s.width = width
	// keep input comfortably wide within pane width
	s.codeInput.SetWidth(max(20, width-4))
}

func (s *anthropicOAuthScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case oauthErrorMsg:
		s.status = fmt.Sprintf("Error: %s", m.err)
		s.exchanging = false
		return s, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(m, s.keyOpen):
			return s, s.openBrowserCmd()
		case key.Matches(m, s.keyCopy):
			if s.url != "" {
				_ = clipboard.WriteAll(s.url)
				return s, util.ReportInfo("Auth link copied to clipboard")
			}
			return s, nil
		case key.Matches(m, s.keySubmit):
			code := strings.TrimSpace(s.codeInput.Value())
			if code == "" {
				return s, nil
			}
			if s.exchanging {
				return s, nil
			}
			s.exchanging = true
			s.status = "Exchanging code..."
			return s, s.exchangeCmd(code)
		}
	}
	var cmd tea.Cmd
	s.codeInput, cmd = s.codeInput.Update(msg)
	return s, cmd
}

func (s *anthropicOAuthScreen) openBrowserCmd() tea.Cmd {
	if s.url == "" {
		return nil
	}
	return func() tea.Msg {
		if !s.lastOpen.IsZero() && time.Since(s.lastOpen) < 2*time.Second {
			return nil
		}
		s.lastOpen = time.Now()
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", s.url)
		case "linux":
			cmd = exec.Command("xdg-open", s.url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", s.url)
		default:
			cmd = exec.Command("open", s.url)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
		return util.InfoMsg{Type: util.InfoTypeInfo, Msg: "Opened browser for Claude Max sign-in"}
	}
}

func (s *anthropicOAuthScreen) exchangeCmd(code string) tea.Cmd {
	// Always target the real Anthropic provider after OAuth
	providerID := string(catwalk.InferenceProviderAnthropic)
	modelID := s.model.ID
	modelType := s.modelType
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = ctx
		info, err := auth.ExchangeCode(code, s.verifier)
		if err != nil {
			return oauthErrorMsg{err: fmt.Sprintf("OAuth exchange failed: %v", err)}
		}
		if err := auth.Set("anthropic", info); err != nil {
			return oauthErrorMsg{err: fmt.Sprintf("failed to persist OAuth tokens: %v", err)}
		}
		// Attempt to create a real API key using the OAuth access token.
		accessToken := info.Access
		apiKey := ""
		{
			req, _ := http.NewRequest("POST", "https://api.anthropic.com/api/oauth/claude_cli/create_api_key", strings.NewReader(""))
			req.Header.Set("Authorization", "Bearer "+accessToken)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json, text/plain, */*")
			httpClient := &http.Client{Timeout: 30 * time.Second}
			resp, err := httpClient.Do(req)
			if err == nil && resp != nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					var out struct {
						RawKey string `json:"raw_key"`
					}
					if derr := json.NewDecoder(resp.Body).Decode(&out); derr == nil && out.RawKey != "" {
						apiKey = out.RawKey
					}
				}
			}
		}

		// Configure provider with API key if created; otherwise fall back to Bearer token
		finalAPIValue := ""
		if apiKey != "" {
			finalAPIValue = apiKey
		} else {
			finalAPIValue = accessToken
			if !strings.HasPrefix(finalAPIValue, "Bearer ") {
				finalAPIValue = "Bearer " + finalAPIValue
			}
		}
		cfg := config.Get()
		pc, ok := cfg.Providers.Get(providerID)
		if !ok {
			// Seed from known providers so models are populated for agent/provider initialization
			known, _ := config.Providers()
			for _, kp := range known {
				if string(kp.ID) == providerID {
					pc = config.ProviderConfig{
						ID:           providerID,
						Name:         kp.Name,
						BaseURL:      kp.APIEndpoint,
						Type:         kp.Type,
						ExtraHeaders: map[string]string{},
						Models:       kp.Models,
					}
					break
				}
			}
			if pc.ID == "" {
				pc = config.ProviderConfig{ID: providerID, Name: "Anthropic", Type: catwalk.TypeAnthropic, BaseURL: config.DefaultAnthropicBaseURL, ExtraHeaders: map[string]string{}}
			}
		} else if len(pc.Models) == 0 {
			// Backfill models if provider exists but models are empty
			known, _ := config.Providers()
			for _, kp := range known {
				if string(kp.ID) == providerID {
					pc.Models = kp.Models
					if pc.BaseURL == "" {
						pc.BaseURL = kp.APIEndpoint
					}
					if pc.Type == "" {
						pc.Type = kp.Type
					}
					break
				}
			}
		}
		if pc.ExtraHeaders == nil {
			pc.ExtraHeaders = map[string]string{}
		}
		pc.APIKey = finalAPIValue
		pc.Disable = false
		pc.ExtraHeaders[config.HeaderAnthropicVersion] = config.DefaultAnthropicAPIVer
		cfg.Providers.Set(providerID, pc)
		_ = cfg.SetConfigField("providers."+providerID, pc)

		// Ensure the selected model references the real provider (not synthetic 'anthropic-max')
		// so downstream components can resolve provider config correctly.
		return oauthSuccessMsg{providerID: providerID, modelID: modelID, modelType: modelType}
	}
}

func (s *anthropicOAuthScreen) View() string {
	t := styles.CurrentTheme()
	title := t.S().Base.Foreground(t.Primary).Bold(true).Render("Sign in to Claude Max")
	maxW := s.width - 4
	if maxW < 20 {
		maxW = 20
	}
	urlText := s.url
	if urlText != "" {
		urlText = wrapUnbroken(urlText, maxW)
	}
	body := []string{
		t.S().Base.Render("1. A browser window was opened to Claude. Sign in and approve."),
		t.S().Base.Render("2. Press 'o' to open link, 'c/y' to copy link."),
		t.S().Muted.Render(urlText),
		t.S().Base.Render("3. Copy the code shown (it looks like code#state)."),
		t.S().Base.Render("4. Paste below and press Enter."),
	}
	input := s.codeInput.View()
	if s.status != "" {
		body = append(body, "", t.S().Base.Render(s.status))
	}
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		strings.Join(body, "\n"),
		"",
		input,
	)
	return content
}

func (s *anthropicOAuthScreen) Cursor() *tea.Cursor {
	return s.codeInput.Cursor()
}

// wrapUnbroken inserts newlines into very long, space-less text (like URLs)
// so that it can wrap within the available pane width.
func wrapUnbroken(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}
	var b strings.Builder
	for i := 0; i < len(text); i += width {
		end := i + width
		if end > len(text) {
			end = len(text)
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(text[i:end])
	}
	return b.String()
}
