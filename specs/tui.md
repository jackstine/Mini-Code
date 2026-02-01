# TUI Specification

## Overview

A terminal user interface for interacting with the agent harness. Built with TypeScript using Solid.js and OpenTUI.

## Packages

| Package | Purpose |
|---------|---------|
| `@opentui/core` | Core terminal rendering engine |
| `@opentui/solid` | Solid.js component bindings |
| `solid-js` | Reactive framework |
| `zod` | Schema validation for events |

## Communication

The TUI communicates with the harness via:

| Direction | Protocol | Purpose |
|-----------|----------|---------|
| Harness → TUI | SSE (Server-Sent Events) | Stream events in real-time |
| TUI → Harness | REST (HTTP POST) | Send prompts, cancel requests |

### SSE Endpoint

```
GET /events
Content-Type: text/event-stream

data: {"type": "text", "content": "..."}

data: {"type": "tool_call", "id": "...", "name": "...", "input": {...}}
```

### REST Endpoints

| Endpoint | Purpose |
|----------|---------|
| `POST /prompt` | Submit user prompt |
| `POST /cancel` | Cancel running agent |

## Parts

The conversation is displayed as a sequence of parts. Each part has a type and associated data.

### UserPart

Displays the user's submitted prompt.

```typescript
type UserPart = {
  type: "user"
  content: string
  timestamp: number
}
```

**Display:**
- Visually distinct from agent output (e.g., different background or prefix)
- Shows the full prompt text
- Wrapped to terminal width

### TextPart

Displays text output from the agent.

```typescript
type TextPart = {
  type: "text"
  content: string
  timestamp: number
}
```

**Display:**
- Wrapped to terminal width at word boundaries
- Markdown rendered (see below)

### Markdown Rendering

Agent text output supports basic markdown:

| Markdown | Rendered |
|----------|----------|
| `**bold**` | Bold text (terminal bold) |
| `*italic*` | Italic text (terminal italic/dim) |
| `` `inline code` `` | Highlighted background |
| ```` ``` ```` code blocks | Yellow background (see Theme) |
| `- item` or `* item` | Bullet point (•) |
| `1. item` | Numbered list |
| `[text](url)` | Underlined text |
| `# Heading` | Bold text |

**Notes:**
- Links are underlined but not clickable (terminal limitation)
- Nested formatting supported (e.g., `**bold and *italic***`)
- Unknown/unsupported markdown passed through as plain text

### ToolPart

Displays a tool invocation and its result.

```typescript
type ToolPart = {
  type: "tool"
  id: string
  name: string
  input: Record<string, unknown>
  result: string | null
  isError: boolean
  timestamp: number
}
```

**Display:**
- Tool name prominently shown
- Input parameters displayed in full
- Result truncated to **100 lines maximum**
  - If truncated, show indicator: `... (X more lines)`
- Errors visually distinct (e.g., red text)

### ReasoningPart

Displays the agent's thinking/reasoning blocks.

```typescript
type ReasoningPart = {
  type: "reasoning"
  content: string
  timestamp: number
}
```

**Display:**
- Visually distinct from regular text (e.g., dimmed or italicized)
- Shows the full reasoning content

## Layout

Single panel layout with scrolling feed:

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  [User] What's in config.json?                          │
│                                                         │
│  [Agent] I'll read that file for you.                   │
│                                                         │
│  [Tool: read]                                           │
│  Input: {"path": "config.json"}                         │
│  Result: {"content": "port=8080\nhost=localhost"}       │
│                                                         │
│  [Agent] The config file contains:                      │
│  - port: 8080                                           │
│  - host: localhost                                      │
│                                                         │
│                                                         │
│                                                         │
├─────────────────────────────────────────────────────────┤
│ > Enter prompt here...                             [...]│
└─────────────────────────────────────────────────────────┘
```

### Regions

| Region | Description |
|--------|-------------|
| **Conversation** | Scrollable area showing all parts |
| **Input** | Text input bar at bottom |
| **Status** | Indicator when agent is working |

### Scrolling Behavior

- **Auto-scroll**: New content automatically scrolls to bottom
- **Manual scroll**: When user scrolls up, auto-scroll pauses
- **Resume auto-scroll**: When user scrolls to bottom, auto-scroll resumes
- Scroll position indicator optional (e.g., "↓ New content" when paused)

### Text Wrapping

- All text wraps at **word boundaries**
- Long words without spaces wrap at character boundary as fallback
- Respects terminal width on resize

## Input Behavior

| Feature | Behavior |
|---------|----------|
| Text entry | Standard text input with cursor |
| Copy/paste | Supported (system clipboard) |
| Line wrapping | Text wraps at word boundaries within input area |
| Multiline | `Ctrl+Enter` inserts newline |
| Submit | `Enter` submits prompt |
| History | Up/Down arrow cycles through previous prompts |

### Prompt History

- History is **persisted between sessions** (saved to local file)
- Maximum **100 prompts** stored
- Oldest prompts removed when limit exceeded
- Up arrow: previous prompt, Down arrow: next prompt
- History file location: `~/.harness/prompt_history`

### Selection and Copy

- User can select text in the conversation area
- Selection uses standard terminal selection (mouse drag or shift+arrow)
- Copy to system clipboard with `Ctrl+Shift+C` or terminal default
- Selected text highlighted with inverted colors

## Keybindings

| Key | Action |
|-----|--------|
| `Enter` | Submit prompt |
| `Ctrl+Enter` | Insert newline in prompt |
| `Ctrl+C` | Cancel running agent / Exit if idle |
| `Up/Down` (in input) | Cycle through prompt history |
| `PageUp/PageDown` | Scroll conversation |
| `Ctrl+U` | Clear input |
| `Ctrl+Shift+C` | Copy selected text |
| `?` or `F1` | Toggle help screen |
| `Esc` | Close help screen / Cancel selection |

## Help Screen

Pressing `?` or `F1` displays an overlay with all keybindings:

```
┌─────────────────────────────────────────────────────────┐
│                     Help - Keybindings                  │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Enter              Submit prompt                       │
│  Ctrl+Enter         Insert newline                      │
│  Ctrl+C             Cancel agent / Exit                 │
│  Up/Down            Prompt history                      │
│  PageUp/PageDown    Scroll conversation                 │
│  Ctrl+U             Clear input                         │
│  Ctrl+Shift+C       Copy selection                      │
│  ?  or  F1          Toggle this help                    │
│  Esc                Close help / Cancel                 │
│                                                         │
│                    Press any key to close               │
└─────────────────────────────────────────────────────────┘
```

- Displayed as a centered modal overlay
- Dismisses on any keypress
- Does not interrupt agent execution

## Theme

Dark mode with minimal color palette:

| Element | Color |
|---------|-------|
| Background | Dark gray (#1a1a1a) |
| Text | Light gray (#e0e0e0) |
| User prompt | Cyan accent |
| Agent text | Default light gray |
| Tool name | Yellow |
| Tool result | Default light gray |
| Error | Red |
| Reasoning | Dimmed gray |
| Input border | Subtle gray |
| Status | Yellow (pulsing optional) |
| Code block background | Light transparent yellow (#3d3a28 or similar) |

### Code Block Highlighting

Code blocks in agent output (text between triple backticks) are rendered with:
- Light transparent yellow background (subtle, works on dark mode)
- Monospace font (terminal default)
- Syntax highlighting optional (future enhancement)

## Status Indicators

| State | Display |
|-------|---------|
| Idle | No indicator |
| Agent responding | "Thinking..." |
| Tool executing | "Running: {tool_name}..." |
| Error | "Error: {message}" (temporary) |

The status appears above or within the input area.

## Event Flow

```
1. User types prompt in input bar
2. User presses Enter
   └─> TUI sends POST /prompt
   └─> TUI displays UserPart
   └─> TUI shows "Thinking..." status

3. Harness streams events via SSE
   └─> TUI receives TextPart → displays text
   └─> TUI receives ToolPart (start) → shows "Running: read..."
   └─> TUI receives ToolPart (complete) → displays full result
   └─> TUI receives ReasoningPart → displays reasoning

4. Stream ends
   └─> TUI clears status indicator
   └─> User can enter next prompt
```
