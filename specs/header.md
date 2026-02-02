# Header Specification

## Overview

Display a welcome header when the TUI application starts for the first time.

## Header Source

The header content is loaded from `tui/assets/heading.md`. This file contains pre-formatted ASCII art or text that serves as the application banner.

## Display Behavior

| Condition | Behavior |
|-----------|----------|
| First launch | Display header at top of conversation area |
| Subsequent content | Header scrolls up with conversation |
| Application restart | Header displays again (per-session) |

## Styling

| Property | Value |
|----------|-------|
| Color | Blue (terminal blue, #3b82f6 or ANSI blue) |
| Position | Top of conversation area, before any messages |
| Alignment | Left-aligned |
| Padding | One blank line below header before first message |

## Rendering

- Header content is rendered as plain text (no markdown processing)
- Preserve all whitespace and line breaks from the source file
- Apply blue color to entire header block
- Respect terminal width (no wrapping of header lines)

## Implementation Notes

### File Loading

```typescript
// Load header content at application startup
const headerContent = await loadFile("tui/assets/heading.md")
```

### Display Component

```typescript
type HeaderPart = {
  type: "header"
  content: string
}
```

The header is treated as a special part that appears before all conversation content.

### Color Application

Apply blue foreground color using terminal escape codes or OpenTUI styling:

```typescript
<Text color="blue">{headerContent}</Text>
```

## Edge Cases

| Case | Behavior |
|------|----------|
| Header file missing | Skip header display, log warning |
| Header file empty | Skip header display |
| Terminal narrower than header | Allow horizontal overflow (no wrap) |
