# Implementation Plan: New TUI Features and Tools

## Overview

This plan covers the implementation of features from commits 756e1e7 and 50cb96d:
- **TUI Features:** Header display, Input box growth
- **Backend Tools:** Bash, Write, Edit, Move

## Current State

**Status: IN PROGRESS**

### Existing Infrastructure
- Tool interface defined in `pkg/tool/tool.go`
- Seven tools implemented: `read`, `list_dir`, `grep`, `bash`, `write`, `edit`, `move`
- Tool registration in `cmd/harness/main.go` (lines 44-51)
- TUI conversation store with Part types in `tui/src/stores/conversation.ts`
- Input bar with fixed height (3 lines) in `tui/src/components/InputBar.tsx`
- Header asset exists at `tui/assets/heading.md`
- OpenTUI framework with box, input, text, scrollbox components
- Markdown rendering library in `tui/src/lib/markdown.tsx`

---

## Acceptance Criteria

### Bash Tool (`specs/tools/bash.md`) ✅ COMPLETE
- [x] Execute commands using `/bin/bash -c "<command>"`
- [x] Capture stdout and stderr separately
- [x] Return exit code with output
- [x] 30-second timeout with process termination
- [x] 1 MB output limits with truncation ("... (truncated)" suffix)
- [x] Return error for: empty command, timeout, spawn failure
- [x] Commands execute in harness working directory
- [x] Inherit harness process environment

### Write Tool (`specs/tools/write.md`) ✅ COMPLETE
- [x] Write content to file (overwrite mode by default)
- [x] Support append mode via `mode` parameter
- [x] Create parent directories recursively (0755)
- [x] Atomic writes (temp file + rename)
- [x] New files created with 0644 permissions
- [x] Existing file permissions preserved
- [x] Return bytesWritten and absolute path
- [x] Return error for: empty path, path is directory, permission denied, disk full

### Edit Tool (`specs/tools/edit.md`) ✅ COMPLETE
- [x] Support three operations: replace, insert, delete
- [x] 1-indexed line numbers, inclusive ranges
- [x] `afterLine: 0` inserts at beginning of file
- [x] Process operations in reverse line order to preserve validity
- [x] Validate all operations before applying any
- [x] Detect and reject overlapping operations
- [x] Atomic file writes (temp file + rename)
- [x] Return path, linesChanged, newLineCount
- [x] Return errors for: file not found, directory, permission denied, invalid line numbers, overlapping operations

### Move Tool (`specs/tools/move.md`) ✅ COMPLETE
- [x] Rename files (same directory)
- [x] Move files to different directory
- [x] Move into existing directory (preserve filename)
- [x] Create parent directories if needed (0755)
- [x] Handle cross-filesystem moves (copy + delete)
- [x] Overwrite existing files, error on non-empty directories
- [x] Prevent moving directory into itself
- [x] Return source and destination absolute paths

### Header Display (`specs/header.md`)
- [ ] Load header from `tui/assets/heading.md` at startup
- [ ] Display as first item in conversation (before any messages)
- [ ] Apply blue color to entire header block
- [ ] Preserve all whitespace and line breaks
- [ ] One blank line below header before first message
- [ ] Show only once per session (at startup)
- [ ] Skip display if file missing or empty (log warning)

### Input Growth (`specs/input-growth.md`)
- [ ] Minimum height: 1 line
- [ ] Maximum height: 10 lines
- [ ] Grow when text wraps to new line
- [ ] Shrink when content fits in fewer lines
- [ ] Enable scroll within input when max height exceeded
- [ ] Keep cursor visible at all times
- [ ] Recalculate on: text input, deletion, terminal resize, paste
- [ ] Conversation area shrinks to accommodate larger input

---

## Implementation Items

### 1. Create Bash Tool (`pkg/tool/bash.go`)

**Priority: HIGH** - Most commonly needed tool for agent operations

**Why:** Enables the agent to execute arbitrary commands, essential for git operations, building projects, running tests, and system exploration.

**Files to create:**
- `pkg/tool/bash.go`
- `pkg/tool/bash_test.go`

**Implementation details:**
```go
type BashTool struct{}

type bashInput struct {
    Command string `json:"command"`
}

type bashOutput struct {
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    ExitCode int    `json:"exitCode"`
}
```

**Key behaviors:**
- Use `exec.CommandContext` with 30-second timeout
- Create cmd with `/bin/bash -c` and the command string
- Capture stdout/stderr with separate buffers
- Check output sizes, truncate at 1MB with "... (truncated)" suffix
- Return exit code from `cmd.ProcessState.ExitCode()`

**Tests to add:**
- Execute simple command (echo)
- Capture stderr (ls nonexistent)
- Return correct exit code on failure
- Timeout handling (sleep command)
- Output truncation at 1MB limit
- Empty command error
- Context cancellation

---

### 2. Create Write Tool (`pkg/tool/write.go`)

**Priority: HIGH** - Required for file creation and modification

**Why:** Enables the agent to create new files and modify existing ones. Foundation for code generation tasks.

**Files to create:**
- `pkg/tool/write.go`
- `pkg/tool/write_test.go`

**Implementation details:**
```go
type WriteTool struct{}

type writeInput struct {
    Path    string `json:"path"`
    Content string `json:"content"`
    Mode    string `json:"mode,omitempty"` // "overwrite" or "append"
}

type writeOutput struct {
    BytesWritten int    `json:"bytesWritten"`
    Path         string `json:"path"`
}
```

**Key behaviors:**
- Resolve relative paths to absolute
- Create parent directories with `os.MkdirAll(dir, 0755)`
- Atomic write: create temp file in same directory, write, rename
- Append mode: open with `os.O_APPEND|os.O_WRONLY`
- Preserve existing file permissions, use 0644 for new files

**Tests to add:**
- Create new file
- Overwrite existing file
- Append to existing file
- Create nested directories
- Preserve file permissions
- Error on directory path
- Error on permission denied

---

### 3. Create Edit Tool (`pkg/tool/edit.go`)

**Priority: HIGH** - Essential for precise code modifications

**Why:** Allows the agent to make surgical edits to existing files without rewriting entire content. Critical for refactoring and bug fixes.

**Files to create:**
- `pkg/tool/edit.go`
- `pkg/tool/edit_test.go`

**Implementation details:**
```go
type EditTool struct{}

type editInput struct {
    Path       string      `json:"path"`
    Operations []Operation `json:"operations"`
}

type Operation struct {
    Op        string   `json:"op"`        // "replace", "insert", "delete"
    StartLine int      `json:"startLine,omitempty"`
    EndLine   int      `json:"endLine,omitempty"`
    AfterLine int      `json:"afterLine,omitempty"`
    Content   []string `json:"content,omitempty"`
}

type editOutput struct {
    Path         string `json:"path"`
    LinesChanged int    `json:"linesChanged"`
    NewLineCount int    `json:"newLineCount"`
}
```

**Key behaviors:**
- Read file into string slice (split by newline)
- Validate all operations before applying:
  - Line numbers in range (1-indexed)
  - startLine <= endLine
  - No overlapping operations
- Sort operations by position descending
- Apply operations using splice pattern (per spec pseudocode)
- Atomic write with temp file + rename

**Tests to add:**
- Single line replace
- Multi-line replace
- Insert at beginning (afterLine: 0)
- Insert in middle
- Delete single line
- Delete range
- Multiple non-overlapping operations
- Overlap detection error
- Invalid line number error
- File not found error

---

### 4. Create Move Tool (`pkg/tool/move.go`)

**Priority: MEDIUM** - Useful for refactoring but less frequently needed

**Why:** Enables file reorganization and renaming. Useful for refactoring operations.

**Files to create:**
- `pkg/tool/move.go`
- `pkg/tool/move_test.go`

**Implementation details:**
```go
type MoveTool struct{}

type moveInput struct {
    Source      string `json:"source"`
    Destination string `json:"destination"`
}

type moveOutput struct {
    Source      string `json:"source"`
    Destination string `json:"destination"`
}
```

**Key behaviors:**
- Resolve both paths to absolute
- Detect if destination is existing directory (move into it)
- Create parent directories with `os.MkdirAll(dir, 0755)`
- Attempt `os.Rename` first (same filesystem)
- Fall back to copy+delete for cross-filesystem
- Check for self-move (destination inside source directory)

**Tests to add:**
- Rename file (same directory)
- Move to different directory
- Move into existing directory
- Create parent directories
- Cross-filesystem move (copy+delete)
- Error: source not found
- Error: move directory into itself
- Error: overwrite non-empty directory

---

### 5. Register New Tools in main.go

**Priority: HIGH** - Required to make tools available

**Why:** Tools must be registered with the harness to be exposed to the AI agent.

**Modify `cmd/harness/main.go`:**
```go
// Create tools
tools := []tool.Tool{
    tool.NewReadTool(),
    tool.NewListDirTool(),
    tool.NewGrepTool(),
    tool.NewBashTool(),   // Add
    tool.NewWriteTool(),  // Add
    tool.NewEditTool(),   // Add
    tool.NewMoveTool(),   // Add
}
```

Update console output to list all tools.

---

### 6. Add Header Part Type to TUI

**Priority: MEDIUM** - Visual enhancement for user experience

**Why:** Provides a branded welcome experience when the application starts.

**Files to modify:**
- `tui/src/stores/conversation.ts` - Add header type to Part union
- `tui/src/components/Conversation.tsx` - Add Match case for header
- `tui/src/components/parts/HeaderPart.tsx` - Create new component
- `tui/src/App.tsx` - Load header on mount

**Implementation details:**

Add to Part type:
```typescript
| { type: "header"; content: string; timestamp: number }
```

Create HeaderPart component:
```typescript
export const HeaderPart: Component<{ content: string }> = (props) => {
  return (
    <box marginBottom={1}>
      <text color="blue">{props.content}</text>
    </box>
  )
}
```

Load header in App.tsx:
```typescript
onMount(async () => {
  try {
    const content = await loadHeader()
    if (content) {
      setParts(produce(p => p.unshift({
        type: "header",
        content,
        timestamp: Date.now()
      }))
    }
  } catch (e) {
    console.warn("Failed to load header:", e)
  }
})
```

---

### 7. Implement Input Box Growth

**Priority: MEDIUM** - Improves multi-line input experience

**Why:** Allows users to compose longer prompts comfortably without the text being obscured.

**Files to modify:**
- `tui/src/components/InputBar.tsx` - Add dynamic height calculation
- `tui/src/lib/text.ts` - Add text wrapping utility (if needed)

**Implementation details:**

Add height calculation:
```typescript
const MIN_INPUT_HEIGHT = 1
const MAX_INPUT_HEIGHT = 10

function calculateInputHeight(content: string, width: number): number {
  if (!content) return MIN_INPUT_HEIGHT

  // Split by newlines first
  const lines = content.split('\n')
  let totalLines = 0

  for (const line of lines) {
    // Calculate wrapped lines for each line
    const wrappedLines = Math.ceil((line.length || 1) / width)
    totalLines += wrappedLines
  }

  return Math.min(Math.max(totalLines, MIN_INPUT_HEIGHT), MAX_INPUT_HEIGHT)
}
```

Modify InputBar:
```typescript
export const InputBar: Component = () => {
  const [dimensions, setDimensions] = createSignal({ width: 80 })

  // Calculate height based on content and width
  const inputHeight = createMemo(() => {
    const width = dimensions().width - 2 // Account for border
    return calculateInputHeight(inputText(), width)
  })

  return (
    <box
      width="100%"
      height={inputHeight() + 2} // +2 for border
      border={true}
      onLayout={(e) => setDimensions({ width: e.width })}
    >
      {/* ... */}
    </box>
  )
}
```

**Tests to verify:**
- Single character: height = 1
- Full line: height = 1
- Line wrap triggers: height = 2
- Multiple newlines: each counts as line
- Delete reduces height
- Max height caps at 10
- Terminal resize recalculates

---

## Implementation Order

The items should be implemented in this order based on dependencies and value:

1. **Backend Tools (Items 1-4)**
   - Bash, Write, Edit, Move tools
   - Each tool is independent, can be developed in parallel
   - Tests ensure correct behavior before integration

2. **Tool Registration (Item 5)**
   - Wire up tools in main.go
   - Verify tools appear in agent tool list

3. **TUI Header Display (Item 6)**
   - Add header part type
   - Create component
   - Load on startup

4. **TUI Input Growth (Item 7)**
   - Modify InputBar component
   - Add height calculation
   - Test resize behavior

---

## Testing Strategy

### Backend Tool Testing
Each tool should have comprehensive unit tests:
- Success cases with expected output format
- Error cases with proper error messages
- Edge cases (empty input, large output, permissions)
- Context cancellation handling
- Timeout behavior (bash tool)

### TUI Testing
Manual verification:
- Header displays on first load
- Header scrolls with conversation
- Input grows as text is entered
- Input shrinks when text deleted
- Max height respected
- Scroll works within max-height input

### Integration Testing
- Agent can successfully use each tool
- Tool results display correctly in TUI
- Error responses handled gracefully

---

## Non-Goals (Out of Scope)

- Tool timeout configuration (use 30s default per spec)
- Tool sandboxing or restrictions (full filesystem access per spec)
- Binary file support for write tool
- Input growth animation (spec says "smooth" but OpenTUI may not support)
- Header customization (use fixed asset file)

---

## Logs

### 2026-02-01: Backend Tools Complete
- **Errors:** None
- **All Tests Pass:** Yes
- **Notes:** Implemented all four backend tools (bash, write, edit, move) with comprehensive test coverage. All tools registered in main.go. Remaining work: TUI header display and input growth features.
