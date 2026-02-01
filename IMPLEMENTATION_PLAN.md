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

#### 1.1 Tool Interface & Implementations

**Why:** Tools are independent units with no external dependencies. They form the foundation that the harness executes against. Implementing them first allows isolated testing and establishes the Tool interface contract.

- [ ] **Define Tool interface** (`pkg/tool/tool.go`)
  - `Name() string`
  - `Description() string`
  - `InputSchema() json.RawMessage`
  - `Execute(ctx context.Context, input json.RawMessage) (string, error)`

- [ ] **Implement READ tool** (`pkg/tool/read.go`)
  - Accept `path`, optional `start_line`, `end_line`
  - Return JSON `{"content": "..."}` or `{"error": "..."}`
  - Handle: file not found, is directory, permission denied, invalid line ranges
  - 1-indexed lines, inclusive end_line

- [ ] **Implement GREP tool** (`pkg/tool/grep.go`)
  - Accept `pattern`, `path`, optional `recursive`
  - Use `/usr/bin/grep` with BRE
  - Return JSON `{"matches": "..."}` or `{"error": "..."}`
  - Empty matches = success (not error)
  - Handle: path not found, permission denied, invalid regex

- [ ] **Implement LIST_DIR tool** (`pkg/tool/list_dir.go`)
  - Accept `path`
  - Execute `ls -al <path>`
  - Return JSON `{"entries": "..."}` or `{"error": "..."}`
  - Handle: path not found, not a directory, permission denied

#### 1.2 Core Harness

**Why:** The harness is the central orchestrator. It must be implemented before the HTTP server since it contains all the business logic for API interaction, conversation management, and tool execution.

- [ ] **Config struct** (`pkg/harness/config.go`)
  - `APIKey` (required)
  - `Model` (default: `claude-3-haiku-20240307`)
  - `MaxTokens` (default: 4096)
  - `SystemPrompt` (optional)
  - `MaxTurns` (default: 10)

- [ ] **EventHandler interface** (`pkg/harness/events.go`)
  - `OnText(text string)`
  - `OnToolCall(id, name string, input json.RawMessage)`
  - `OnToolResult(id, result string, isError bool)`

- [ ] **Harness struct** (`pkg/harness/harness.go`)
  - Constructor: `NewHarness(config Config, tools []Tool, handler EventHandler) *Harness`
  - Conversation state management (ordered message list)
  - Concurrency control (single Prompt at a time)

- [ ] **Prompt method** (`pkg/harness/harness.go`)
  - `func (h *Harness) Prompt(ctx context.Context, content string) error`
  - Append user message to history
  - Emit `user` event
  - Run agent loop until termination

- [ ] **Cancel method** (`pkg/harness/harness.go`)
  - `func (h *Harness) Cancel()`
  - Safe to call when no prompt running

#### 1.3 Agent Loop

**Why:** The agent loop is the core algorithm that drives the harness. It must correctly implement the send→receive→execute→repeat cycle with proper streaming and event emission.

- [ ] **Streaming API integration**
  - Use `client.Messages.NewStreaming()` from anthropic-sdk-go
  - Accumulate deltas with `message.Accumulate(event)`
  - Emit events on `ContentBlockStopEvent`

- [ ] **Tool execution logic**
  - Sequential execution in response order
  - Fail-fast: stop on first error
  - Serialize all results (including errors) as tool result messages
  - Errors go back to agent (not exceptions)

- [ ] **Termination conditions**
  - No tool calls in response → end
  - MaxTurns exceeded → end
  - API error → return error
  - Context cancelled → return error

#### 1.4 HTTP Server

**Why:** The HTTP server is the interface between harness and TUI. It must be implemented after the harness core is stable, but before the TUI can be developed.

- [ ] **SSE endpoint** (`GET /events`)
  - Content-Type: `text/event-stream`
  - Event format: `data: {"type": "...", ...}\n\n`
  - Heartbeat every 30 seconds (`: heartbeat\n`)
  - Event types: `user`, `text`, `tool_call`, `tool_result`, `reasoning`, `status`

- [ ] **REST endpoints**
  - `POST /prompt` - Body: `{"content": "..."}`
  - `POST /cancel` - Empty body

- [ ] **Event bridging**
  - Implement EventHandler that broadcasts to SSE clients
  - Include timestamps in all events

---

### Phase 2: Terminal UI (TypeScript)

#### 2.1 Project Setup

**Why:** Establish the TypeScript project structure with proper tooling before building components.

- [ ] **Initialize project**
  - TypeScript configuration
  - Package dependencies: `@opentui/core`, `@opentui/solid`, `solid-js`, `zod`
  - Build configuration

- [ ] **Event schemas** (`src/schemas/events.ts`)
  - Zod schemas for all event types
  - Type inference from schemas

#### 2.2 Communication Layer

**Why:** The communication layer must be solid before building UI components that depend on it.

- [ ] **SSE client** (`src/lib/sse.ts`)
  - Connect to `GET /events`
  - Parse event stream
  - Validate events with Zod schemas
  - Handle reconnection

- [ ] **REST client** (`src/lib/api.ts`)
  - `submitPrompt(content: string): Promise<void>`
  - `cancelAgent(): Promise<void>`

#### 2.3 State Management

**Why:** Centralized state management ensures UI components react correctly to events.

- [ ] **Conversation store** (`src/stores/conversation.ts`)
  - Parts array: `UserPart | TextPart | ToolPart | ReasoningPart`
  - Reactive updates via Solid.js signals
  - Event handler that updates store

- [ ] **Status store** (`src/stores/status.ts`)
  - States: idle, thinking, running tool, error
  - Current tool name when executing

- [ ] **Input store** (`src/stores/input.ts`)
  - Current input text
  - History array (max 100)
  - History navigation index

#### 2.4 UI Components

**Why:** Components are built bottom-up, starting with primitives and composing into the full layout.

- [ ] **Part components** (`src/components/parts/`)
  - `UserPart` - Cyan accent, full prompt text
  - `TextPart` - Markdown rendered (see below)
  - `ToolPart` - Tool name, input JSON, result (truncated to 100 lines), error styling
  - `ReasoningPart` - Dimmed/italicized

- [ ] **Markdown renderer** (`src/lib/markdown.ts`)
  - `**bold**` → terminal bold
  - `*italic*` → terminal dim/italic
  - `` `code` `` → highlighted background
  - ```` ``` ```` blocks → yellow background (#3d3a28)
  - `- item` / `* item` → bullet (•)
  - `1. item` → numbered list
  - `[text](url)` → underlined
  - `# Heading` → bold
  - Nested formatting support

- [ ] **Conversation view** (`src/components/Conversation.tsx`)
  - Scrollable area with all parts
  - Auto-scroll on new content
  - Pause auto-scroll when user scrolls up
  - Resume when scrolled to bottom

- [ ] **Input bar** (`src/components/InputBar.tsx`)
  - Text input with cursor
  - Word-boundary wrapping
  - Multiline with Ctrl+Enter
  - Submit with Enter
  - History navigation (Up/Down)
  - Clear with Ctrl+U

- [ ] **Status indicator** (`src/components/Status.tsx`)
  - "Thinking..." when agent responding
  - "Running: {tool_name}..." when tool executing
  - "Error: {message}" on errors

- [ ] **Help overlay** (`src/components/Help.tsx`)
  - Centered modal
  - All keybindings listed
  - Dismiss on any keypress

#### 2.5 Layout & Theme

**Why:** Final assembly of components into the complete application.

- [ ] **Main layout** (`src/App.tsx`)
  - Conversation region (scrollable)
  - Input region (bottom)
  - Status region

- [ ] **Theme** (`src/theme.ts`)
  - Background: #1a1a1a
  - Text: #e0e0e0
  - User prompt: Cyan
  - Tool name: Yellow
  - Error: Red
  - Reasoning: Dimmed gray
  - Code block bg: #3d3a28
  - Status: Yellow

#### 2.6 Keybindings

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

#### 2.7 Persistence

**Why:** User experience improvement through session persistence.

- [ ] **Prompt history** (`src/lib/history.ts`)
  - Save to `~/.harness/prompt_history`
  - Load on startup
  - Max 100 prompts (FIFO)

---

## Acceptance Criteria

### Harness (Go)

| Requirement | Acceptance Criteria |
|-------------|---------------------|
| Config defaults | Harness initializes with correct defaults when optional fields omitted |
| Tool registration | All tools appear in API requests with correct schemas |
| Streaming | Events emitted on ContentBlockStopEvent, not before |
| Tool execution | Tools execute sequentially; first error stops execution |
| Error propagation | Tool errors serialized to agent, not thrown as exceptions |
| Concurrency | Second Prompt() call returns error while first is running |
| Cancellation | Cancel() stops running prompt; safe when idle |
| MaxTurns | Agent loop terminates after MaxTurns iterations |
| SSE heartbeat | Heartbeat sent every 30 seconds |
| Event format | All events include type and timestamp |

### Tools

| Tool | Acceptance Criteria |
|------|---------------------|
| read | Returns file content; handles line ranges (1-indexed, inclusive); errors for missing/directory/permission |
| grep | Uses /usr/bin/grep with BRE; empty matches = success; recursive flag works |
| list_dir | Returns ls -al output including hidden files; errors for non-directory |

### TUI

| Requirement | Acceptance Criteria |
|-------------|---------------------|
| SSE connection | Connects to /events; handles reconnection |
| Event display | All event types render with correct styling |
| Markdown | Bold, italic, code, code blocks, lists, links, headings render correctly |
| Tool result truncation | Results > 100 lines show truncation indicator |
| Auto-scroll | New content scrolls to bottom; manual scroll pauses; resume at bottom |
| Input | Text entry, cursor, multiline (Ctrl+Enter), submit (Enter) work |
| History | Up/Down navigates history; persisted to ~/.harness/prompt_history |
| Keybindings | All specified keybindings function correctly |
| Help overlay | ? and F1 toggle; any key dismisses |
| Status | Shows correct state (idle/thinking/running/error) |
| Theme | Colors match specification |

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

## Notes

- **Tool choice is always `auto`** - not configurable per spec
- **Streaming is required** - events must be emitted on block completion, not message completion
- **Error handling philosophy** - tool errors go to agent for intelligent handling, not as exceptions
- **Sequential tool execution** - no parallel execution; fail-fast on error
- **History limit** - 100 prompts max, oldest removed first (FIFO)
