# Logging Specification

## Overview

The harness application uses two distinct logging systems to serve different debugging needs:

| System | Purpose | Output | Configuration |
|--------|---------|--------|---------------|
| **Server Logging** | Debug harness internals | stderr | `HARNESS_LOG_LEVEL` |
| **Agent Interaction Logging** | View conversation flow | file | `HARNESS_AGENT_LOG` |

These systems are independent and can be used together or separately.

## Server Logging

Server logs write to **stderr** and are controlled by `HARNESS_LOG_LEVEL`. They help debug the harness infrastructure without showing conversation content.

### Categories

| Category | Description | Example Events |
|----------|-------------|----------------|
| `http` | HTTP request handling | Request received, response sent, validation errors |
| `sse` | SSE client management | Client connected, event delivered, client disconnected |
| `api` | Anthropic API communication | Request sent, response received, rate limits, errors |
| `tool` | Tool execution lifecycle | Execution started, completed, failed, slow execution |
| `harness` | Agent loop orchestration | Loop started, turn completed, loop terminated |

### What Server Logs Show

Server logs show **metadata and status**, not content:

```
INFO  [http] Request received method=POST path=/prompt content_length=45
INFO  [api] Request sent model=claude-3-haiku messages=3 tools=3
INFO  [api] Response received input_tokens=150 output_tokens=89 duration_ms=1523
INFO  [tool] Execution started tool=read id=toolu_123
INFO  [tool] Execution completed tool=read duration_ms=15 success=true
INFO  [harness] Agent loop completed turns=2 duration_ms=2700
```

Notice: You see *that* a request was made, but not *what* the user asked or *what* the agent responded.

## Agent Interaction Logging

Agent interaction logs write to a **file** specified by `HARNESS_AGENT_LOG`. They capture the full conversation flow including content.

### Event Types

| Type | Description | Content |
|------|-------------|---------|
| `user` | User prompt submitted | Full prompt text |
| `assistant` | Agent text response | Full response text |
| `tool_call` | Agent requested a tool | Tool name and full input JSON |
| `tool_result` | Tool execution result | Full result or error message |

### What Agent Logs Show

Agent logs show **full content** of the conversation:

```
=== 2024-01-15T10:30:45.123Z USER ===
What's in the config.json file?

=== 2024-01-15T10:30:46.000Z ASSISTANT ===
I'll read that file for you.

=== 2024-01-15T10:30:46.010Z TOOL_CALL [read] id=toolu_123 ===
{"path": "/config.json"}

=== 2024-01-15T10:30:46.025Z TOOL_RESULT [toolu_123] success ===
{"content": "port=8080\nhost=localhost"}

=== 2024-01-15T10:30:47.000Z ASSISTANT ===
The config file contains:
- port: 8080
- host: localhost
```

## When to Use Each

| Debugging Question | Use |
|--------------------|-----|
| "Is the server receiving requests?" | Server logs (`http`) |
| "Is the TUI connected via SSE?" | Server logs (`sse`) |
| "Is the API responding?" | Server logs (`api`) |
| "What did the user ask?" | Agent logs |
| "What did the agent respond?" | Agent logs |
| "What input was sent to the tool?" | Agent logs |
| "What did the tool return?" | Agent logs |

## Server Log Levels

Server logs support four levels, controlled by `HARNESS_LOG_LEVEL`:

| Level | Numeric | Description | Use Case |
|-------|---------|-------------|----------|
| `DEBUG` | 0 | Verbose diagnostic information | Development, deep debugging |
| `INFO` | 1 | Normal operational events | Production monitoring |
| `WARN` | 2 | Potentially harmful situations | Degraded functionality |
| `ERROR` | 3 | Error events, application continues | Failures requiring attention |

### Level Guidelines

**DEBUG:**
- HTTP request/response headers
- SSE event payloads (truncated)
- Full API request parameters
- Tool input/output (truncated)
- Internal state changes

**INFO:**
- HTTP request received (method, path, status)
- SSE client connected/disconnected
- API request sent (model, token count)
- Tool execution started/completed
- Agent loop started/terminated

**WARN:**
- API rate limiting detected
- Tool execution slow (> 5 seconds)
- SSE client reconnection
- Deprecated feature usage

**ERROR:**
- API request failed
- Tool execution failed
- SSE delivery failed
- Configuration errors

## Configuration

### Server Logging (stderr)

| Variable | Default | Description |
|----------|---------|-------------|
| `HARNESS_LOG_LEVEL` | `INFO` | Minimum log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `HARNESS_LOG_FORMAT` | `text` | Output format: `text` or `json` |
| `HARNESS_LOG_CATEGORIES` | (all) | Comma-separated categories to enable: `http,sse,api,tool,harness` |

### Agent Interaction Logging (file)

| Variable | Default | Description |
|----------|---------|-------------|
| `HARNESS_AGENT_LOG` | (disabled) | File path for agent logs. Not written if unset. |
| `HARNESS_AGENT_LOG_FORMAT` | `text` | Output format: `text` or `json` |

### Examples

```bash
# Server logs only (stderr) - see infrastructure activity
HARNESS_LOG_LEVEL=INFO go run cmd/harness/main.go

# Debug server logs - verbose infrastructure details
HARNESS_LOG_LEVEL=DEBUG go run cmd/harness/main.go

# Agent logs only (file) - see conversation content
HARNESS_AGENT_LOG=~/.harness/agent.log go run cmd/harness/main.go

# Both systems together - full visibility
HARNESS_LOG_LEVEL=DEBUG HARNESS_AGENT_LOG=~/.harness/agent.log go run cmd/harness/main.go

# Filter server logs to specific categories
HARNESS_LOG_CATEGORIES=api,tool HARNESS_LOG_LEVEL=DEBUG go run cmd/harness/main.go

# JSON format for production log aggregation
HARNESS_LOG_FORMAT=json HARNESS_AGENT_LOG_FORMAT=json HARNESS_AGENT_LOG=/var/log/harness/agent.log go run cmd/harness/main.go
```

### Viewing Logs

```bash
# Watch server logs in real-time (they go to stderr)
go run cmd/harness/main.go 2>&1 | grep -E '\[(api|tool)\]'

# Watch agent logs in real-time
tail -f ~/.harness/agent.log

# Both in separate terminals
# Terminal 1: HARNESS_LOG_LEVEL=DEBUG go run cmd/harness/main.go
# Terminal 2: tail -f ~/.harness/agent.log
```

## Output Formats

### Server Log Format (stderr)

**Text Format** (`HARNESS_LOG_FORMAT=text`, default):

```
TIMESTAMP LEVEL [CATEGORY] MESSAGE key=value key2=value2
```

| Field | Format | Description |
|-------|--------|-------------|
| Timestamp | ISO 8601 | `2024-01-15T10:30:45.123Z` |
| Level | Uppercase | `DEBUG`, `INFO`, `WARN`, `ERROR` |
| Category | Bracketed | `[http]`, `[api]`, `[tool]` |
| Message | Free text | Human-readable description |
| Fields | Key-value pairs | Structured data |

**JSON Format** (`HARNESS_LOG_FORMAT=json`):

```json
{"timestamp":"2024-01-15T10:30:45.123Z","level":"INFO","category":"http","message":"Request received","method":"POST","path":"/prompt"}
```

Each log entry is a single JSON object on one line (NDJSON format).

### Agent Log Format (file)

**Text Format** (`HARNESS_AGENT_LOG_FORMAT=text`, default):

```
=== TIMESTAMP TYPE [details] ===
content
```

Designed for human review of conversations with clear visual separation.

**JSON Format** (`HARNESS_AGENT_LOG_FORMAT=json`):

```json
{"timestamp":"2024-01-15T10:30:45.123Z","type":"user","content":"What's in config.json?"}
{"timestamp":"2024-01-15T10:30:46.000Z","type":"assistant","content":"I'll read that file."}
{"timestamp":"2024-01-15T10:30:46.010Z","type":"tool_call","id":"toolu_123","name":"read","input":{"path":"/config.json"}}
{"timestamp":"2024-01-15T10:30:46.025Z","type":"tool_result","id":"toolu_123","success":true,"result":"port=8080"}
```

Each entry is NDJSON, suitable for log aggregation or programmatic analysis.

## Server Log Examples

### HTTP Logging

```
# Text format
2024-01-15T10:30:45.123Z INFO [http] Request received method=POST path=/prompt
2024-01-15T10:30:45.168Z INFO [http] Response sent method=POST path=/prompt status=200 duration_ms=45

# JSON format
{"timestamp":"2024-01-15T10:30:45.123Z","level":"INFO","category":"http","message":"Request received","fields":{"method":"POST","path":"/prompt"}}
```

### SSE Logging

```
# Text format
2024-01-15T10:30:45.123Z INFO [sse] Client connected client_id=abc123 remote_addr=127.0.0.1:54321
2024-01-15T10:30:45.124Z DEBUG [sse] Event sent client_id=abc123 event_type=text bytes=256
2024-01-15T10:31:15.123Z INFO [sse] Heartbeat sent client_id=abc123
2024-01-15T10:35:00.000Z INFO [sse] Client disconnected client_id=abc123 duration_s=295

# JSON format
{"timestamp":"2024-01-15T10:30:45.123Z","level":"INFO","category":"sse","message":"Client connected","fields":{"client_id":"abc123","remote_addr":"127.0.0.1:54321"}}
```

### API Logging

```
# Text format
2024-01-15T10:30:45.123Z INFO [api] Request sent model=claude-3-haiku-20240307 messages=5 tools=3
2024-01-15T10:30:46.500Z INFO [api] Response received model=claude-3-haiku-20240307 input_tokens=1234 output_tokens=567 duration_ms=1377
2024-01-15T10:30:46.501Z DEBUG [api] Response content blocks=2 text_blocks=1 tool_use_blocks=1

# Error example
2024-01-15T10:30:46.500Z ERROR [api] Request failed model=claude-3-haiku-20240307 error="rate_limit_exceeded" retry_after_s=30

# JSON format
{"timestamp":"2024-01-15T10:30:45.123Z","level":"INFO","category":"api","message":"Request sent","fields":{"model":"claude-3-haiku-20240307","messages":5,"tools":3}}
```

### Tool Logging

```
# Text format
2024-01-15T10:30:46.510Z INFO [tool] Execution started tool=read id=toolu_123
2024-01-15T10:30:46.520Z DEBUG [tool] Tool input tool=read id=toolu_123 input={"path":"/config.json"}
2024-01-15T10:30:46.525Z INFO [tool] Execution completed tool=read id=toolu_123 duration_ms=15 success=true
2024-01-15T10:30:46.525Z DEBUG [tool] Tool output tool=read id=toolu_123 output_bytes=1024

# Error example
2024-01-15T10:30:46.525Z ERROR [tool] Execution failed tool=read id=toolu_123 error="file not found" duration_ms=5

# JSON format
{"timestamp":"2024-01-15T10:30:46.510Z","level":"INFO","category":"tool","message":"Execution started","fields":{"tool":"read","id":"toolu_123"}}
```

### Harness Logging

```
# Text format
2024-01-15T10:30:45.100Z INFO [harness] Agent loop started prompt_length=45
2024-01-15T10:30:46.600Z DEBUG [harness] Turn completed turn=1 tool_calls=1
2024-01-15T10:30:47.800Z INFO [harness] Agent loop completed turns=2 total_duration_ms=2700

# JSON format
{"timestamp":"2024-01-15T10:30:45.100Z","level":"INFO","category":"harness","message":"Agent loop started","fields":{"prompt_length":45}}
```

## Complete Example

A single user interaction showing both log systems:

**Server Logs (stderr):**
```
INFO  [http] Request received method=POST path=/prompt content_length=32
INFO  [harness] Agent loop started prompt_length=32
INFO  [api] Request sent model=claude-3-haiku messages=1 tools=3
INFO  [api] Response received input_tokens=150 output_tokens=45 duration_ms=823
INFO  [sse] Event sent client_id=1 event_type=text
INFO  [tool] Execution started tool=read id=toolu_123
INFO  [tool] Execution completed tool=read id=toolu_123 duration_ms=8 success=true
INFO  [sse] Event sent client_id=1 event_type=tool_result
INFO  [api] Request sent model=claude-3-haiku messages=3 tools=3
INFO  [api] Response received input_tokens=280 output_tokens=67 duration_ms=654
INFO  [sse] Event sent client_id=1 event_type=text
INFO  [harness] Agent loop completed turns=2 duration_ms=1520
INFO  [sse] Event sent client_id=1 event_type=status state=idle
```

**Agent Logs (file):**
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

=== 2024-01-15T10:30:47.000Z ASSISTANT ===
The config file contains:
- port: 8080
- host: localhost
```

**What each tells you:**
- Server logs: The request came in, API responded in 823ms, tool ran in 8ms, total 2 turns
- Agent logs: User asked about config.json, agent read it, found port and host settings

## Implementation Notes

### Logger Interface

```go
type Logger interface {
    Debug(category string, message string, fields ...Field)
    Info(category string, message string, fields ...Field)
    Warn(category string, message string, fields ...Field)
    Error(category string, message string, fields ...Field)
}

type Field struct {
    Key   string
    Value any
}
```

### Initialization

```go
func NewLogger(config LogConfig) Logger

type LogConfig struct {
    Level      string   // DEBUG, INFO, WARN, ERROR
    Format     string   // text, json
    Categories []string // Empty means all
    Output     io.Writer // Default: os.Stderr
}
```

### Usage Pattern

```go
logger.Info("http", "Request received",
    Field{"method", r.Method},
    Field{"path", r.URL.Path},
)

logger.Error("api", "Request failed",
    Field{"error", err.Error()},
    Field{"model", config.Model},
)
```

### Agent Interaction Logger

```go
type AgentLogger interface {
    LogUser(content string)
    LogAssistant(content string)
    LogToolCall(id, name string, input json.RawMessage)
    LogToolResult(id string, result string, isError bool)
}
```

### Thread Safety

- All logging operations must be thread-safe
- Use mutex or channel-based synchronization
- Buffer writes to reduce I/O overhead in high-throughput scenarios

### Performance Considerations

- Check log level before formatting expensive messages
- Use lazy evaluation for debug-level field values
- Avoid allocations in hot paths when log level would skip the message

```go
// Good: level check before expensive operation
if logger.IsDebugEnabled() {
    logger.Debug("api", "Response body", Field{"body", formatJSON(body)})
}

// Bad: always formats even when debug disabled
logger.Debug("api", "Response body", Field{"body", formatJSON(body)})
```

### Sensitive Data

Never log:
- API keys or tokens
- User credentials
- Full response bodies containing user data (use truncation or hashing)

Mask sensitive fields:
```
2024-01-15T10:30:45.123Z DEBUG [api] Request headers authorization=Bearer ***redacted***
```

## File Rotation (Agent Logs)

When `HARNESS_AGENT_LOG` is set:

| Setting | Value |
|---------|-------|
| Max file size | 10 MB |
| Max files | 5 |
| Rotation | Rename with timestamp suffix |

Example rotation:
```
agent.log           # Current
agent.log.2024-01-15T10-00-00  # Rotated
agent.log.2024-01-14T10-00-00  # Older
```

## Relationship to TUI

The TUI receives events via SSE and displays them. The two logging systems help debug different failure modes:

| Problem | Use | Why |
|---------|-----|-----|
| TUI shows nothing | Server logs (`sse`) | Check if SSE events are being sent |
| TUI shows partial response | Server logs (`api`) | Check if API is returning errors |
| Wrong content displayed | Agent logs | Compare what was sent vs what TUI shows |
| TUI not connecting | Server logs (`http`, `sse`) | Check if connection is established |

**Agent logs mirror TUI content** — they capture the same user/assistant/tool events that the TUI displays, but persisted to a file for:
- Debugging SSE delivery issues (compare agent log vs TUI display)
- Reviewing conversation history without running TUI
- Automated testing verification
- Post-mortem debugging of agent behavior
