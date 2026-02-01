# Core Harness Specification

## Overview

The harness manages an AI agent's interaction with the Anthropic API, handling the conversation loop and tool execution.

## Configuration

```go
type Config struct {
    APIKey       string  // Anthropic API key (required)
    Model        string  // Model ID (default: "claude-3-haiku-20240307")
    MaxTokens    int     // Max tokens per response (default: 4096)
    SystemPrompt string  // System prompt for agent behavior
    MaxTurns     int     // Max agent loop iterations (default: 10)
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `APIKey` | string | (required) | Anthropic API key |
| `Model` | string | `claude-3-haiku-20240307` | Model to use |
| `MaxTokens` | int | 4096 | Maximum tokens in each response |
| `SystemPrompt` | string | (empty) | Instructions for agent behavior |
| `MaxTurns` | int | 10 | Maximum iterations before forced termination |

### Tool Choice

The harness uses `auto` tool choice — the agent decides whether to use tools based on the user's request. This is not configurable.

| Mode | Behavior |
|------|----------|
| `auto` | Agent decides when to use tools |

### Available Models

| Model | ID | Use Case |
|-------|-----|----------|
| Haiku (default) | `claude-3-haiku-20240307` | Fast, cost-effective |
| Sonnet | `claude-3-5-sonnet-20241022` | Balanced |
| Opus | `claude-3-opus-20240229` | Most capable |

## Go API

### Interfaces

```go
// Tool defines a tool the agent can use.
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// EventHandler receives events from the harness.
type EventHandler interface {
    OnText(text string)
    OnToolCall(id string, name string, input json.RawMessage)
    OnToolResult(id string, result string, isError bool)
}
```

### Constructor

```go
func NewHarness(config Config, tools []Tool, handler EventHandler) *Harness
```

Creates a new harness instance. Call this once when the TUI starts.

| Parameter | Description |
|-----------|-------------|
| `config` | Configuration options |
| `tools` | Available tools for the agent |
| `handler` | Event handler for streaming events (can be nil) |

### Prompt

```go
func (h *Harness) Prompt(ctx context.Context, content string) error
```

Submits a user prompt and runs the agent loop until completion.

| Parameter | Description |
|-----------|-------------|
| `ctx` | Context for cancellation |
| `content` | The user's prompt text |

**Behavior:**
- Appends the user message to conversation history
- Emits a `user` event via the event handler
- Runs the agent loop (send → receive → handle tools → repeat)
- Returns when the agent responds with no tool calls, or max turns reached
- Returns an error if the API fails or context is cancelled

**Concurrency:**
- Only one `Prompt` call can run at a time
- Calling `Prompt` while another is running returns an error

### Cancel

```go
func (h *Harness) Cancel()
```

Cancels the currently running prompt. Safe to call if no prompt is running.

### Lifecycle

```
TUI starts
    │
    ▼
harness := NewHarness(config, tools, handler)
    │
    ▼
harness.Prompt(ctx, "user input")  ←─── called each time user submits
    │
    ▼
(agent loop runs, events emitted)
    │
    ▼
Prompt returns ───────────────────────► ready for next prompt
```

## Agent Loop

The agent operates in a request-response loop:

1. **Send Message** — Submit the current conversation (system prompt + message history) to the Anthropic API
2. **Receive Response** — Parse the assistant's response for text and tool use blocks
3. **Handle Tool Calls** — If tool use blocks are present:
   - Execute each tool with the provided inputs
   - Collect results (success or error)
   - Append tool results to the conversation
   - Return to step 1
4. **Terminate** — If no tool calls, the loop ends

```
┌─────────────────────────────────────────────────┐
│                   Start                         │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│         Send message to API                     │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│         Receive response                        │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│         Response contains tool calls?           │
└─────────────────┬───────────────────────────────┘
                  │
         ┌────────┴────────┐
         │ Yes             │ No
         ▼                 ▼
┌─────────────────┐  ┌─────────────────────────────┐
│ Execute tools   │  │         End                 │
│ Append results  │  └─────────────────────────────┘
└────────┬────────┘
         │
         └──────────────────┐
                            │
                  ▲─────────┘
                  │
         (loop back to Send)
```

## Conversation State

The harness maintains an ordered list of messages:

| Role | Content Types |
|------|---------------|
| `user` | Text, tool results |
| `assistant` | Text, tool use |

Messages are appended in order and never modified or removed during a session.

## Streaming and Event Handler

The harness uses streaming to receive responses from the API incrementally. An event handler interface allows consumers to observe the agent's activity in real-time.

### Event Handler Interface

```go
type EventHandler interface {
    // OnText is called when a text content block completes.
    // Called once per text block with the complete text.
    OnText(text string)

    // OnToolCall is called when a tool_use content block completes.
    // Called once per tool with the complete tool call information.
    OnToolCall(id string, name string, input json.RawMessage)

    // OnToolResult is called when tool execution completes.
    // Called once per tool with the result or error.
    OnToolResult(id string, result string, isError bool)
}
```

### Event Emission Timing

Events are emitted at specific points tied to the streaming protocol:

| Trigger | Event Emitted |
|---------|---------------|
| `ContentBlockStopEvent` for text block | `OnText(completeText)` |
| `ContentBlockStopEvent` for tool_use block | `OnToolCall(id, name, input)` |
| Tool execution completes | `OnToolResult(id, result, isError)` |

The harness accumulates streaming deltas internally and emits events only when blocks are complete. This ensures handlers receive complete, usable data.

### Event Sequence

For a typical agent turn with tool use:

```
[API: ContentBlockStopEvent type=text]
  → OnText("I'll read that file for you.")

[API: ContentBlockStopEvent type=tool_use]
  → OnToolCall("id1", "read", {"path": "config.json"})

[Harness: executes tool]
  → OnToolResult("id1", "{\"content\": \"...\"}", false)
```

### Handler Requirements

- The harness accepts an `EventHandler` at construction
- All events are delivered synchronously on the harness goroutine
- Handlers must not block; long operations should be dispatched asynchronously
- A nil or no-op handler is valid (silent operation)

## Tool Registration

Tools are declared to the API using the Anthropic tool schema format:

```json
{
  "name": "tool_name",
  "description": "What the tool does",
  "input_schema": {
    "type": "object",
    "properties": {
      "param_name": {
        "type": "string",
        "description": "Parameter description"
      }
    },
    "required": ["param_name"]
  }
}
```

All tools are registered in the initial API request and remain constant for the session.

## Termination Conditions

The agent loop terminates when any of the following occur:

1. **No Tool Calls** — The assistant responds with only text (no tool use blocks)
2. **Max Turns Reached** — A configurable maximum number of loop iterations is exceeded
3. **Unrecoverable Error** — An API error or system failure prevents continuation

## Tool Execution

When the assistant response contains multiple tool calls:

- Tools are executed sequentially in the order they appear in the response
- If a tool returns an error, execution stops immediately
- Only results for tools executed so far (including the error) are returned to the agent
- Remaining tools are not executed

This fail-fast behavior allows the agent to reassess its plan when something goes wrong, rather than continuing with potentially invalid assumptions.

## Error Propagation

Tool execution errors are **not** exceptions that halt the harness. Instead:

- All tool errors are serialized as tool result messages
- The error is sent back to the agent in the conversation
- The agent decides how to proceed (retry, try alternative, or give up)

This allows the agent to handle errors intelligently rather than failing immediately.

## HTTP Server

The harness exposes an HTTP server for the TUI to connect to.

### SSE Endpoint

```
GET /events
Content-Type: text/event-stream
```

Streams events to the TUI in real-time. Each event is a JSON object prefixed with `data: `:

```
data: {"type": "user", "content": "...", "timestamp": 1234567890}

data: {"type": "text", "content": "...", "timestamp": 1234567890}

data: {"type": "tool_call", "id": "...", "name": "...", "input": {...}, "timestamp": 1234567890}

data: {"type": "tool_result", "id": "...", "result": "...", "isError": false, "timestamp": 1234567890}

data: {"type": "reasoning", "content": "...", "timestamp": 1234567890}
```

**Heartbeat:** The server sends a comment line every 30 seconds to prevent connection timeout:

```
: heartbeat
```

### REST Endpoints

| Method | Path | Request Body | Description |
|--------|------|--------------|-------------|
| `POST` | `/prompt` | `{"content": "..."}` | Submit a user prompt |
| `POST` | `/cancel` | (empty) | Cancel the running agent |

### Event Types

| Type | Fields | Description |
|------|--------|-------------|
| `user` | `content`, `timestamp` | User submitted a prompt |
| `text` | `content`, `timestamp` | Agent text output |
| `tool_call` | `id`, `name`, `input`, `timestamp` | Agent invoked a tool |
| `tool_result` | `id`, `result`, `isError`, `timestamp` | Tool execution completed |
| `reasoning` | `content`, `timestamp` | Agent reasoning/thinking |
| `status` | `state`, `message` | Status update (thinking, running tool, idle) |
