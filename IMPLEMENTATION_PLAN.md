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

- [ ] **Initialize Go module** (`go.mod`)
  - Module path: `github.com/user/harness` (or appropriate path)
  - Add dependency: `github.com/anthropics/anthropic-sdk-go`

- [ ] **Define Tool interface** (`pkg/tool/tool.go`)
  - `Name() string` - returns tool identifier
  - `Description() string` - returns tool description for API
  - `InputSchema() json.RawMessage` - returns JSON schema for parameters
  - `Execute(ctx context.Context, input json.RawMessage) (string, error)` - executes tool

**Acceptance Criteria:**
- Go module initializes and dependencies resolve
- Tool interface compiles and is importable

---

#### 1.2 READ Tool Implementation

**Why:** The READ tool is the simplest tool to implement and test, establishing patterns for error handling and JSON output.

- [ ] **Implement READ tool** (`pkg/tool/read.go`)
  - Accept `path` (required), `start_line` (optional), `end_line` (optional)
  - Lines are 1-indexed, `end_line` is inclusive
  - Return `{"content": "..."}` on success
  - Return `{"error": "..."}` on failure

- [ ] **Unit tests** (`pkg/tool/read_test.go`)
  - Test: read entire file
  - Test: read with start_line only
  - Test: read with end_line only
  - Test: read with both start_line and end_line
  - Test: error - file not found
  - Test: error - path is directory
  - Test: error - permission denied (if testable)
  - Test: error - start_line < 1
  - Test: error - start_line > end_line
  - Test: error - start_line exceeds file lines

**Acceptance Criteria:**
| Scenario | Expected Result |
|----------|-----------------|
| Read entire file | Returns full content in `{"content": "..."}` |
| Read lines 5-10 | Returns lines 5 through 10 inclusive |
| Read from line 3 to end | Returns line 3 to last line |
| Read from start to line 7 | Returns line 1 through 7 |
| File not found | Returns `{"error": "file not found"}` or similar |
| Path is directory | Returns `{"error": "path is a directory"}` |
| start_line = 0 | Returns error (1-indexed) |
| start_line > end_line | Returns error |
| start_line > file length | Returns error |

---

#### 1.3 LIST_DIR Tool Implementation

**Why:** LIST_DIR is a straightforward wrapper around `ls -al`, establishing the pattern for command execution tools.

- [ ] **Implement LIST_DIR tool** (`pkg/tool/list_dir.go`)
  - Accept `path` (required)
  - Execute `ls -al <path>`
  - Return `{"entries": "..."}` on success
  - Return `{"error": "..."}` on failure

- [ ] **Unit tests** (`pkg/tool/list_dir_test.go`)
  - Test: list existing directory
  - Test: includes hidden files (files starting with `.`)
  - Test: error - path not found
  - Test: error - path is a file (not directory)
  - Test: error - permission denied (if testable)

**Acceptance Criteria:**
| Scenario | Expected Result |
|----------|-----------------|
| Valid directory | Returns `ls -al` output with permissions, owner, size, date, name |
| Directory with hidden files | Output includes `.` prefixed files |
| Non-existent path | Returns `{"error": "..."}` |
| Path is file | Returns `{"error": "not a directory"}` |

---

#### 1.4 GREP Tool Implementation

**Why:** GREP introduces pattern matching and optional parameters (recursive flag), completing the tool set.

- [ ] **Implement GREP tool** (`pkg/tool/grep.go`)
  - Accept `pattern` (required), `path` (required), `recursive` (optional, default: false)
  - Use `/usr/bin/grep` with Basic Regular Expressions (BRE)
  - Return `{"matches": "..."}` on success (including empty matches)
  - Return `{"error": "..."}` on failure

- [ ] **Unit tests** (`pkg/tool/grep_test.go`)
  - Test: pattern found in single file
  - Test: pattern with recursive search
  - Test: no matches (success with empty string)
  - Test: error - path not found
  - Test: error - invalid regex pattern
  - Test: error - permission denied (if testable)

**Acceptance Criteria:**
| Scenario | Expected Result |
|----------|-----------------|
| Pattern found | Returns `filename:line_number:content` format |
| Single file match | Returns `line_number:content` format (no filename) |
| Recursive search | Searches subdirectories when `recursive=true` |
| No matches | Returns `{"matches": ""}` (success, not error) |
| Invalid regex | Returns `{"error": "..."}` |
| Path not found | Returns `{"error": "..."}` |

---

#### 1.5 Config & EventHandler

**Why:** Configuration and event handling are prerequisites for the core harness logic.

- [ ] **Define Config struct** (`pkg/harness/config.go`)
  - `APIKey` (required) - Anthropic API key
  - `Model` (default: `claude-3-haiku-20240307`)
  - `MaxTokens` (default: 4096)
  - `SystemPrompt` (optional)
  - `MaxTurns` (default: 10)

- [ ] **Define EventHandler interface** (`pkg/harness/events.go`)
  - `OnText(text string)` - called when text block completes
  - `OnToolCall(id, name string, input json.RawMessage)` - called when tool_use block completes
  - `OnToolResult(id, result string, isError bool)` - called when tool execution completes

**Acceptance Criteria:**
- Config struct has all required fields with correct defaults
- EventHandler interface is implementable
- Nil EventHandler is accepted (silent operation)

---

#### 1.6 Core Harness Implementation

**Why:** The harness is the central orchestrator connecting the API, tools, and event handler.

- [ ] **Implement Harness struct** (`pkg/harness/harness.go`)
  - Constructor: `NewHarness(config Config, tools []Tool, handler EventHandler) *Harness`
  - Conversation state: ordered list of messages
  - Concurrency control: mutex to ensure single Prompt at a time

- [ ] **Implement Prompt method**
  - `func (h *Harness) Prompt(ctx context.Context, content string) error`
  - Append user message to history
  - Emit `user` event (if handler != nil)
  - Run agent loop until termination
  - Return error if API fails or context cancelled
  - Return error if another Prompt is already running

- [ ] **Implement Cancel method**
  - `func (h *Harness) Cancel()`
  - Cancel context of running prompt
  - Safe to call when no prompt running

**Acceptance Criteria:**
| Requirement | Test |
|-------------|------|
| Config defaults | Harness initializes with correct defaults when optional fields omitted |
| Tool registration | All tools appear in API requests with correct schemas |
| Concurrency | Second Prompt() call returns error while first is running |
| Cancellation | Cancel() stops running prompt; safe when idle |

---

#### 1.7 Agent Loop & Streaming

**Why:** The agent loop is the core algorithm that drives the harness. It must correctly implement streaming with proper event emission timing.

- [ ] **Streaming API integration**
  - Use `client.Messages.NewStreaming()` from anthropic-sdk-go
  - Accumulate deltas with `message.Accumulate(event)`
  - Process `ContentBlockStopEvent` for text blocks → emit `OnText`
  - Process `ContentBlockStopEvent` for tool_use blocks → emit `OnToolCall`

- [ ] **Tool execution logic**
  - Execute tools sequentially in response order
  - Fail-fast: stop on first tool error
  - Serialize all results (including errors) as tool result messages
  - Tool errors go back to agent (not thrown as exceptions)
  - Emit `OnToolResult` after each tool execution

- [ ] **Termination conditions**
  - No tool calls in response → end loop
  - MaxTurns exceeded → end loop
  - API error → return error
  - Context cancelled → return error

- [ ] **Unit/Integration tests** (`pkg/harness/harness_test.go`)
  - Test: text-only response terminates loop
  - Test: tool call triggers execution and loop continuation
  - Test: multiple tool calls execute sequentially
  - Test: first tool error stops remaining tools
  - Test: MaxTurns limit enforced
  - Test: cancellation works mid-loop
  - Test: events emitted in correct order

**Acceptance Criteria:**
| Requirement | Test |
|-------------|------|
| Streaming | Events emitted on ContentBlockStopEvent, not before |
| Tool execution | Tools execute sequentially; first error stops execution |
| Error propagation | Tool errors serialized to agent, not thrown as exceptions |
| MaxTurns | Agent loop terminates after MaxTurns iterations |
| Event order | OnText → OnToolCall → OnToolResult sequence maintained |

---

#### 1.8 HTTP Server

**Why:** The HTTP server is the interface between harness and TUI, exposing SSE and REST endpoints.

- [ ] **SSE endpoint** (`pkg/server/sse.go`)
  - `GET /events`
  - Content-Type: `text/event-stream`
  - Event format: `data: {"type": "...", ...}\n\n`
  - Heartbeat every 30 seconds: `: heartbeat\n`
  - Event types: `user`, `text`, `tool_call`, `tool_result`, `reasoning`, `status`

- [ ] **REST endpoints** (`pkg/server/server.go`)
  - `POST /prompt` - Body: `{"content": "..."}`
  - `POST /cancel` - Empty body

- [ ] **Event bridging**
  - Implement EventHandler that broadcasts to SSE clients
  - Include timestamps in all events
  - Handle multiple SSE client connections

- [ ] **Server tests** (`pkg/server/server_test.go`)
  - Test: SSE connection receives events
  - Test: heartbeat sent every 30 seconds
  - Test: POST /prompt triggers harness
  - Test: POST /cancel stops running prompt

**Acceptance Criteria:**
| Requirement | Test |
|-------------|------|
| SSE endpoint | Connects and streams events correctly |
| Heartbeat | Sent every 30 seconds to prevent timeout |
| Event format | All events include type and timestamp |
| REST /prompt | Accepts prompt and returns success |
| REST /cancel | Cancels running agent |

---

### Phase 2: Terminal UI (TypeScript)

#### 2.1 Project Setup

**Why:** Establish TypeScript project structure with proper tooling before building components.

- [ ] **Initialize project** (`tui/`)
  - TypeScript configuration (`tsconfig.json`)
  - Package dependencies: `@opentui/core`, `@opentui/solid`, `solid-js`, `zod`
  - Build configuration

- [ ] **Event schemas** (`tui/src/schemas/events.ts`)
  - Zod schemas for all event types: `user`, `text`, `tool_call`, `tool_result`, `reasoning`, `status`
  - Type inference from schemas

**Acceptance Criteria:**
- Project builds without errors
- Zod schemas validate sample events correctly

---

#### 2.2 Communication Layer

**Why:** Reliable communication is essential before building UI components.

- [ ] **SSE client** (`tui/src/lib/sse.ts`)
  - Connect to `GET /events`
  - Parse event stream
  - Validate events with Zod schemas
  - Handle reconnection on disconnect

- [ ] **REST client** (`tui/src/lib/api.ts`)
  - `submitPrompt(content: string): Promise<void>`
  - `cancelAgent(): Promise<void>`
  - Error handling for failed requests

**Acceptance Criteria:**
| Requirement | Test |
|-------------|------|
| SSE connection | Connects to /events and receives events |
| Reconnection | Automatically reconnects on disconnect |
| Event validation | Invalid events logged/handled gracefully |
| REST calls | submitPrompt and cancelAgent work correctly |

---

#### 2.3 State Management

**Why:** Centralized state ensures UI components react correctly to events.

- [ ] **Conversation store** (`tui/src/stores/conversation.ts`)
  - Parts array: `UserPart | TextPart | ToolPart | ReasoningPart`
  - Reactive updates via Solid.js signals
  - Event handler that updates store

- [ ] **Status store** (`tui/src/stores/status.ts`)
  - States: idle, thinking, running tool, error
  - Current tool name when executing

- [ ] **Input store** (`tui/src/stores/input.ts`)
  - Current input text
  - History array (max 100)
  - History navigation index

**Acceptance Criteria:**
- Stores update reactively when events arrive
- Status reflects current agent state
- Input history persists and navigates correctly

---

#### 2.4 Markdown Renderer

**Why:** Agent text output must be formatted correctly for terminal display.

- [ ] **Markdown renderer** (`tui/src/lib/markdown.ts`)
  - `**bold**` → terminal bold
  - `*italic*` → terminal dim/italic
  - `` `code` `` → highlighted background
  - ```` ``` ```` blocks → yellow background (#3d3a28)
  - `- item` / `* item` → bullet (•)
  - `1. item` → numbered list
  - `[text](url)` → underlined
  - `# Heading` → bold
  - Nested formatting support

**Acceptance Criteria:**
| Markdown | Rendered |
|----------|----------|
| `**bold**` | Bold text |
| `*italic*` | Italic/dim text |
| `` `inline code` `` | Highlighted background |
| Code blocks | Yellow background |
| Lists | Proper bullets/numbers |
| Links | Underlined text |
| Headings | Bold text |
| Nested `**bold *italic***` | Both applied |

---

#### 2.5 UI Components

**Why:** Components are built bottom-up, starting with primitives.

- [ ] **Part components** (`tui/src/components/parts/`)
  - `UserPart` - Cyan accent, full prompt text
  - `TextPart` - Markdown rendered
  - `ToolPart` - Tool name, input JSON, result (truncated to 100 lines), error styling
  - `ReasoningPart` - Dimmed/italicized

- [ ] **Conversation view** (`tui/src/components/Conversation.tsx`)
  - Scrollable area with all parts
  - Auto-scroll on new content
  - Pause auto-scroll when user scrolls up
  - Resume when scrolled to bottom

- [ ] **Input bar** (`tui/src/components/InputBar.tsx`)
  - Text input with cursor
  - Word-boundary wrapping
  - Multiline with Ctrl+Enter
  - Submit with Enter
  - History navigation (Up/Down)
  - Clear with Ctrl+U

- [ ] **Status indicator** (`tui/src/components/Status.tsx`)
  - "Thinking..." when agent responding
  - "Running: {tool_name}..." when tool executing
  - "Error: {message}" on errors

- [ ] **Help overlay** (`tui/src/components/Help.tsx`)
  - Centered modal
  - All keybindings listed
  - Dismiss on any keypress

**Acceptance Criteria:**
| Component | Requirement |
|-----------|-------------|
| UserPart | Cyan styling, full text visible |
| TextPart | Markdown renders correctly |
| ToolPart | Name yellow, result truncated at 100 lines, errors red |
| ReasoningPart | Dimmed/italic styling |
| Conversation | Auto-scroll with manual override |
| InputBar | All input features work |
| Status | Shows correct state |
| Help | Modal displays and dismisses |

---

#### 2.6 Layout & Theme

**Why:** Final assembly of components into the complete application.

- [ ] **Main layout** (`tui/src/App.tsx`)
  - Conversation region (scrollable)
  - Input region (bottom)
  - Status region

- [ ] **Theme** (`tui/src/theme.ts`)
  - Background: #1a1a1a
  - Text: #e0e0e0
  - User prompt: Cyan
  - Tool name: Yellow
  - Error: Red
  - Reasoning: Dimmed gray
  - Code block bg: #3d3a28
  - Status: Yellow

**Acceptance Criteria:**
- Layout renders correctly at various terminal sizes
- Colors match specification

---

#### 2.7 Keybindings

**Why:** Keyboard interaction is critical for terminal UIs.

- [ ] **Implement keybindings**
  - `Enter` - Submit prompt
  - `Ctrl+Enter` - Insert newline
  - `Ctrl+C` - Cancel agent / Exit if idle
  - `Up/Down` - Prompt history (in input)
  - `PageUp/PageDown` - Scroll conversation
  - `Ctrl+U` - Clear input
  - `Ctrl+Shift+C` - Copy selection
  - `?` / `F1` - Toggle help
  - `Esc` - Close help / Cancel selection

**Acceptance Criteria:**
- All keybindings function as specified
- No conflicts between keybindings

---

#### 2.8 Persistence

**Why:** User experience improvement through session persistence.

- [ ] **Prompt history** (`tui/src/lib/history.ts`)
  - Save to `~/.harness/prompt_history`
  - Load on startup
  - Max 100 prompts (FIFO)

**Acceptance Criteria:**
- History persists between sessions
- Oldest entries removed when limit exceeded

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

- Tool implementations: file operations, edge cases, error conditions
- Harness: config validation, event emission timing, termination conditions
- Markdown parser: all formatting combinations
- Zod schemas: valid and invalid event parsing

### Integration Tests

- Harness + Tools: full agent loop with mock API
- TUI + Harness: SSE event flow, REST commands
- History persistence: write/read cycle

### Manual Testing

- Full conversation flow with real API
- All keybindings
- Terminal resize handling
- Long outputs and truncation

---

## Key Implementation Notes

- **Tool choice is always `auto`** — not configurable per spec
- **Streaming is required** — events must be emitted on block completion, not message completion
- **Error handling philosophy** — tool errors go to agent for intelligent handling, not as exceptions
- **Sequential tool execution** — no parallel execution; fail-fast on error
- **History limit** — 100 prompts max, oldest removed first (FIFO)
- **Event emission** — use `ContentBlockStopEvent` from streaming API, accumulate with `message.Accumulate(event)`
