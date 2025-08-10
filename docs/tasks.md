## Lash: Tasks (Fork of Charmbracelet Crush)

### Milestone M0 — Fork & Scaffolding
- T0.1: Fork `charmbracelet/crush`; set module name and licensing/NOTICE updates.
  - Acceptance: `lash --version` prints fork info; license headers preserved.
- T0.2: Add `lash` config namespace to `crush.json` schema and parser.
  - Acceptance: `lash.default_mode` (defaults to `auto`), `lash.keymap`, `lash.real_shell` parsed with defaults; last selected mode is persisted and takes precedence on startup.
- T0.3: Integrate minimal statusline rendering for mode and key hints.
  - Acceptance: Statusline shows Shell/Agent/Auto; help overlay toggles.

### Milestone M1 — Shell Mode (PTY)
- T1.1: Implement PTY manager to spawn real shell (`$SHELL` or configured).
  - Acceptance: Run interactive commands; arrow keys; resize works.
- T1.2: Non-interactive guard: if no TTY or `-c`, exec real shell with args.
  - Acceptance: Remote commands over SSH unaffected when set as login shell.
- T1.3: Route input/output between TUI and PTY when Shell mode is active.
  - Acceptance: Latency and echo behave like a native terminal.

### Milestone M2 — Agent Mode (Preserve + Confirm)
- T2.1: Preserve existing Crush Agent flow and MCP configuration.
  - Acceptance: MCP servers connect via stdio/http/sse per config.
- T2.2: Add suggestion capture and confirmation panel.
  - Acceptance: Suggested command executes in PTY only on Ctrl-Enter.

### Milestone M3 — Auto Mode & Routing
- T3.1: Add routing by command existence (no prefixes): if first token resolves to an executable → Shell; otherwise → Agent.
  - Acceptance: PATH/executable routing works; statusline reflects mode.
- T3.2: Add configurable keybindings: Ctrl-1/2/3, Ctrl-Enter, Ctrl-/.
  - Acceptance: Overrides via `crush.json` under `lash.keymap`.

### Milestone M4 — SSH Interop & Polish
- T4.1: Optional `lash ssh user@host` command: delegate to system `ssh` in PTY.
  - Acceptance: Behaves like native terminal; resize propagated.
- T4.2: Logging/redaction: integrate redact patterns; `--debug` flag.
  - Acceptance: Logs rotate; secrets redacted; debug toggles verbosity.
- T4.3: Error handling: MCP failure degrades gracefully; PTY fallback.
  - Acceptance: Killing MCP keeps Shell usable; restart works.

### Milestone M5 — Packaging & Login Shell Docs
- T5.1: GoReleaser targets macOS/Linux (amd64/arm64) with static builds.
  - Acceptance: CI artifacts built; checksums; signed if configured.
- T5.2: Install docs for login shell (`/etc/shells`, `chsh`) and bypass.
  - Acceptance: User can set as login shell; `LASH_DISABLE=1` bypass works.

### Testing
- Unit: config parsing, keymap, router, redaction.
- Integration: PTY lifecycle, resize, login-shell non-interactive exec, confirmation guard, MCP I/O.
- E2E: expect-like scripts for Shell/Agent/Auto flows and SSH interop.

### Backlog
- SSH profiles UI; multi-pane/tab support; native MCP client (no subprocess); model/tool selection palette.

### References
- Base + MCP: [charmbracelet/crush](https://github.com/charmbracelet/crush)
- Related agents (no native MCP): [BuilderIO/ai-shell](https://github.com/BuilderIO/ai-shell), [google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli)


