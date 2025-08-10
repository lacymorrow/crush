## Lash: Requirements (Fork of Charmbracelet Crush)

### Must-haves
- MCP is mandatory. Use Crush’s built-in MCP mechanisms and configuration; do not remove or regress them. Reference: [charmbracelet/crush](https://github.com/charmbracelet/crush)
- Headless operation in any Unix terminal, including over SSH.
- Behaves like a normal login shell by default:
  - Interactive TTY: start in the previously selected mode; on first run default to Auto. A real shell is always available in a PTY when Shell mode is active.
  - Non-interactive or `-c`: immediately exec the real shell with original args (preserve scripts/remote commands).
- Minimal UI with a one-line statusline showing current mode: Shell | Agent | Auto.
- Mode switching via keybindings (default Ctrl-1/2/3); configurable.
- Agent-suggested commands never auto-execute; require explicit confirmation (default Ctrl-Enter).
- Configuration via `crush.json` with a `lash` extension block; environment and flags override.
- Logging compatible with Crush’s logging; add redaction for likely secrets.

### Functional Requirements
1) Shell Mode
   - Spawn real shell (`$SHELL` or configured binary) in a PTY.
   - Pass-through keystrokes; support terminal features (vim, less, fzf, tmux inside).
   - Handle window resize events and propagate to PTY.
   - Execute confirmed commands from Agent in the same PTY context.

2) Agent Mode
   - Keep Crush’s agent experience intact.
   - MCP servers via stdio/http/sse as configured; environment expansion supported.
   - Stream responses; capture suggested command and explanation for confirmation.

3) Auto Mode
   - Router enabled by default on first run; heuristics:
     - `ai:` or `?` prefix → Agent
     - Otherwise → Shell
   - Provide an override keybinding to temporarily route the next line to Agent.

4) Login Shell Safety
   - Works as the user’s login shell (`chsh`) and within SSH sessions.
   - Non-interactive behavior falls through to real shell.
   - Bypass via `LASH_DISABLE=1` env var to exec real shell immediately.

5) SSH Interop
   - When users run `lash` on a remote host via SSH, it should render as usual.
   - Optional convenience: `lash ssh user@host` delegates to system `ssh` in a PTY.

6) Configuration
   - Preserve `crush.json` schema; add `lash` namespaced keys for mode defaults, keymap, safety toggles, shell path, router settings. Persist the last selected mode across sessions; if not present, use `lash.default_mode` (default: `auto`).
   - Hot-reload not required; re-read on startup is sufficient.

7) Logging/Observability
   - Reuse Crush’s logging; log to project-local and/or state directory per Crush behavior.
   - Redact likely secrets via regex patterns; toggle debug via config/flag.

### Non-functional Requirements
- Startup latency for interactive sessions: < 200ms on typical dev hardware.
- Memory: Shell-only baseline < 60MB; with Agent active < 140MB.
- CPU: idle usage negligible; streaming under 1 core in common cases.
- Binaries for macOS and Linux (amd64, arm64); no GUI dependencies.

### Security
- Confirm-to-execute is on by default and cannot be disabled without an explicit config option.
- Honor SSH `known_hosts`; do not silently accept new host keys.
- No secrets stored; redact in logs and transcripts.

### Compatibility & Limits
- Compatible with POSIX shells; does not aim to replace shell scripting semantics.
- Does not interfere with `scp`/`sftp` or non-interactive SSH commands when set as login shell.

### External References
- Crush (base, MCP, config, logging): [charmbracelet/crush](https://github.com/charmbracelet/crush)
- Optional inspirations (no native MCP):
  - [BuilderIO/ai-shell](https://github.com/BuilderIO/ai-shell)
  - [google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli)


