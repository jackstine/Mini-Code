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

**Event Emission Timing (Critical):**
| API Event | Harness Action |
|-----------|----------------|
| `ContentBlockStopEvent` (text) | `OnText(completeText)` |
| `ContentBlockStopEvent` (tool_use) | `OnToolCall(id, name, input)` |
| Tool execution completes | `OnToolResult(id, result, isError)` |

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

**Tasks:**
- [ ] Initialize project in `tui/` directory
- [ ] Configure TypeScript (`tsconfig.json`)
- [ ] Add dependencies: `@opentui/core`, `@opentui/solid`, `solid-js`, `zod`
- [ ] Define Zod schemas for all event types in `tui/src/schemas/events.ts`

**Event Schemas (Zod):**
```typescript
// User event
const UserEventSchema = z.object({
  type: z.literal("user"),
  content: z.string(),
  timestamp: z.number()
})

// Text event
const TextEventSchema = z.object({
  type: z.literal("text"),
  content: z.string(),
  timestamp: z.number()
})

// Tool call event
const ToolCallEventSchema = z.object({
  type: z.literal("tool_call"),
  id: z.string(),
  name: z.string(),
  input: z.record(z.unknown()),
  timestamp: z.number()
})

// Tool result event
const ToolResultEventSchema = z.object({
  type: z.literal("tool_result"),
  id: z.string(),
  result: z.string(),
  isError: z.boolean(),
  timestamp: z.number()
})

// Reasoning event
const ReasoningEventSchema = z.object({
  type: z.literal("reasoning"),
  content: z.string(),
  timestamp: z.number()
})

// Status event
const StatusEventSchema = z.object({
  type: z.literal("status"),
  state: z.string(),
  message: z.string().optional()
})
```

**Acceptance Criteria:**
- Project builds without errors (`npm run build`)
- TypeScript strict mode enabled
- All Zod schemas validate sample events correctly
- Type inference from schemas works

---

#### 2.2 Communication Layer

**Why:** Reliable communication is essential before building UI components.

**Specification Reference:** `specs/tui.md` (Communication section)

**Tasks:**
- [ ] Implement SSE client in `tui/src/lib/sse.ts`
- [ ] Implement REST client in `tui/src/lib/api.ts`
- [ ] Add automatic reconnection on disconnect
- [ ] Validate events with Zod schemas

**SSE Client Requirements:**
- Connect to `GET /events`
- Parse `data: {...}` events
- Validate against Zod schemas
- Emit typed events to handlers
- Auto-reconnect on disconnect

**REST Client Requirements:**
```typescript
async function submitPrompt(content: string): Promise<void>
async function cancelAgent(): Promise<void>
```

**Acceptance Criteria:**
| Requirement | Verification |
|-------------|--------------|
| SSE connects | Receives events from `/events` |
| Event parsing | JSON parsed correctly |
| Schema validation | Invalid events logged/rejected gracefully |
| Reconnection | Auto-reconnects after disconnect |
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
- [ ] Use Solid.js signals for reactivity

**Part Types:**
```typescript
type UserPart = { type: "user"; content: string; timestamp: number }
type TextPart = { type: "text"; content: string; timestamp: number }
type ToolPart = {
  type: "tool"; id: string; name: string;
  input: Record<string, unknown>; result: string | null;
  isError: boolean; timestamp: number
}
type ReasoningPart = { type: "reasoning"; content: string; timestamp: number }
```

**Status States:**
- `idle` - No indicator shown
- `thinking` - "Thinking..."
- `running_tool` - "Running: {tool_name}..."
- `error` - "Error: {message}"

**Input Store:**
- Current input text
- History array (max 100 entries)
- History navigation index

**Acceptance Criteria:**
- Stores update reactively when events arrive
- Status reflects current agent state
- Input history navigates correctly (Up/Down)
- Parts array maintains order

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

**ToolPart Display Rules:**
- Tool name prominently shown (yellow)
- Input parameters displayed in full (JSON)
- Result truncated to **100 lines maximum**
- Truncation indicator: `... (X more lines)`
- Errors in red

**Scrolling Behavior:**
- Auto-scroll on new content
- Pause auto-scroll when user scrolls up
- Resume when scrolled to bottom
- Optional: "↓ New content" indicator

**Acceptance Criteria:**
| Component | Requirement |
|-----------|-------------|
| UserPart | Cyan styling, full text visible |
| TextPart | Markdown renders correctly |
| ToolPart | Name yellow, result ≤100 lines, errors red |
| ReasoningPart | Dimmed/italic styling |
| Conversation | Auto-scroll with manual override |
| InputBar | Text entry, history, multiline |
| Status | Shows correct state |
| Help | Modal displays and dismisses |

---

#### 2.6 Layout & Theme

**Why:** Final assembly of components into the complete application.

**Specification Reference:** `specs/tui.md` (Layout, Theme sections)

**Tasks:**
- [ ] Implement main layout in `tui/src/App.tsx`
- [ ] Define theme in `tui/src/theme.ts`
- [ ] Handle terminal resize

**Layout Regions:**
| Region | Position | Content |
|--------|----------|---------|
| Conversation | Top (scrollable) | All parts |
| Input | Bottom | Text input bar |
| Status | Above/within input | Status indicator |

**Theme Colors:**
| Element | Color |
|---------|-------|
| Background | #1a1a1a (dark gray) |
| Text | #e0e0e0 (light gray) |
| User prompt | Cyan accent |
| Agent text | Light gray (default) |
| Tool name | Yellow |
| Tool result | Light gray |
| Error | Red |
| Reasoning | Dimmed gray |
| Input border | Subtle gray |
| Status | Yellow |
| Code block bg | #3d3a28 |

**Acceptance Criteria:**
- Layout renders correctly at various terminal sizes
- Colors match specification
- Resize handled gracefully

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
require (
    github.com/anthropics/anthropic-sdk-go
)
```

### TypeScript

```json
{
  "dependencies": {
    "@opentui/core": "latest",
    "@opentui/solid": "latest",
    "solid-js": "^1.8",
    "zod": "^3.22"
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
