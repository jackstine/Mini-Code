# Implementation Plan - Code References

This document contains all code examples and implementations referenced in IMPLEMENTATION_PLAN.md.

---

## 1.1 Tool Interface {#tool-interface}

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (string, error)
}
```

---

## 1.2 READ Tool Schemas {#read-tool-schemas}

### Input Schema

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

### Output Format

Success:
```json
{"content": "file contents as string"}
```

Error:
```json
{"error": "error message"}
```

---

## 1.3 LIST_DIR Tool Schemas {#list-dir-tool-schemas}

### Input Schema

```json
{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Directory path to list"}
  },
  "required": ["path"]
}
```

### Output Format

Success:
```json
{"entries": "raw ls -al output as string"}
```

Error:
```json
{"error": "error message"}
```

---

## 1.4 GREP Tool Schemas {#grep-tool-schemas}

### Input Schema

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

### Output Format

Success:
```json
{"matches": "grep output as string"}
```

Error:
```json
{"error": "error message"}
```

---

## 1.5 Config and EventHandler {#config-eventhandler}

### Config Struct

```go
type Config struct {
    APIKey       string  // Required - Anthropic API key
    Model        string  // Default: "claude-3-haiku-20240307"
    MaxTokens    int     // Default: 4096
    SystemPrompt string  // Optional
    MaxTurns     int     // Default: 10
}
```

### EventHandler Interface

```go
type EventHandler interface {
    OnText(text string)
    OnToolCall(id string, name string, input json.RawMessage)
    OnToolResult(id string, result string, isError bool)
}
```

---

## 1.7 Agent Loop Implementation {#agent-loop}

### Main Agent Loop

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
```

### Tool Execution

```go
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

### Block Type Detection

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

---

## 1.8 HTTP Server Implementation {#http-server}

### SSE Endpoint Handler

```go
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
```

### Broadcast Implementation

```go
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
```

### SSE EventHandler

```go
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

### Event Types

```json
{"type": "user", "content": "...", "timestamp": 1234567890}
{"type": "text", "content": "...", "timestamp": 1234567890}
{"type": "tool_call", "id": "...", "name": "...", "input": {...}, "timestamp": 1234567890}
{"type": "tool_result", "id": "...", "result": "...", "isError": false, "timestamp": 1234567890}
{"type": "reasoning", "content": "...", "timestamp": 1234567890}
{"type": "status", "state": "...", "message": "..."}
```

---

## 2.1 TUI Project Setup {#tui-project-setup}

### Project Initialization

```bash
cd tui
bun init
bun add @opentui/core @opentui/solid solid-js zod
```

### TypeScript Configuration

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

### Event Schemas (Zod)

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

---

## 2.2 Communication Layer {#communication-layer}

### SSE Client

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

### REST Client

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

---

## 2.3 State Management {#state-management}

### Conversation Store

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

### Status Store

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

### Input Store

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

---

## 2.5 UI Components {#ui-components}

### UserPart Component

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

### ToolPart Component

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

### Conversation Component

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

### InputBar Component

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

### Help Component

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

---

## 2.6 Layout & Theme {#layout-theme}

### Main Entry Point

```tsx
// tui/src/index.tsx
import { render } from "@opentui/solid"
import { App } from "./App"

render(App, {
  targetFps: 30,
  exitOnCtrlC: false, // We handle Ctrl+C manually
})
```

### App Component

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

### Theme Constants

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

---

## Dependencies {#dependencies}

### Go Module

```go
// go.mod
module github.com/user/harness

go 1.21

require (
    github.com/anthropics/anthropic-sdk-go v0.2.0
)
```

### TypeScript Package

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

## Tool Registration Pattern {#tool-registration}

### Converting Tool to API Format

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
