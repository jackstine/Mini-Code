# TUI Development Notes

## OpenTUI Scrollbox Issues

### scrollTo with Infinity Causes Left-Edge Clipping

**Problem:** Using `scrollbox.scrollTo({ y: Infinity })` to scroll to the bottom causes the first character(s) of content to be clipped or hidden at the left edge of the scrollbox viewport.

**Root Cause:** OpenTUI's scrollbox has a rendering bug when handling `Infinity` in object notation. When the y-coordinate is set to Infinity, it corrupts or resets the internal viewport x-position calculation, causing the content area to shift left and clip at the edge.

**Solution:** Use a large finite number instead:
```typescript
// BAD - causes left-edge clipping
scrollbox.scrollTo({ y: Infinity })

// GOOD - scrolls to bottom without clipping
scrollbox.scrollTo(999999)
```

**Related Files:**
- `/tui/src/stores/scroll.ts` - scrollToBottom() implementation
- `/tui/src/components/Conversation.tsx` - scrollbox usage

**Note:** The scrollbox component has `stickyScroll={true}` which auto-scrolls to bottom on new content, so manual scrollToBottom() calls are only needed for explicit user actions (like pressing the End key).
