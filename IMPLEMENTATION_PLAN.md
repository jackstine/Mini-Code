# Implementation Plan: Logging Specification

## Overview

This plan covers the implementation of the logging system as defined in `specs/logging.md`. The harness application requires two distinct logging systems:

1. **Server Logging** - Debug harness internals (stderr, controlled by `HARNESS_LOG_LEVEL`)
2. **Agent Interaction Logging** - View conversation flow (file, controlled by `HARNESS_AGENT_LOG`)

## Current State

- No structured logging exists - only `log.Fatal()` for startup errors and `fmt.Printf()` for startup messages
- The `EventHandler` interface already captures agent events (text, tool calls, tool results, reasoning)
- Environment variable parsing exists for `HARNESS_MODEL`, `HARNESS_ADDR`, `HARNESS_SYSTEM_PROMPT`

## Acceptance Criteria

### Server Logging
- [ ] Logs written to stderr with configurable level (DEBUG, INFO, WARN, ERROR)
- [ ] Supports text and JSON output formats via `HARNESS_LOG_FORMAT`
- [ ] Supports category filtering via `HARNESS_LOG_CATEGORIES` (http, sse, api, tool, harness)
- [ ] Default level is INFO when `HARNESS_LOG_LEVEL` is unset
- [ ] Log entries include: timestamp, level, category, message, and key-value fields
- [ ] Sensitive data (API keys, tokens) are never logged or are redacted
- [ ] Level check before expensive formatting operations (performance)

### Agent Interaction Logging
- [ ] Logs written to file specified by `HARNESS_AGENT_LOG` (disabled if unset)
- [ ] Supports text and JSON output formats via `HARNESS_AGENT_LOG_FORMAT`
- [ ] Captures: user prompts, assistant responses, tool calls with inputs, tool results
- [ ] Text format uses human-readable separator blocks (`=== TIMESTAMP TYPE ===`)
- [ ] JSON format uses NDJSON (one JSON object per line)
- [ ] File rotation: 10MB max size, 5 max files, timestamp suffix on rotated files

### Integration Points
- [ ] HTTP requests logged with method, path, status, duration
- [ ] SSE client connect/disconnect logged with client_id
- [ ] API requests logged with model, message count, token counts, duration
- [ ] Tool execution logged with tool name, id, duration, success/failure
- [ ] Agent loop logged with turn count, total duration

---

## Implementation Items

### 1. Create Logger Package (`pkg/log/`)

**Priority: HIGH** - Foundation for all other logging work

**Why:** Provides the core logging infrastructure that all other components depend on. Without this, no logging can be added anywhere in the application.

**Files to create:**
- `pkg/log/logger.go` - Logger interface and server logger implementation
- `pkg/log/agent.go` - Agent interaction logger implementation
- `pkg/log/config.go` - Configuration parsing from environment variables
- `pkg/log/format.go` - Text and JSON formatters
- `pkg/log/rotate.go` - File rotation for agent logs

**Implementation details:**

```go
// Logger interface (as specified)
type Logger interface {
    Debug(category string, message string, fields ...Field)
    Info(category string, message string, fields ...Field)
    Warn(category string, message string, fields ...Field)
    Error(category string, message string, fields ...Field)
    IsDebugEnabled() bool
}

type Field struct {
    Key   string
    Value any
}

// AgentLogger interface (as specified)
type AgentLogger interface {
    LogUser(content string)
    LogAssistant(content string)
    LogToolCall(id, name string, input json.RawMessage)
    LogToolResult(id string, result string, isError bool)
}
```

**Tests to add:**
- `pkg/log/logger_test.go` - Level filtering, category filtering, format output
- `pkg/log/agent_test.go` - Event logging, file writing, rotation
- `pkg/log/config_test.go` - Environment variable parsing, defaults

---

### 2. Create Log Configuration Type

**Priority: HIGH** - Required before logger can be initialized

**Why:** Centralizes all logging configuration in one place, making it easy to parse from environment and pass to logger constructors.

**Add to `pkg/log/config.go`:**

```go
type LogConfig struct {
    Level      string   // DEBUG, INFO, WARN, ERROR (default: INFO)
    Format     string   // text, json (default: text)
    Categories []string // Empty means all
    Output     io.Writer // Default: os.Stderr
}

type AgentLogConfig struct {
    FilePath string // Empty means disabled
    Format   string // text, json (default: text)
    MaxSize  int64  // Default: 10MB
    MaxFiles int    // Default: 5
}

func LoadFromEnv() (LogConfig, AgentLogConfig)
```

**Environment variables:**
- `HARNESS_LOG_LEVEL` → `LogConfig.Level`
- `HARNESS_LOG_FORMAT` → `LogConfig.Format`
- `HARNESS_LOG_CATEGORIES` → `LogConfig.Categories` (comma-separated)
- `HARNESS_AGENT_LOG` → `AgentLogConfig.FilePath`
- `HARNESS_AGENT_LOG_FORMAT` → `AgentLogConfig.Format`

---

### 3. Implement Server Logger

**Priority: HIGH** - Core logging functionality

**Why:** Enables infrastructure debugging without exposing conversation content. Critical for production monitoring and troubleshooting.

**Implementation in `pkg/log/logger.go`:**
- Thread-safe (mutex-protected writes)
- Level filtering (skip if below configured level)
- Category filtering (skip if category not in allowed list)
- Text format: `TIMESTAMP LEVEL [CATEGORY] MESSAGE key=value...`
- JSON format: Single-line JSON object with all fields

**Key behaviors:**
- `IsDebugEnabled()` method for performance optimization
- Lazy field evaluation pattern for expensive debug logging
- Write directly to `io.Writer` (default stderr)

---

### 4. Implement Agent Interaction Logger

**Priority: HIGH** - Conversation debugging capability

**Why:** Enables debugging of agent behavior by capturing full conversation content. Essential for understanding what the agent did and why.

**Implementation in `pkg/log/agent.go`:**
- File-based output (configurable path)
- Thread-safe file writes
- Text format with clear visual separators
- JSON format as NDJSON

**Text format example:**
```
=== 2024-01-15T10:30:45.123Z USER ===
What's in the config.json file?

=== 2024-01-15T10:30:46.000Z ASSISTANT ===
I'll read that file for you.

=== 2024-01-15T10:30:46.010Z TOOL_CALL [read] id=toolu_123 ===
{"path": "/config.json"}

=== 2024-01-15T10:30:46.025Z TOOL_RESULT [toolu_123] success ===
port=8080
host=localhost
```

---

### 5. Implement File Rotation

**Priority: MEDIUM** - Prevents disk space exhaustion

**Why:** Agent logs can grow large over extended sessions. Rotation prevents filling disk and maintains manageable file sizes.

**Implementation in `pkg/log/rotate.go`:**
- Check file size before each write
- When size exceeds 10MB:
  - Close current file
  - Rename to `{filename}.{timestamp}` format
  - Delete oldest if more than 5 files exist
  - Open new file

---

### 6. Initialize Loggers in main.go

**Priority: HIGH** - Enables logging throughout application

**Why:** Loggers must be created at startup and passed to components that need them.

**Modify `cmd/harness/main.go`:**
```go
// Parse logging configuration
logConfig, agentLogConfig := log.LoadFromEnv()

// Create server logger
logger := log.NewLogger(logConfig)

// Create agent logger (may be nil if disabled)
agentLogger := log.NewAgentLogger(agentLogConfig)

// Pass to harness and server
```

---

### 7. Add HTTP Request Logging

**Priority: MEDIUM** - Track incoming requests

**Why:** Essential for debugging connectivity issues and understanding request patterns.

**Modify `pkg/server/server.go`:**
- Add logger field to Server struct
- Log on request received: method, path, content_length
- Log on response sent: method, path, status, duration_ms
- Log validation errors at WARN level

**Example output:**
```
INFO [http] Request received method=POST path=/prompt content_length=45
INFO [http] Response sent method=POST path=/prompt status=200 duration_ms=12
WARN [http] Request validation failed method=POST path=/prompt error="content is required"
```

---

### 8. Add SSE Client Logging

**Priority: MEDIUM** - Track TUI connections

**Why:** Critical for debugging SSE delivery issues and understanding client lifecycle.

**Modify `pkg/server/sse.go`:**
- Log client connected with client_id, remote_addr
- Log client disconnected with client_id, duration_s
- Log events sent at DEBUG level: client_id, event_type, bytes
- Log heartbeats at DEBUG level
- Log buffer full (skipped event) at WARN level

**Example output:**
```
INFO [sse] Client connected client_id=1 remote_addr=127.0.0.1:54321
DEBUG [sse] Event sent client_id=1 event_type=text bytes=256
INFO [sse] Heartbeat sent client_id=1
INFO [sse] Client disconnected client_id=1 duration_s=120
```

---

### 9. Add API Request Logging

**Priority: MEDIUM** - Track Anthropic API calls

**Why:** Essential for understanding API performance, costs, and debugging API-related issues.

**Modify `pkg/harness/harness.go`:**
- Log request sent: model, message count, tool count
- Log response received: input_tokens, output_tokens, duration_ms
- Log errors: error message, model, retry info
- Log rate limits at WARN level

**Example output:**
```
INFO [api] Request sent model=claude-3-haiku messages=3 tools=3
INFO [api] Response received input_tokens=150 output_tokens=89 duration_ms=1523
ERROR [api] Request failed model=claude-3-haiku error="rate_limit_exceeded" retry_after_s=30
```

---

### 10. Add Tool Execution Logging

**Priority: MEDIUM** - Track tool lifecycle

**Why:** Helps debug tool failures and performance issues.

**Modify `pkg/harness/harness.go`:**
- Log execution started: tool name, id
- Log execution completed: tool name, id, duration_ms, success
- Log execution failed: tool name, id, error, duration_ms
- Log slow execution (>5s) at WARN level
- Log tool input/output at DEBUG level (truncated)

**Example output:**
```
INFO [tool] Execution started tool=read id=toolu_123
DEBUG [tool] Tool input tool=read id=toolu_123 input={"path":"/config.json"}
INFO [tool] Execution completed tool=read id=toolu_123 duration_ms=15 success=true
WARN [tool] Slow execution tool=read id=toolu_123 duration_ms=6500
```

---

### 11. Add Harness Loop Logging

**Priority: MEDIUM** - Track agent loop orchestration

**Why:** Provides high-level visibility into agent execution patterns.

**Modify `pkg/harness/harness.go`:**
- Log loop started: prompt_length
- Log turn completed at DEBUG level: turn number, tool_calls count
- Log loop completed: turns, total_duration_ms

**Example output:**
```
INFO [harness] Agent loop started prompt_length=45
DEBUG [harness] Turn completed turn=1 tool_calls=1
INFO [harness] Agent loop completed turns=2 total_duration_ms=2700
```

---

### 12. Integrate Agent Logger with EventHandler

**Priority: HIGH** - Enables conversation logging

**Why:** The EventHandler already receives all agent events. Connecting it to the agent logger enables conversation logging with minimal code changes.

**Create wrapper in `pkg/log/event_handler.go`:**
```go
// LoggingEventHandler wraps an EventHandler and logs to AgentLogger
type LoggingEventHandler struct {
    wrapped     harness.EventHandler
    agentLogger AgentLogger
}

func (h *LoggingEventHandler) OnText(text string) {
    if h.agentLogger != nil {
        h.agentLogger.LogAssistant(text)
    }
    if h.wrapped != nil {
        h.wrapped.OnText(text)
    }
}
// ... similar for other methods
```

**Modify `cmd/harness/main.go`:**
- Wrap the SSE event handler with LoggingEventHandler
- Log user prompts in HandlePrompt before broadcasting

---

### 13. Add Sensitive Data Redaction

**Priority: HIGH** - Security requirement

**Why:** Prevents accidental exposure of API keys and credentials in logs.

**Implementation in `pkg/log/redact.go`:**
- Redact `authorization` header values
- Redact any field containing "key", "token", "secret", "password"
- Replace with `***redacted***`

---

## Implementation Order

The items should be implemented in this order to minimize integration issues:

1. **Logger Package Foundation** (Items 1-4)
   - Config, Logger interface, Server Logger, Agent Logger

2. **Application Integration** (Item 6)
   - Initialize loggers in main.go

3. **Agent Logging Integration** (Item 12)
   - Connect agent logger to event handler

4. **Server Logging Points** (Items 7-11)
   - HTTP, SSE, API, Tool, Harness logging

5. **Supporting Features** (Items 5, 13)
   - File rotation, Sensitive data redaction

## Testing Strategy

Each logging component should have:
- Unit tests for log level filtering
- Unit tests for category filtering
- Unit tests for format output (text and JSON)
- Integration tests verifying logs appear at correct points

Agent logger tests should:
- Test file writing and content format
- Test rotation triggers at size threshold
- Test max files cleanup

## Non-Goals (Out of Scope)

- TUI-side logging (spec is backend-focused)
- Log aggregation or shipping
- Log searching/filtering tools
- Metrics or tracing integration
