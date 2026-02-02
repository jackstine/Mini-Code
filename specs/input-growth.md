# Input Growth Specification

## Overview

The input text box dynamically grows in height as the user types, expanding when text reaches the boundary.

## Growth Behavior

| Condition | Behavior |
|-----------|----------|
| Text fits on one line | Input height: 1 line |
| Text wraps to new line | Input height increases by 1 line |
| Text deleted | Input height shrinks to fit content |
| Empty input | Input height: 1 line (minimum) |

## Constraints

| Property | Value |
|----------|-------|
| Minimum height | 1 line |
| Maximum height | 10 lines |
| Growth trigger | Text reaches right boundary |
| Shrink trigger | Content fits in fewer lines |

## Wrapping

- Text wraps at word boundaries when reaching terminal width
- Long words without spaces wrap at character boundary
- Input width remains constant (full terminal width minus borders)

## Scroll Behavior

When input exceeds maximum height:

| State | Behavior |
|-------|----------|
| Typing at end | Auto-scroll to show cursor |
| Cursor in middle | Scroll to keep cursor visible |
| Maximum reached | Content scrolls within fixed max-height area |

## Layout Impact

| Component | Behavior |
|-----------|----------|
| Conversation area | Shrinks to accommodate larger input |
| Input border | Expands with content |
| Status indicator | Stays positioned relative to input |

## Visual Feedback

- Input area smoothly expands (no jarring jumps)
- Border adjusts to contain all content
- Cursor remains visible at all times

## Implementation Notes

### Height Calculation

```typescript
function calculateInputHeight(content: string, terminalWidth: number): number {
  const lines = wrapText(content, terminalWidth)
  return Math.min(Math.max(lines.length, MIN_HEIGHT), MAX_HEIGHT)
}
```

### Reactive Updates

Height recalculates on:
- Text input
- Text deletion
- Terminal resize
- Paste operations

### Constants

```typescript
const MIN_INPUT_HEIGHT = 1
const MAX_INPUT_HEIGHT = 10
```

## Edge Cases

| Case | Behavior |
|------|----------|
| Paste large text | Grow to max height, enable scroll |
| Terminal resize (narrower) | Recalculate wrapping, adjust height |
| Terminal resize (wider) | Recalculate wrapping, may shrink height |
| Only whitespace | Count as content, grow accordingly |
