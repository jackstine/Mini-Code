/**
 * Theme constants for the Harness TUI.
 * Dark mode with minimal color palette per specification.
 */
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
    header: "#3b82f6",         // Blue for header banner
  },
  attributes: {
    bold: 1,
    dim: 2,
    italic: 4,
    underline: 8,
  }
} as const

export type Theme = typeof theme
