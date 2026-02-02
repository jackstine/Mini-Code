# Scrolling and Focus Specification

## Overview

This document specifies how focus and scrolling interact in the TUI. Focus and scroll are managed independently — keyboard navigation scrolls content without changing focus state.

## Core Principle

**Focus never moves to the scrollbox.** The scrollbox is a passive display area. Focus remains on the input field or the current interactive element (dialog, autocomplete, etc.).

## Focus Management

### Focus Ownership

| Component | Can Receive Focus |
|-----------|-------------------|
| Input field | Yes |
| Dialogs | Yes |
| Autocomplete popover | Yes |
| Scrollbox (conversation) | No |
| Message content | No |

### Auto-Focus Behavior

The input field automatically receives focus when:

| Trigger | Behavior |
|---------|----------|
| Single character key (no modifiers) | Focus input, insert character |
| Application start | Focus input |
| Dialog closes | Restore previous focus (usually input) |
| Agent completes | Focus input |

The input field does **not** auto-focus when:

| Trigger | Behavior |
|---------|----------|
| PageUp/PageDown/Home/End | Scroll only, no focus change |
| Ctrl/Cmd + key | Execute shortcut, no focus change |
| Terminal is open | Terminal retains focus |

### Focus Protection

Elements can be protected from auto-focus using a data attribute:

```html
<element data-prevent-autofocus>
  <!-- Content here won't trigger auto-focus -->
</element>
```

Protected elements include:
- Terminal panel
- Active dialogs
- Text selection areas

## Scrolling Behavior

### Keyboard Scrolling

| Key | Action | Focus Change |
|-----|--------|--------------|
| PageUp | Scroll up 10 viewports | None |
| PageDown | Scroll down 10 viewports | None |
| Home | Scroll to top | None |
| End | Scroll to bottom | None |

All scroll keys operate on the conversation scrollbox without affecting input focus.

**Scroll Speed:** PageUp/PageDown scroll 10 viewports per keypress for fast navigation through long conversations.

### Auto-Scroll

The conversation auto-scrolls to show new content:

| State | Behavior |
|-------|----------|
| User at bottom | Auto-scroll enabled, new content scrolls into view |
| User scrolled up | Auto-scroll paused |
| User returns to bottom | Auto-scroll resumes |

**Bottom detection:** User is considered "at bottom" when scroll position is within a small threshold (e.g., 10 pixels) of the maximum scroll position.

### Message Submission Scroll Behavior

When the user submits a message:
1. Message is immediately added to the conversation
2. **Viewport automatically scrolls to bottom** to show the newly submitted message
3. This ensures the user always sees their message and the agent's response without manual scrolling

This behavior is **immediate** and happens synchronously with message submission, before the agent responds.

### Scroll Position Preservation

When content above the viewport changes (e.g., tool result expands):
- Current scroll position is preserved relative to visible content
- Use scroll anchoring to prevent jarring jumps

## Focus Restoration

When dialogs or popovers close, focus returns to the previously focused element.

### Implementation Pattern

```
1. Before opening overlay:
   - Save reference to currently focused element
   - Blur current element
   - Focus overlay

2. After closing overlay:
   - Check if saved element still exists
   - Check if saved element is not destroyed
   - Restore focus with small delay (1ms) for layout settling
```

### Validation Before Restore

Before restoring focus:
- Verify element exists in the render tree
- Verify element is not destroyed/unmounted
- If invalid, focus falls back to input field

## Selection and Scroll Interaction

### Text Selection in Scrollbox

When user selects text in the conversation:
- Selection does not move focus to scrollbox
- Input field remains focused (but inactive for typing)
- Scroll keys still work during selection
- Escape cancels selection and returns to normal state

### Mouse Interaction

| Action | Result |
|--------|--------|
| Click in scrollbox | Start text selection, no focus change |
| Click in input | Focus input |
| Scroll wheel in scrollbox | Scroll content, no focus change |

## Scroll and Focus Coordination

### Dialog with Scrollable Content

When a dialog contains a scrollable list (e.g., file picker):

```
┌─────────────────────────────────┐
│ Select File                     │
├─────────────────────────────────┤
│ > file1.txt    ← selected       │
│   file2.txt                     │
│   file3.txt                     │
│   ...                           │
└─────────────────────────────────┘
```

Navigation pattern:
1. Arrow keys change selection
2. Scroll automatically adjusts to keep selection visible
3. Focus stays on the dialog (not individual items)

### Keep Selection Visible

When selection changes via keyboard:

```
function moveTo(next) {
  setSelected(next)

  const target = findSelectedElement()
  const y = target.y - scroll.y

  // Selection below viewport
  if (y >= scroll.height) {
    scroll.scrollBy(y - scroll.height + 1)
  }

  // Selection above viewport
  if (y < 0) {
    scroll.scrollBy(y)
  }
}
```

## Edge Cases

### Rapid Key Presses

When user types quickly while scrolled up:
- First character focuses input and pauses auto-scroll
- Subsequent characters are typed normally
- Auto-scroll does not resume until user explicitly scrolls to bottom

### Focus During Agent Execution

While agent is running:
- User can scroll freely
- User can type in input (queued or disabled based on implementation)
- Focus remains on input unless user explicitly interacts with another element

### Window Resize

On terminal resize:
- Scroll position preserved relative to content
- Focus unchanged
- Text rewraps to new width
