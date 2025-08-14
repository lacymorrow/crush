# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Lash is a terminal-based AI assistant for software developers, built in Go. It's a fork of Charmbracelet's Crush with additional features including Shell, Agent, and Auto modes, plus MCP (Model Context Protocol) support.

## Essential Development Commands

### Build and Run
```bash
# Build the project
go build -o lash cmd/lash/main.go

# Run with profiling enabled
task dev

# Run tests
task test

# Run specific test
go test -v ./internal/tui/components -run TestComponentName
```

### Code Quality
```bash
# Run linting (required before commits)
task lint

# Run linting with auto-fixes
task lint-fix

# Format code with gofumpt (required)
task fmt
```

### Database Operations
```bash
# Generate type-safe SQL code from queries
sqlc generate

# Create new migration
goose -dir internal/db/migrations create migration_name sql

# Apply migrations (happens automatically on startup)
goose -dir internal/db/migrations sqlite3 ~/.config/crush/crush.db up
```

### Schema Generation
```bash
# Generate JSON schema for configuration validation
task schema
```

## Architecture Overview

### Core Architecture Pattern
The codebase follows clean architecture principles with clear separation:
- **CLI Layer** (`/internal/cmd/`): Cobra commands that handle user input
- **Application Layer** (`/internal/app/`): Main application orchestration and LSP management
- **Domain Layer** (`/internal/llm/`, `/internal/session/`): Core business logic
- **Infrastructure Layer** (`/internal/db/`, `/internal/shell/`): External integrations

### Key Architectural Components

1. **LLM Provider System** (`/internal/llm/provider/`)
   - Interface-based design allowing multiple provider implementations
   - Each provider implements the `Provider` interface with streaming support
   - Providers handle their own authentication and API communication

2. **Tool System** (`/internal/llm/tools/`)
   - Each tool implements the `Tool` interface with Execute method
   - Tools are registered in a central registry
   - Permission system controls tool execution per session

3. **TUI Architecture** (`/internal/tui/`)
   - Built on Bubble Tea's Model-Update-View pattern
   - Components are composable and reusable
   - Pages manage state and coordinate components
   - Event system for inter-component communication

4. **Session Management** (`/internal/session/`)
   - Sessions persist conversation context in SQLite
   - Each session maintains its own configuration and permissions
   - Messages are stored with attachments support

5. **MCP Integration** (`/internal/llm/agent/mcp/`)
   - Supports stdio, HTTP, and SSE transports
   - Dynamic tool discovery and registration
   - Handles server lifecycle management

### Database Design
- SQLite for local persistence at `~/.config/crush/crush.db`
- Migrations in `/internal/db/migrations/`
- SQLC generates type-safe Go code from SQL queries in `/internal/db/queries/`
- All database operations go through generated code for type safety

### Configuration System
- JSON-based configuration with schema validation
- Config file at `~/.config/crush/crush.json`
- Schema defined in `schema.json` for IDE autocomplete
- Environment variable overrides supported

## Testing Approach

### Test Structure
- Unit tests alongside code files (`*_test.go`)
- Golden file testing for UI components in `/testdata/`
- Mock providers in `/internal/llm/provider/mock/`
- Integration tests for database operations

### Running Tests
```bash
# Run all tests with coverage
go test -coverprofile=coverage.out ./...

# Update golden files when UI changes are intentional
go test ./internal/tui/components -update

# Run tests with race detection
go test -race ./...
```

## Code Standards

### Go Conventions
- Use gofumpt for formatting (enforced by linter)
- Error wrapping with `fmt.Errorf` and `%w` verb
- Context propagation for cancellation and timeouts
- Structured logging with slog package

### Error Handling Pattern
```go
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### Interface Design
- Define interfaces where implementations are used, not where they're implemented
- Keep interfaces small and focused
- Use interface segregation principle

## Adding New Features

### Adding a New LLM Provider
1. Create new provider in `/internal/llm/provider/yourprovider/`
2. Implement the `Provider` interface
3. Register in `/internal/llm/provider/registry.go`
4. Add configuration schema to `schema.json`

### Adding a New Tool
1. Create tool in `/internal/llm/tools/`
2. Implement the `Tool` interface
3. Register in tool registry
4. Add to appropriate permission groups

### Adding a New TUI Component
1. Create component in `/internal/tui/components/`
2. Implement Bubble Tea Model interface
3. Add golden test with expected output
4. Integrate into appropriate page

## Important Implementation Details

### Streaming Response Handling
- All LLM providers must support streaming responses
- Use channels for streaming data between goroutines
- Handle context cancellation properly to avoid goroutine leaks

### Database Migrations
- Never modify existing migrations
- Always create new migrations for schema changes
- Test migrations both up and down

### UI State Management
- Keep component state minimal and focused
- Use messages for state updates
- Avoid direct component coupling

### Security Considerations
- Never log sensitive information (API keys, tokens)
- Validate all user input before processing
- Use prepared statements for all database queries (handled by SQLC)