## Claude Max OAuth flow — handoff

### What was implemented
- A complete Claude Max (Anthropic) OAuth flow inside the lash TUI, modeled after OpenCode.
- Separate model choices for Anthropic Max (Sonnet/Opus) vs normal Anthropic models.
  - Normal Anthropic uses API key flow as before.
  - Anthropic Max uses an OAuth flow and stores a Bearer token.

### Key files
- `internal/tui/components/dialogs/models/list.go`
  - Adds a new "Anthropic Max" synthetic provider section (`provider.id = "anthropic-max"`) and surfaces Sonnet/Opus.
- `internal/tui/components/dialogs/models/models.go`
  - Routes only `anthropic-max` entries through OAuth.
- `internal/tui/components/dialogs/models/anthropic_oauth.go`
  - New OAuth dialog: opens browser, shows/copies link, accepts code, exchanges tokens asynchronously, and emits selection.
  - Keys: `o` open link, `c/y` copy link, `Enter` submit pasted `code#state`, `Esc` cancel.
- `internal/auth/auth.go`, `internal/auth/anthropic_max.go`
  - Minimal auth store under XDG data and an Anthropic PKCE flow (authorize/exchange/refresh).
- `internal/config/config.go`
  - Accepts Anthropic Bearer tokens for connectivity checks.
  - `GetModel()` resolves models for synthetic `anthropic-max` by falling back to the real Anthropic catalog.
- `internal/llm/provider/anthropic.go`
  - On 401, attempts OAuth refresh (if present) before falling back to configured key.
- `internal/tui/tui.go`
  - On `ModelSelectedMsg`, ensures agents exist, initializes agent if needed, then updates model.

### How it works (runtime)
1. Open the model chooser and select a Sonnet/Opus entry under the "Anthropic Max" section.
2. OAuth dialog appears and opens the browser.
   - Press `o` to re-open the link; `c/y` copies it.
3. Paste the `code#state` into the input and press `Enter`.
   - Exchange runs asynchronously; UI stays responsive.
4. On success:
   - Tokens are stored in `~/.local/share/lash/auth.json`.
   - A provider config `providers.anthropic-max` is created/updated with `Authorization: Bearer ...` and mirrored Anthropic models.
   - The dialog closes and the selected model is applied. The agent is initialized (if uninitialized) and then updated.

### Testing
```bash
# Optional: reset provider cache and auth
rm -f ~/.local/share/lash/providers.json
rm -f ~/.local/share/lash/auth.json

go build -o lash && ./lash -d
```
- Open switch-model dialog, choose an "Anthropic Max" model.
- Verify browser opens; copy link works; paste code and press Enter.
- After success, confirm:
  - `~/.local/share/lash/auth.json` contains `anthropic` OAuth tokens.
  - Your config data includes `providers.anthropic-max` with a Bearer token.
  - Status shows model switched; agent runs with the selected model.

### Troubleshooting
- OAuth shown for non-Max models:
  - Only entries under the synthetic provider `anthropic-max` trigger OAuth; normal `anthropic` entries still use API key.
- Freeze on submit:
  - Exchange now returns a typed message (`oauthSuccessMsg`); UI should not block. If it does, check debug logs and ensure network connectivity.
- Nil agent error on first-time setup:
  - `tui.go` now sets up agents (if missing) and initializes `CoderAgent` on selection before updating the model.
- Missing Sonnet/Opus entries:
  - The chooser uses the provider catalog; if missing, remove `~/.local/share/lash/providers.json` and restart to refresh.

### Notes
- OAuth endpoints mirror OpenCode:
  - Authorize: `https://claude.ai/oauth/authorize` (or console variant)
  - Token: `https://console.anthropic.com/v1/oauth/token`
- We store Bearer tokens for Anthropic Max so the Anthropic SDK uses `Authorization` instead of `x-api-key`.

### Follow-ups (optional)
- Add explicit status messages for OAuth success/failure in the dialog footer.
- Consider persisting which Anthropic Max plan (Opus/Sonnet) was last used to pre-select next time.

