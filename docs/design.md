## Lash: Design (Fork of Charmbracelet Crush)

### Overview
Lash is a login-shell-friendly fork of Charmbracelet Crush that adds Shell and Auto modes while preserving Crush’s Agent mode and built-in Model Context Protocol (MCP) support. It runs headless in any Unix terminal (including over SSH), launches in the previously selected mode (persisted across sessions) and defaults to Auto on first run, and exposes a minimal statusline showing the active mode: Shell, Agent, or Auto.

Reference: [charmbracelet/crush](https://github.com/charmbracelet/crush)

### Goals
- Mandatory MCP: reuse Crush’s native MCP support (stdio/http/SSE) and configuration.
- Shell-like UX: provide a real shell in a PTY; behave like a typical login shell.
- Headless + SSH-safe: no GUI; operates inside SSH sessions; safe non-interactive behavior.
- Minimal UI: single-row statusline and a small confirmation panel for agent-suggested commands.
- Hotkeys for mode switching; configurable.

### Non-goals
- Re-implementing POSIX shell semantics.
- Building a GUI/terminal emulator.
- Replacing `ssh`; we delegate to the system `ssh` as needed.

### High-level Architecture (on top of Crush)
- Agent Mode (existing): retain Crush’s agent loop, MCP plumbing, config, and logging.
- New Shell Mode: spawn the user’s real shell (e.g., `/bin/zsh`) in a PTY; pass-through input/output, support resize, preserve terminal features (e.g., Vim, less, fzf).
- New Auto Mode: if the first token is a valid executable (absolute/relative path or found via PATH), route to Shell; otherwise route to Agent. Selected by default on first run; users can change modes via config or UI. The last selected mode is persisted across sessions.
- Mode Router: central dispatcher that directs input to PTY (Shell) or MCP agent (Agent); applies Auto heuristics when enabled.
- Statusline + Keymap: minimal mode indicator and key hints; configurable keybindings for mode switching and confirmations.
- Non-interactive Guard: if no TTY or invoked with `-c`, immediately exec the real shell to preserve scripts/remote commands.

### Process Model
1) Startup (interactive TTY):
   - Load `crush.json` (kept for compatibility), read Lash extensions.
   - Initialize TUI; active mode is the previously selected mode; if none exists (first run), default to Auto.
   - Spawn PTY with real shell; lazy-start MCP client on first Agent use.
2) Non-interactive or `-c` mode: exec the real shell with original arguments.
3) SSH: works transparently when Lash is a login shell on remote hosts; the TUI renders over SSH.

### Configuration
Crush already uses `crush.json` and supports MCP provider/server configuration and logging. Lash extends the schema via additive fields while preserving compatibility.

Example (JSON):
```json
{
  "$schema": "https://charm.land/crush.json",
  "options": { "debug": false },
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"]
    }
  },
  "lash": {
    "default_mode": "auto",         
    "real_shell": "/bin/zsh",       
    "statusline_position": "bottom",
    "auto_mode_enabled": true,
    "safety": { "confirm_agent_exec": true },
    "keymap": {
      "shell_mode": "ctrl+1",
      "agent_mode": "ctrl+2",
      "auto_mode":  "ctrl+3",
      "confirm":    "ctrl+enter",
      "help":       "ctrl+/"
    }
  }
}
```

MCP in Crush supports stdio, http, and sse transports and environment variable expansion. See: [charmbracelet/crush](https://github.com/charmbracelet/crush)

### UI
- Statusline (one row):
  - Left: Mode [Shell|Agent|Auto]
  - Middle: Context (local or `user@host` if Lash later adds a wrapper to system ssh)
  - Right: Key hints (Ctrl-1/2/3; Ctrl-Enter)
- Agent confirmation panel: Shows suggested command and short explanation; options: Confirm (Ctrl-Enter), Revise (prompt), Cancel (Esc). Hidden unless Agent proposes a command.

### Keyboard Defaults
- Ctrl-1: Shell mode
- Ctrl-2: Agent mode
- Ctrl-3: Auto mode
- Ctrl-Enter: Confirm execution of agent-suggested command in Shell PTY
- Ctrl-/: Help overlay

### Error Handling & Resilience
- PTY failure: render inline error and exec the real shell directly as fallback.
- MCP failure: keep Shell operational; surface non-intrusive error with retry.
- Resize failures ignored but logged at debug level.

### Security
- Mandatory manual confirmation before executing agent-suggested commands.
- Honor `known_hosts` via system `ssh` when used; do not auto-accept.
- Redact likely secrets in logs; no secret persistence.
- Bypass environment variable: `LASH_DISABLE=1` forces immediate exec of real shell at startup.

### Implementation Notes
- Language: Go (same as Crush).
- PTY: `creack/pty` or equivalent to spawn and manage the user’s shell.
- Integrate with Crush’s Bubble Tea stack and event loop; add a Shell subsystem and routing layer.
- Packaging: keep GoReleaser flow; produce macOS and Linux binaries (amd64/arm64).
- Licensing: maintain upstream license headers (FSL-1.1-MIT per Crush), include attribution and notices.

### Alternatives Considered
- Wrapper around Crush (unforked) plus a separate PTY TUI. Rejected for UX cohesion and deeper MCP features already embedded in Crush.
- Other agents like `ai-shell` or `gemini-cli` (no native MCP). Not chosen because MCP is mandatory. References: [BuilderIO/ai-shell](https://github.com/BuilderIO/ai-shell), [google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli)


