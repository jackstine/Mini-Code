/**
 * Text utility functions for the TUI.
 */

/**
 * Truncate text to a maximum number of lines.
 * Returns the truncated text and the number of lines removed.
 */
export function truncateLines(text: string, maxLines: number): { text: string; truncated: number } {
  const lines = text.split("\n")
  if (lines.length <= maxLines) {
    return { text, truncated: 0 }
  }
  return {
    text: lines.slice(0, maxLines).join("\n"),
    truncated: lines.length - maxLines
  }
}
