# Harness Implementation Plan

This document outlines the implementation plan for the Harness system - an AI agent interaction framework with a terminal UI.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    TUI (TypeScript)                         │
│              Solid.js + OpenTUI + Zod                       │
└─────────────────────┬───────────────────────────────────────┘
                      │ HTTP (REST + SSE)
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                 Harness (Go)                                │
│           Anthropic SDK + HTTP Server                       │
└─────────────────────┬───────────────────────────────────────┘
                      │
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
   ┌─────────┐  ┌─────────┐  ┌──────────┐
   │  read   │  │  grep   │  │ list_dir │
   └─────────┘  └─────────┘  └──────────┘
```

---

## Implementation Priority

### Phase 1: Foundation (Go Backend)

#### 1.1 Project Setup & Tool Interface

**Why:** Establish Go module structure and define the foundational Tool interface that all tools implement. Tools are independent units with no external dependencies, making them ideal starting points.

**Tasks:**
- [ ] Initialize Go module (`go.mod`) with path `github.com/user/harness`
- [ ] Add dependency: `github.com/anthropics/anthropic-sdk-go`
- [ ] Define Tool interface in `pkg/tool/tool.go`:
  ```go
  type Tool interface {
      Name() string
      Description() string
      InputSchema() json.RawMessage
      Execute(ctx context.Context, input json.RawMessage) (string, error)
  }
  ```

**Acceptance Criteria:**
| Requirement | Verification |
|-------------|--------------|
| Go module initializes | `go mod tidy` succeeds without errors |
| Anthropic SDK resolves | `go build ./...` succeeds |
| Tool interface compiles | Interface is importable from `pkg/tool` |
| Interface is complete | All four methods defined per spec |

---

#### 1.2 READ Tool Implementation

**Why:** The READ tool is the simplest tool to implement and test, establishing patterns for JSON input/output and error handling.

**Specification Reference:** `specs/tools/read.md`

**Tasks:**
- [ ] Implement READ tool in `pkg/tool/read.go`
- [ ] Implement JSON schema for input validation
- [ ] Write comprehensive unit tests in `pkg/tool/read_test.go`

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Absolute or relative file path"},
    "start_line": {"type": "integer", "description": "First line to read (1-indexed)"},
    "end_line": {"type": "integer", "description": "Last line to read (inclusive)"}
  },
  "required": ["path"]
}
```

**Output Format:**
- Success: `{"content": "file contents as string"}`
- Error: `{"error": "error message"}`

**Test Matrix:**

| Test Case | Input | Expected Output |
|-----------|-------|-----------------|
| Read entire file | `{path: "test.txt"}` | `{"content": "line1\nline2\nline3"}` |
| Read with start_line only | `{path: "test.txt", start_line: 2}` | Lines 2 to end |
| Read with end_line only | `{path: "test.txt", end_line: 2}` | Lines 1 to 2 |
| Read specific range | `{path: "test.txt", start_line: 2, end_line: 3}` | Lines 2-3 inclusive |
| File not found | `{path: "nonexistent.txt"}` | `{"error": "file not found"}` |
| Path is directory | `{path: "/tmp"}` | `{"error": "path is a directory"}` |
| Permission denied | `{path: "/etc/shadow"}` | `{"error": "permission denied"}` |
| start_line < 1 | `{path: "test.txt", start_line: 0}` | `{"error": "..."}` (invalid) |
| start_line > end_line | `{start_line: 5, end_line: 2}` | `{"error": "..."}` |
| start_line > file length | `{start_line: 9999}` | `{"error": "..."}` |

**Acceptance Criteria:**
- Lines are 1-indexed (first line is line 1)
- `end_line` is inclusive
- All error conditions return `{"error": "..."}` format
- All success cases return `{"content": "..."}` format

---

#### 1.3 LIST_DIR Tool Implementation

**Why:** LIST_DIR establishes the pattern for executing system commands and capturing output.

**Specification Reference:** `specs/tools/list_dir.md`

**Tasks:**
- [ ] Implement LIST_DIR tool in `pkg/tool/list_dir.go`
- [ ] Execute `ls -al <path>` and capture raw output
- [ ] Write unit tests in `pkg/tool/list_dir_test.go`

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Directory path to list"}
  },
  "required": ["path"]
}
```

**Output Format:**
- Success: `{"entries": "raw ls -al output as string"}`
- Error: `{"error": "error message"}`

**Test Matrix:**

| Test Case | Input | Expected Output |
|-----------|-------|-----------------|
| Valid directory | `{path: "/tmp"}` | `{"entries": "total ...\ndrwx..."}` |
| Includes hidden files | `{path: "."}` | Output contains `.` prefixed files |
| Non-existent path | `{path: "/nonexistent"}` | `{"error": "..."}` |
| Path is file | `{path: "/etc/passwd"}` | `{"error": "not a directory"}` |
| Permission denied | `{path: "/root"}` | `{"error": "permission denied"}` |

**Acceptance Criteria:**
- Output includes hidden files (files starting with `.`)
- Output shows permissions, owner, group, size, date, name
- Returns raw `ls -al` output without modification

---

#### 1.4 GREP Tool Implementation

**Why:** GREP introduces optional parameters and regex pattern matching, completing the tool set.

**Specification Reference:** `specs/tools/grep.md`

**Tasks:**
- [ ] Implement GREP tool in `pkg/tool/grep.go`
- [ ] Use `/usr/bin/grep` with Basic Regular Expressions (BRE)
- [ ] Support recursive flag
- [ ] Write unit tests in `pkg/tool/grep_test.go`

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "pattern": {"type": "string", "description": "Search pattern (BRE regex)"},
    "path": {"type": "string", "description": "File or directory path"},
    "recursive": {"type": "boolean", "description": "Search recursively (default: false)"}
  },
  "required": ["pattern", "path"]
}
```

**Output Format:**
- Success: `{"matches": "grep output as string"}`
- Error: `{"error": "error message"}`

**Test Matrix:**

| Test Case | Input | Expected Output Format |
|-----------|-------|------------------------|
| Pattern found (single file) | `{pattern: "foo", path: "test.txt"}` | `{"matches": "line_number:content"}` |
| Pattern found (directory) | `{pattern: "foo", path: ".", recursive: true}` | `{"matches": "filename:line_number:content"}` |
| No matches | `{pattern: "xyz123", path: "test.txt"}` | `{"matches": ""}` (success, empty) |
| Recursive search | `{recursive: true}` | Searches subdirectories |
| Invalid regex | `{pattern: "[invalid"}` | `{"error": "..."}` |
| Path not found | `{path: "/nonexistent"}` | `{"error": "..."}` |

**Acceptance Criteria:**
- Uses macOS `/usr/bin/grep`
- Pattern is case-sensitive by default
- No matches returns success with empty string (not error)
- Single file format: `line_number:content`
- Multi-file format: `filename:line_number:content`

---

#### 1.5 Config & EventHandler

**Why:** Configuration and event handling are prerequisites for the core harness logic. These interfaces define the contract between components.

**Specification Reference:** `specs/harness.md` (Configuration, EventHandler sections)

**Tasks:**
- [ ] Define Config struct in `pkg/harness/config.go`
- [ ] Define EventHandler interface in `pkg/harness/events.go`
- [ ] Implement config validation and defaults

**Config Struct:**
```go
type Config struct {
    APIKey       string  // Required - Anthropic API key
    Model        string  // Default: "claude-3-haiku-20240307"
    MaxTokens    int     // Default: 4096
    SystemPrompt string  // Optional
    MaxTurns     int     // Default: 10
}
```

**EventHandler Interface:**
```go
type EventHandler interface {
    OnText(text string)
    OnToolCall(id string, name string, input json.RawMessage)
    OnToolResult(id string, result string, isError bool)
}
```

**Acceptance Criteria:**
| Requirement | Verification |
|-------------|--------------|
| APIKey required | Error if empty |
| Model defaults | `"claude-3-haiku-20240307"` when not specified |
| MaxTokens defaults | `4096` when not specified |
| MaxTurns defaults | `10` when not specified |
| SystemPrompt optional | Empty string is valid |
| Nil EventHandler valid | Harness operates silently without handler |

---

#### 1.6 Core Harness Implementation

**Why:** The harness is the central orchestrator connecting the API, tools, and event handler. It manages conversation state and concurrency.

**Specification Reference:** `specs/harness.md` (Go API, Conversation State sections)

**Tasks:**
- [ ] Implement Harness struct in `pkg/harness/harness.go`
- [ ] Implement constructor `NewHarness(config Config, tools []Tool, handler EventHandler) *Harness`
- [ ] Implement conversation state management (ordered message list)
- [ ] Implement concurrency control (mutex for single Prompt at a time)
- [ ] Implement `Prompt(ctx context.Context, content string) error`
- [ ] Implement `Cancel()` method

**Harness Struct Requirements:**
- Holds Anthropic client instance
- Maintains conversation history (never modified/removed during session)
- Tracks current running prompt context for cancellation
- Registers all tools in API requests

**Prompt Method Behavior:**
1. Return error if another Prompt is running
2. Append user message to conversation history
3. Emit `user` event (if handler != nil)
4. Run agent loop until termination
5. Return error if API fails or context cancelled

**Cancel Method Behavior:**
- Cancels context of running prompt
- Safe to call when no prompt is running (no-op)

**Acceptance Criteria:**
| Requirement | Verification |
|-------------|--------------|
| Concurrency control | Second Prompt() returns error while first runs |
| Tool registration | All tools appear in API requests with schemas |
| Conversation persistence | Messages preserved across turns within session |
| Cancellation works | Cancel() stops running prompt |
| Cancellation safe | Cancel() when idle does not panic |
| User event emitted | OnText not called, user event separate |

---

#### 1.7 Agent Loop & Streaming

**Why:** The agent loop is the core algorithm driving the harness. Streaming ensures real-time event emission.

**Specification Reference:** `specs/harness.md` (Agent Loop, Streaming, Tool Execution sections)

**Tasks:**
- [ ] Integrate streaming API using `client.Messages.NewStreaming()`
- [ ] Accumulate deltas with `message.Accumulate(event)`
- [ ] Emit events on `ContentBlockStopEvent` (not before)
- [ ] Implement sequential tool execution with fail-fast
- [ ] Implement termination conditions
- [ ] Write unit/integration tests

**Anthropic Go SDK Implementation Pattern:**
```go
func (h *Harness) runAgentLoop(ctx context.Context) error {
    for turn := 0; turn < h.config.MaxTurns; turn++ {
        // Create streaming request
        stream := h.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
            Model:     anthropic.Model(h.config.Model),
            MaxTokens: int64(h.config.MaxTokens),
            System:    []anthropic.TextBlockParam{{Text: h.config.SystemPrompt}},
            Messages:  h.messages,
            Tools:     h.toolParams,
        })

        // Accumulate streaming response
        message := anthropic.Message{}
        for stream.Next() {
            event := stream.Current()
            if err := message.Accumulate(event); err != nil {
                return err
            }

            // Emit events on ContentBlockStopEvent
            switch e := event.AsAny().(type) {
            case anthropic.ContentBlockStopEvent:
                h.emitBlockComplete(&message, e.Index)
            }
        }
        if stream.Err() != nil {
            return stream.Err()
        }

        // Append assistant message to history
        h.messages = append(h.messages, message.ToParam())

        // Process tool calls
        toolCalls := h.extractToolCalls(&message)
        if len(toolCalls) == 0 {
            return nil // No tool calls = done
        }

        // Execute tools sequentially with fail-fast
        toolResults, err := h.executeTools(ctx, toolCalls)
        if err != nil {
            return err
        }

        // Append tool results as user message
        h.messages = append(h.messages, anthropic.NewUserMessage(toolResults...))
    }
    return nil // MaxTurns reached
}

// Tool result creation
func (h *Harness) executeTools(ctx context.Context, calls []ToolCall) ([]anthropic.ContentBlockParamUnion, error) {
    var results []anthropic.ContentBlockParamUnion
    for _, call := range calls {
        result, err := h.executeTool(ctx, call)
        isError := err != nil
        resultStr := result
        if isError {
            resultStr = err.Error()
        }

        // Emit tool result event
        if h.handler != nil {
            h.handler.OnToolResult(call.ID, resultStr, isError)
        }

        // Create tool result block
        results = append(results, anthropic.NewToolResultBlock(call.ID, resultStr, isError))

        // Fail-fast: stop on first error
        if isError {
            break
        }
    }
    return results, nil
}
```

**Event Emission Timing (Critical):**
| API Event | Harness Action |
|-----------|----------------|
| `ContentBlockStopEvent` (text) | `OnText(completeText)` |
| `ContentBlockStopEvent` (tool_use) | `OnToolCall(id, name, input)` |
| Tool execution completes | `OnToolResult(id, result, isError)` |

**Detecting Block Types:**
```go
func (h *Harness) emitBlockComplete(msg *anthropic.Message, index int64) {
    block := msg.Content[index]
    switch b := block.AsAny().(type) {
    case anthropic.TextBlock:
        if h.handler != nil {
            h.handler.OnText(b.Text)
        }
    case anthropic.ToolUseBlock:
        if h.handler != nil {
            inputJSON, _ := json.Marshal(b.Input)
            h.handler.OnToolCall(b.ID, b.Name, inputJSON)
        }
    }
}
```

**Tool Execution Rules:**
1. Execute tools sequentially in response order
2. If tool returns error, stop immediately (fail-fast)
3. Return all results so far (including error) to agent
4. Remaining tools not executed
5. Tool errors serialized as tool result messages (not exceptions)

**Termination Conditions:**
1. No tool calls in response → end loop
2. MaxTurns exceeded → end loop
3. API error → return error
4. Context cancelled → return error

**Test Matrix:**

| Test Case | Expected Behavior |
|-----------|-------------------|
| Text-only response | Loop terminates after single turn |
| Single tool call | Tool executes, result sent, loop continues |
| Multiple tool calls | All execute sequentially |
| First tool error | Remaining tools skipped, error sent to agent |
| MaxTurns = 3, 4 turns needed | Loop terminates after turn 3 |
| Context cancelled mid-turn | Returns context.Canceled error |
| Event sequence | OnText → OnToolCall → OnToolResult order |

**Acceptance Criteria:**
| Requirement | Verification |
|-------------|--------------|
| Streaming accumulation | Events only on ContentBlockStopEvent |
| Sequential execution | Tools run in order, not parallel |
| Fail-fast | First error stops remaining tools |
| Error propagation | Errors go to agent, not thrown |
| MaxTurns enforced | Loop stops at limit |
| Event order | Text → ToolCall → ToolResult sequence |

---

#### 1.8 HTTP Server

**Why:** The HTTP server is the interface between harness and TUI, exposing SSE and REST endpoints.

**Specification Reference:** `specs/harness.md` (HTTP Server section)

**Tasks:**
- [ ] Implement HTTP server in `pkg/server/server.go`
- [ ] Implement SSE endpoint in `pkg/server/sse.go`
- [ ] Implement EventHandler that broadcasts to SSE clients
- [ ] Handle multiple concurrent SSE client connections
- [ ] Write server tests in `pkg/server/server_test.go`

**SSE Endpoint (`GET /events`):**
- Content-Type: `text/event-stream`
- Event format: `data: {JSON}\n\n`
- Heartbeat every 30 seconds: `: heartbeat\n`

**Event Types:**
```json
{"type": "user", "content": "...", "timestamp": 1234567890}
{"type": "text", "content": "...", "timestamp": 1234567890}
{"type": "tool_call", "id": "...", "name": "...", "input": {...}, "timestamp": 1234567890}
{"type": "tool_result", "id": "...", "result": "...", "isError": false, "timestamp": 1234567890}
{"type": "reasoning", "content": "...", "timestamp": 1234567890}
{"type": "status", "state": "...", "message": "..."}
```

**REST Endpoints:**
| Method | Path | Request Body | Response |
|--------|------|--------------|----------|
| POST | `/prompt` | `{"content": "..."}` | 200 OK or error |
| POST | `/cancel` | (empty) | 200 OK |

**Go SSE Server Implementation Pattern:**
```go
// SSE endpoint handler
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Register this client
    client := s.addClient()
    defer s.removeClient(client)

    // Heartbeat ticker
    heartbeat := time.NewTicker(30 * time.Second)
    defer heartbeat.Stop()

    for {
        select {
        case event := <-client.events:
            fmt.Fprintf(w, "data: %s\n\n", event)
            flusher.Flush()
        case <-heartbeat.C:
            fmt.Fprintf(w, ": heartbeat\n\n")
            flusher.Flush()
        case <-r.Context().Done():
            return
        }
    }
}

// Broadcast event to all SSE clients
func (s *Server) broadcast(event Event) {
    event.Timestamp = time.Now().Unix()
    data, _ := json.Marshal(event)

    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, client := range s.clients {
        select {
        case client.events <- data:
        default:
            // Client buffer full, skip
        }
    }
}

// EventHandler implementation for SSE broadcasting
type SSEEventHandler struct {
    server *Server
}

func (h *SSEEventHandler) OnText(text string) {
    h.server.broadcast(Event{Type: "text", Content: text})
}

func (h *SSEEventHandler) OnToolCall(id, name string, input json.RawMessage) {
    h.server.broadcast(Event{Type: "tool_call", ID: id, Name: name, Input: input})
}

func (h *SSEEventHandler) OnToolResult(id, result string, isError bool) {
    h.server.broadcast(Event{Type: "tool_result", ID: id, Result: result, IsError: isError})
}
```

**Acceptance Criteria:**
| Requirement | Verification |
|-------------|--------------|
| SSE connects | Client receives event stream |
| Heartbeat sent | Every 30 seconds, `: heartbeat\n` |
| Event format | All events have `type` and `timestamp` |
| Multiple clients | Broadcast to all connected SSE clients |
| POST /prompt | Triggers harness.Prompt() |
| POST /cancel | Triggers harness.Cancel() |
| Error handling | HTTP errors return appropriate status codes |

---

### Phase 2: Terminal UI (TypeScript)

#### 2.1 Project Setup

**Why:** Establish TypeScript project structure with proper tooling before building components.

**Specification Reference:** `specs/tui.md` (Packages section)

**Runtime:** Bun (for native TypeScript support and faster execution)

**Tasks:**
- [ ] Initialize project in `tui/` directory with `bun init`
- [ ] Configure TypeScript (`tsconfig.json`) with strict mode
- [ ] Add dependencies: `@opentui/core`, `@opentui/solid`, `solid-js`, `zod`
- [ ] Define Zod schemas for all event types in `tui/src/schemas/events.ts`
- [ ] Create entry point `tui/src/index.tsx`

**Project Initialization:**
```bash
cd tui
bun init
bun add @opentui/core @opentui/solid solid-js zod
```

**tsconfig.json:**
```json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "jsx": "preserve",
    "jsxImportSource": "solid-js",
    "noEmit": true,
    "skipLibCheck": true
  },
  "include": ["src/**/*"]
}
```

**Event Schemas (Zod with Discriminated Union):**
```typescript
import { z } from "zod"

// Individual event schemas
const UserEventSchema = z.object({
  type: z.literal("user"),
  content: z.string(),
  timestamp: z.number()
})

const TextEventSchema = z.object({
  type: z.literal("text"),
  content: z.string(),
  timestamp: z.number()
})

const ToolCallEventSchema = z.object({
  type: z.literal("tool_call"),
  id: z.string(),
  name: z.string(),
  input: z.record(z.unknown()),
  timestamp: z.number()
})

const ToolResultEventSchema = z.object({
  type: z.literal("tool_result"),
  id: z.string(),
  result: z.string(),
  isError: z.boolean(),
  timestamp: z.number()
})

const ReasoningEventSchema = z.object({
  type: z.literal("reasoning"),
  content: z.string(),
  timestamp: z.number()
})

const StatusEventSchema = z.object({
  type: z.literal("status"),
  state: z.enum(["idle", "thinking", "running_tool", "error"]),
  message: z.string().optional()
})

// Discriminated union for efficient parsing
export const EventSchema = z.discriminatedUnion("type", [
  UserEventSchema,
  TextEventSchema,
  ToolCallEventSchema,
  ToolResultEventSchema,
  ReasoningEventSchema,
  StatusEventSchema,
])

// Type inference
export type Event = z.infer<typeof EventSchema>
export type UserEvent = z.infer<typeof UserEventSchema>
export type TextEvent = z.infer<typeof TextEventSchema>
export type ToolCallEvent = z.infer<typeof ToolCallEventSchema>
export type ToolResultEvent = z.infer<typeof ToolResultEventSchema>
```

**Acceptance Criteria:**
- Project builds without errors (`bun run build`)
- TypeScript strict mode enabled
- All Zod schemas validate sample events correctly
- Type inference from schemas works
- `z.discriminatedUnion` used for efficient event parsing

---

#### 2.2 Communication Layer

**Why:** Reliable communication is essential before building UI components.

**Specification Reference:** `specs/tui.md` (Communication section)

**Tasks:**
- [ ] Implement SSE client in `tui/src/lib/sse.ts`
- [ ] Implement REST client in `tui/src/lib/api.ts`
- [ ] Add automatic reconnection on disconnect
- [ ] Validate events with Zod schemas

**SSE Client Implementation:**
```typescript
// tui/src/lib/sse.ts
import { EventSchema, type Event } from "../schemas/events"

type EventCallback = (event: Event) => void

export function createSSEClient(baseUrl: string, onEvent: EventCallback) {
  let eventSource: EventSource | null = null
  let reconnectTimeout: Timer | null = null

  function connect() {
    eventSource = new EventSource(`${baseUrl}/events`)

    eventSource.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        const result = EventSchema.safeParse(data)
        if (result.success) {
          onEvent(result.data)
        } else {
          console.error("Invalid event:", result.error)
        }
      } catch (err) {
        console.error("Failed to parse SSE event:", err)
      }
    }

    eventSource.onerror = () => {
      eventSource?.close()
      // Auto-reconnect after 1 second
      reconnectTimeout = setTimeout(connect, 1000)
    }
  }

  function disconnect() {
    if (reconnectTimeout) clearTimeout(reconnectTimeout)
    eventSource?.close()
  }

  connect()
  return { disconnect }
}
```

**REST Client Implementation:**
```typescript
// tui/src/lib/api.ts
const BASE_URL = "http://localhost:8080"

export async function submitPrompt(content: string): Promise<void> {
  const response = await fetch(`${BASE_URL}/prompt`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  })
  if (!response.ok) {
    throw new Error(`Failed to submit prompt: ${response.statusText}`)
  }
}

export async function cancelAgent(): Promise<void> {
  const response = await fetch(`${BASE_URL}/cancel`, {
    method: "POST",
  })
  if (!response.ok) {
    throw new Error(`Failed to cancel: ${response.statusText}`)
  }
}
```

**Acceptance Criteria:**
| Requirement | Verification |
|-------------|--------------|
| SSE connects | Receives events from `/events` |
| Event parsing | JSON parsed correctly |
| Schema validation | Invalid events logged/rejected gracefully |
| Reconnection | Auto-reconnects after 1 second on disconnect |
| submitPrompt works | POST /prompt succeeds |
| cancelAgent works | POST /cancel succeeds |

---

#### 2.3 State Management

**Why:** Centralized state ensures UI components react correctly to events.

**Specification Reference:** `specs/tui.md` (Parts section)

**Tasks:**
- [ ] Implement conversation store in `tui/src/stores/conversation.ts`
- [ ] Implement status store in `tui/src/stores/status.ts`
- [ ] Implement input store in `tui/src/stores/input.ts`
- [ ] Use Solid.js signals and stores for reactivity

**Solid.js State Patterns:**

```typescript
// tui/src/stores/conversation.ts
import { createStore, produce } from "solid-js/store"
import type { Event } from "../schemas/events"

export type Part =
  | { type: "user"; content: string; timestamp: number }
  | { type: "text"; content: string; timestamp: number }
  | { type: "tool"; id: string; name: string; input: Record<string, unknown>; result: string | null; isError: boolean; timestamp: number }
  | { type: "reasoning"; content: string; timestamp: number }

const [parts, setParts] = createStore<Part[]>([])

export function handleEvent(event: Event) {
  switch (event.type) {
    case "user":
      setParts(produce(p => p.push({ type: "user", content: event.content, timestamp: event.timestamp })))
      break
    case "text":
      setParts(produce(p => p.push({ type: "text", content: event.content, timestamp: event.timestamp })))
      break
    case "tool_call":
      setParts(produce(p => p.push({
        type: "tool",
        id: event.id,
        name: event.name,
        input: event.input,
        result: null,
        isError: false,
        timestamp: event.timestamp
      })))
      break
    case "tool_result":
      // Update existing tool part with result
      setParts(
        part => part.type === "tool" && part.id === event.id,
        produce(part => {
          part.result = event.result
          part.isError = event.isError
        })
      )
      break
    case "reasoning":
      setParts(produce(p => p.push({ type: "reasoning", content: event.content, timestamp: event.timestamp })))
      break
  }
}

export { parts }
```

```typescript
// tui/src/stores/status.ts
import { createSignal } from "solid-js"

type StatusState = "idle" | "thinking" | "running_tool" | "error"

const [status, setStatus] = createSignal<StatusState>("idle")
const [statusMessage, setStatusMessage] = createSignal<string>("")
const [currentTool, setCurrentTool] = createSignal<string>("")

export function handleStatusEvent(event: { state: string; message?: string }) {
  setStatus(event.state as StatusState)
  setStatusMessage(event.message ?? "")
}

export function setRunningTool(toolName: string) {
  setStatus("running_tool")
  setCurrentTool(toolName)
}

export { status, statusMessage, currentTool }
```

```typescript
// tui/src/stores/input.ts
import { createSignal } from "solid-js"

const MAX_HISTORY = 100

const [inputText, setInputText] = createSignal("")
const [history, setHistory] = createSignal<string[]>([])
const [historyIndex, setHistoryIndex] = createSignal(-1)

export function addToHistory(prompt: string) {
  setHistory(prev => {
    const newHistory = [...prev, prompt]
    if (newHistory.length > MAX_HISTORY) {
      newHistory.shift() // Remove oldest (FIFO)
    }
    return newHistory
  })
  setHistoryIndex(-1)
}

export function navigateHistoryUp() {
  const h = history()
  const idx = historyIndex()
  if (idx < h.length - 1) {
    const newIdx = idx + 1
    setHistoryIndex(newIdx)
    setInputText(h[h.length - 1 - newIdx])
  }
}

export function navigateHistoryDown() {
  const idx = historyIndex()
  if (idx > 0) {
    const newIdx = idx - 1
    setHistoryIndex(newIdx)
    setInputText(history()[history().length - 1 - newIdx])
  } else if (idx === 0) {
    setHistoryIndex(-1)
    setInputText("")
  }
}

export { inputText, setInputText, history }
```

**Acceptance Criteria:**
- Stores update reactively when events arrive
- Status reflects current agent state
- Input history navigates correctly (Up/Down)
- Parts array maintains order
- Tool parts updated in-place when result arrives

---

#### 2.4 Markdown Renderer

**Why:** Agent text output must be formatted correctly for terminal display.

**Specification Reference:** `specs/tui.md` (Markdown Rendering section)

**Tasks:**
- [ ] Implement markdown renderer in `tui/src/lib/markdown.ts`
- [ ] Support nested formatting
- [ ] Return terminal-compatible styled text

**Markdown Support Matrix:**

| Markdown | Terminal Rendering |
|----------|-------------------|
| `**bold**` | Terminal bold |
| `*italic*` | Terminal dim/italic |
| `` `inline code` `` | Highlighted background |
| ```` ``` ```` blocks | Yellow background (#3d3a28) |
| `- item` / `* item` | Bullet point (•) |
| `1. item` | Numbered list |
| `[text](url)` | Underlined text |
| `# Heading` | Bold text |
| Nested `**bold *italic***` | Both styles applied |

**Acceptance Criteria:**
| Input | Output |
|-------|--------|
| `**bold**` | Bold terminal text |
| `*italic*` | Dim/italic terminal text |
| `` `code` `` | Highlighted background |
| Code blocks | Yellow background |
| `- item` | `• item` |
| `1. item` | `1. item` (numbered) |
| `[link](url)` | Underlined text |
| `# Heading` | Bold text |
| Unknown markdown | Passed through as-is |

---

#### 2.5 UI Components

**Why:** Components are built bottom-up, starting with primitives.

**Specification Reference:** `specs/tui.md` (Parts, Layout sections)

**Tasks:**
- [ ] Implement part components in `tui/src/components/parts/`
  - [ ] `UserPart.tsx` - Cyan accent, full prompt text
  - [ ] `TextPart.tsx` - Markdown rendered
  - [ ] `ToolPart.tsx` - Tool name, input, result (truncated), errors
  - [ ] `ReasoningPart.tsx` - Dimmed/italicized
- [ ] Implement `Conversation.tsx` - Scrollable area with auto-scroll
- [ ] Implement `InputBar.tsx` - Text input with history
- [ ] Implement `Status.tsx` - Status indicator
- [ ] Implement `Help.tsx` - Centered modal overlay

**OpenTUI Component Patterns:**

```tsx
// tui/src/components/parts/UserPart.tsx
import type { Component } from "solid-js"

interface Props {
  content: string
}

export const UserPart: Component<Props> = (props) => (
  <box flexDirection="column" marginBottom={1}>
    <text content="You:" fg="#00FFFF" attributes={1} />
    <text content={props.content} fg="#00FFFF" />
  </box>
)
```

```tsx
// tui/src/components/parts/ToolPart.tsx
import type { Component } from "solid-js"
import { Show } from "solid-js"

interface Props {
  name: string
  input: Record<string, unknown>
  result: string | null
  isError: boolean
}

const MAX_LINES = 100

function truncateResult(result: string): { text: string; truncated: number } {
  const lines = result.split("\n")
  if (lines.length <= MAX_LINES) {
    return { text: result, truncated: 0 }
  }
  return {
    text: lines.slice(0, MAX_LINES).join("\n"),
    truncated: lines.length - MAX_LINES
  }
}

export const ToolPart: Component<Props> = (props) => {
  const truncated = () => props.result ? truncateResult(props.result) : null

  return (
    <box flexDirection="column" marginBottom={1} border={true} borderColor="#444444">
      <text content={`Tool: ${props.name}`} fg="#FFFF00" attributes={1} />
      <text content={`Input: ${JSON.stringify(props.input)}`} fg="#888888" />
      <Show when={props.result !== null}>
        <text
          content={truncated()?.text ?? ""}
          fg={props.isError ? "#FF0000" : "#e0e0e0"}
        />
        <Show when={truncated()?.truncated ?? 0 > 0}>
          <text content={`... (${truncated()?.truncated} more lines)`} fg="#888888" />
        </Show>
      </Show>
    </box>
  )
}
```

```tsx
// tui/src/components/Conversation.tsx
import { render } from "@opentui/solid"
import { For } from "solid-js"
import { parts } from "../stores/conversation"
import { UserPart } from "./parts/UserPart"
import { TextPart } from "./parts/TextPart"
import { ToolPart } from "./parts/ToolPart"
import { ReasoningPart } from "./parts/ReasoningPart"

export const Conversation = () => (
  <scrollbox
    width="100%"
    height="100%-3"
    stickyScroll={true}
    borderStyle="single"
    borderColor="#444444"
  >
    <For each={parts}>
      {(part) => {
        switch (part.type) {
          case "user":
            return <UserPart content={part.content} />
          case "text":
            return <TextPart content={part.content} />
          case "tool":
            return <ToolPart name={part.name} input={part.input} result={part.result} isError={part.isError} />
          case "reasoning":
            return <ReasoningPart content={part.content} />
        }
      }}
    </For>
  </scrollbox>
)
```

```tsx
// tui/src/components/InputBar.tsx
import { useKeyboard } from "@opentui/solid"
import { inputText, setInputText, navigateHistoryUp, navigateHistoryDown, addToHistory } from "../stores/input"
import { submitPrompt } from "../lib/api"

export const InputBar = () => {
  const handleSubmit = async (value: string) => {
    if (value.trim()) {
      addToHistory(value)
      setInputText("")
      await submitPrompt(value)
    }
  }

  useKeyboard((key) => {
    if (key.name === "up") {
      navigateHistoryUp()
    } else if (key.name === "down") {
      navigateHistoryDown()
    }
  })

  return (
    <box width="100%" height={3} position="absolute" bottom={0}>
      <input
        placeholder="Enter prompt..."
        value={inputText()}
        onInput={(value) => setInputText(value)}
        onSubmit={handleSubmit}
        focused={true}
        width="100%"
        border={true}
        borderColor="#666666"
      />
    </box>
  )
}
```

```tsx
// tui/src/components/Help.tsx
import { Show } from "solid-js"
import { useTerminalDimensions } from "@opentui/solid"

interface Props {
  visible: boolean
  onClose: () => void
}

export const Help = (props: Props) => {
  const dimensions = useTerminalDimensions()
  const width = 50
  const height = 15

  return (
    <Show when={props.visible}>
      <box
        position="absolute"
        left={Math.floor((dimensions().width - width) / 2)}
        top={Math.floor((dimensions().height - height) / 2)}
        width={width}
        height={height}
        border={true}
        borderColor="#FFFF00"
        backgroundColor="#1a1a1a"
        padding={1}
      >
        <text content="Help - Keybindings" fg="#FFFF00" attributes={1} />
        <text content="" />
        <text content="Enter           Submit prompt" fg="#e0e0e0" />
        <text content="Ctrl+Enter      Insert newline" fg="#e0e0e0" />
        <text content="Ctrl+C          Cancel / Exit" fg="#e0e0e0" />
        <text content="Up/Down         Prompt history" fg="#e0e0e0" />
        <text content="PageUp/Down     Scroll" fg="#e0e0e0" />
        <text content="Ctrl+U          Clear input" fg="#e0e0e0" />
        <text content="? or F1         Toggle help" fg="#e0e0e0" />
        <text content="Esc             Close" fg="#e0e0e0" />
        <text content="" />
        <text content="Press any key to close" fg="#888888" />
      </box>
    </Show>
  )
}
```

**ToolPart Display Rules:**
- Tool name prominently shown (yellow)
- Input parameters displayed in full (JSON)
- Result truncated to **100 lines maximum**
- Truncation indicator: `... (X more lines)`
- Errors in red

**Scrolling Behavior:**
- `stickyScroll={true}` enables auto-scroll on new content
- Automatically pauses when user scrolls up
- Resumes when scrolled to bottom

**Acceptance Criteria:**
| Component | Requirement |
|-----------|-------------|
| UserPart | Cyan styling, full text visible |
| TextPart | Markdown renders correctly |
| ToolPart | Name yellow, result ≤100 lines, errors red |
| ReasoningPart | Dimmed/italic styling |
| Conversation | Auto-scroll with manual override via stickyScroll |
| InputBar | Text entry, history, multiline |
| Status | Shows correct state |
| Help | Modal displays and dismisses on any key |

---

#### 2.6 Layout & Theme

**Why:** Final assembly of components into the complete application.

**Specification Reference:** `specs/tui.md` (Layout, Theme sections)

**Tasks:**
- [ ] Implement main layout in `tui/src/App.tsx`
- [ ] Define theme in `tui/src/theme.ts`
- [ ] Handle terminal resize
- [ ] Wire up SSE client to state stores

**Main Application Entry Point:**
```tsx
// tui/src/index.tsx
import { render } from "@opentui/solid"
import { App } from "./App"

render(App, {
  targetFps: 30,
  exitOnCtrlC: false, // We handle Ctrl+C manually
})
```

```tsx
// tui/src/App.tsx
import { createSignal, onMount, onCleanup } from "solid-js"
import { useKeyboard, useRenderer } from "@opentui/solid"
import { Conversation } from "./components/Conversation"
import { InputBar } from "./components/InputBar"
import { Status } from "./components/Status"
import { Help } from "./components/Help"
import { createSSEClient } from "./lib/sse"
import { handleEvent } from "./stores/conversation"
import { cancelAgent } from "./lib/api"
import { status } from "./stores/status"
import { loadHistory } from "./lib/history"

export const App = () => {
  const renderer = useRenderer()
  const [showHelp, setShowHelp] = createSignal(false)

  // Connect to SSE on mount
  onMount(() => {
    loadHistory() // Load persisted history

    const sse = createSSEClient("http://localhost:8080", (event) => {
      handleEvent(event)
    })

    onCleanup(() => sse.disconnect())
  })

  // Global keybindings
  useKeyboard((key) => {
    // Help toggle
    if (key.name === "?" || key.name === "f1") {
      setShowHelp(v => !v)
      return
    }

    // Close help on any key when open
    if (showHelp()) {
      setShowHelp(false)
      return
    }

    // Ctrl+C: Cancel or exit
    if (key.ctrl && key.name === "c") {
      if (status() !== "idle") {
        cancelAgent()
      } else {
        renderer.destroy()
      }
      return
    }

    // Esc: Close help
    if (key.name === "escape") {
      setShowHelp(false)
    }
  })

  return (
    <box flexDirection="column" width="100%" height="100%" backgroundColor="#1a1a1a">
      <Conversation />
      <Status />
      <InputBar />
      <Help visible={showHelp()} onClose={() => setShowHelp(false)} />
    </box>
  )
}
```

**Theme Constants:**
```typescript
// tui/src/theme.ts
export const theme = {
  colors: {
    background: "#1a1a1a",
    text: "#e0e0e0",
    textDim: "#888888",
    userPrompt: "#00FFFF",    // Cyan
    toolName: "#FFFF00",       // Yellow
    error: "#FF0000",          // Red
    reasoning: "#666666",      // Dimmed gray
    border: "#444444",
    status: "#FFFF00",
    codeBlockBg: "#3d3a28",
  },
  attributes: {
    bold: 1,
    dim: 2,
    italic: 4,
    underline: 8,
  }
} as const
```

**Layout Regions:**
| Region | Position | Content |
|--------|----------|---------|
| Conversation | Top (scrollable, flex: 1) | All parts |
| Status | Above input | Status indicator |
| Input | Bottom (height: 3) | Text input bar |

**Theme Colors:**
| Element | Color |
|---------|-------|
| Background | #1a1a1a (dark gray) |
| Text | #e0e0e0 (light gray) |
| User prompt | #00FFFF (cyan) |
| Agent text | #e0e0e0 (light gray) |
| Tool name | #FFFF00 (yellow) |
| Tool result | #e0e0e0 (light gray) |
| Error | #FF0000 (red) |
| Reasoning | #666666 (dimmed gray) |
| Border | #444444 (subtle gray) |
| Status | #FFFF00 (yellow) |
| Code block bg | #3d3a28 |

**Acceptance Criteria:**
- Layout renders correctly at various terminal sizes
- Colors match specification
- Resize handled gracefully via `useTerminalDimensions()`
- SSE client connects on mount and disconnects on cleanup

---

#### 2.7 Keybindings

**Why:** Keyboard interaction is critical for terminal UIs.

**Specification Reference:** `specs/tui.md` (Keybindings, Input Behavior sections)

**Tasks:**
- [ ] Implement all keybindings
- [ ] Ensure no conflicts between bindings

**Keybinding Matrix:**

| Key | Action |
|-----|--------|
| `Enter` | Submit prompt |
| `Ctrl+Enter` | Insert newline in prompt |
| `Ctrl+C` | Cancel running agent / Exit if idle |
| `Up` (in input) | Previous prompt from history |
| `Down` (in input) | Next prompt from history |
| `PageUp` | Scroll conversation up |
| `PageDown` | Scroll conversation down |
| `Ctrl+U` | Clear input |
| `Ctrl+Shift+C` | Copy selected text |
| `?` or `F1` | Toggle help screen |
| `Esc` | Close help / Cancel selection |

**Acceptance Criteria:**
- All keybindings function as specified
- No key conflicts
- Help shows all bindings
- History navigation works at input boundaries

---

#### 2.8 Persistence

**Why:** User experience improvement through session persistence.

**Specification Reference:** `specs/tui.md` (Prompt History section)

**Tasks:**
- [ ] Implement history persistence in `tui/src/lib/history.ts`
- [ ] Save to `~/.harness/prompt_history`
- [ ] Load on startup
- [ ] Enforce max 100 prompts (FIFO)

**Acceptance Criteria:**
| Requirement | Verification |
|-------------|--------------|
| Persistence | History survives TUI restart |
| File location | `~/.harness/prompt_history` |
| Max entries | 100 prompts maximum |
| FIFO behavior | Oldest removed when limit exceeded |
| Load on startup | Previous prompts available via Up arrow |

---

## File Structure

```
harness/
├── cmd/
│   └── harness/
│       └── main.go           # Entry point
├── pkg/
│   ├── harness/
│   │   ├── config.go         # Config struct
│   │   ├── events.go         # EventHandler interface
│   │   ├── harness.go        # Core harness
│   │   └── harness_test.go
│   ├── server/
│   │   ├── server.go         # HTTP server
│   │   ├── sse.go            # SSE endpoint
│   │   └── server_test.go
│   └── tool/
│       ├── tool.go           # Tool interface
│       ├── read.go
│       ├── read_test.go
│       ├── grep.go
│       ├── grep_test.go
│       ├── list_dir.go
│       └── list_dir_test.go
├── tui/
│   ├── src/
│   │   ├── components/
│   │   │   ├── parts/
│   │   │   │   ├── UserPart.tsx
│   │   │   │   ├── TextPart.tsx
│   │   │   │   ├── ToolPart.tsx
│   │   │   │   └── ReasoningPart.tsx
│   │   │   ├── Conversation.tsx
│   │   │   ├── InputBar.tsx
│   │   │   ├── Status.tsx
│   │   │   └── Help.tsx
│   │   ├── lib/
│   │   │   ├── api.ts
│   │   │   ├── sse.ts
│   │   │   ├── markdown.ts
│   │   │   └── history.ts
│   │   ├── schemas/
│   │   │   └── events.ts
│   │   ├── stores/
│   │   │   ├── conversation.ts
│   │   │   ├── status.ts
│   │   │   └── input.ts
│   │   ├── theme.ts
│   │   └── App.tsx
│   ├── package.json
│   └── tsconfig.json
├── specs/                    # Specifications (reference)
├── go.mod
├── go.sum
└── IMPLEMENTATION_PLAN.md
```

---

## Dependencies

### Go

```go
// go.mod
module github.com/user/harness

go 1.21

require (
    github.com/anthropics/anthropic-sdk-go v0.2.0
)
```

### TypeScript (Bun)

**Runtime:** Bun v1.0+ (for native TypeScript support)

```json
// tui/package.json
{
  "name": "harness-tui",
  "type": "module",
  "scripts": {
    "start": "bun run src/index.tsx",
    "build": "bun build src/index.tsx --outdir=dist"
  },
  "dependencies": {
    "@opentui/core": "^0.1.75",
    "@opentui/solid": "^0.1.75",
    "solid-js": "^1.9.0",
    "zod": "^3.22.0"
  },
  "devDependencies": {
    "@types/bun": "latest",
    "typescript": "^5.0.0"
  }
}
```

---

## Testing Strategy

### Unit Tests

| Component | Test Focus |
|-----------|------------|
| READ tool | File operations, line ranges, error conditions |
| LIST_DIR tool | Directory listing, hidden files, errors |
| GREP tool | Pattern matching, recursive, empty results |
| Config | Defaults, validation, required fields |
| Harness | Concurrency, cancellation, event emission |
| Markdown | All formatting combinations |
| Zod schemas | Valid and invalid event parsing |

### Integration Tests

| Test | Components |
|------|------------|
| Agent loop | Harness + Tools + Mock API |
| HTTP flow | Server + SSE + REST |
| Full stack | TUI + Server + Harness |
| Persistence | History write/read cycle |

### Manual Testing

- Full conversation flow with real Anthropic API
- All keybindings in terminal
- Terminal resize handling
- Long outputs and truncation behavior

---

## Key Implementation Notes

1. **Tool choice is always `auto`** — Not configurable per spec
2. **Streaming is required** — Events emitted on block completion, not message completion
3. **Error handling philosophy** — Tool errors go to agent for intelligent handling, not as exceptions
4. **Sequential tool execution** — No parallel execution; fail-fast on error
5. **History limit** — 100 prompts max, oldest removed first (FIFO)
6. **Event emission timing** — Use `ContentBlockStopEvent` from streaming API, accumulate with `message.Accumulate(event)`
7. **Heartbeat interval** — 30 seconds for SSE to prevent timeout
8. **Line indexing** — 1-indexed for READ tool (first line is line 1)
9. **Result truncation** — Tool results truncated at 100 lines in TUI
10. **Bun runtime** — TUI uses Bun for native TypeScript execution
11. **Zod discriminated unions** — Use `z.discriminatedUnion("type", [...])` for efficient event parsing

---

## Tool Registration Pattern (Go)

Tools must be converted to Anthropic API format when registering with the harness:

```go
// Convert Tool interface to Anthropic ToolParam
func toolToParam(t tool.Tool) anthropic.ToolUnionParam {
    return anthropic.ToolUnionParam{
        OfTool: &anthropic.ToolParam{
            Name:        t.Name(),
            Description: anthropic.String(t.Description()),
            InputSchema: anthropic.ToolInputSchemaParam{
                Properties: t.InputSchema(), // json.RawMessage
            },
        },
    }
}

// In Harness constructor
func NewHarness(config Config, tools []tool.Tool, handler EventHandler) *Harness {
    // Convert tools to API format
    toolParams := make([]anthropic.ToolUnionParam, len(tools))
    for i, t := range tools {
        toolParams[i] = toolToParam(t)
    }

    // Create tool lookup map for execution
    toolMap := make(map[string]tool.Tool)
    for _, t := range tools {
        toolMap[t.Name()] = t
    }

    return &Harness{
        client:     anthropic.NewClient(),
        config:     config,
        tools:      toolMap,
        toolParams: toolParams,
        handler:    handler,
        messages:   []anthropic.MessageParam{},
    }
}
```
