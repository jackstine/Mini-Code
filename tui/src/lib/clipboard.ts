/**
 * Clipboard utilities for the TUI.
 * Uses OSC 52 escape sequence to copy text to system clipboard.
 *
 * OSC 52 is a terminal escape sequence supported by modern terminals
 * (Alacritty, Ghostty, Kitty, WezTerm, iTerm2, etc.) that allows
 * copying text to the clipboard through escape sequences.
 */

/**
 * Copy text to the system clipboard using OSC 52 escape sequence.
 *
 * Format: OSC 52 ; Pc ; [base64 encoded string] ST
 * Where:
 * - OSC = \x1b] (escape bracket)
 * - Pc = "c" for clipboard
 * - ST = \x07 (bell) as string terminator
 *
 * @param text The text to copy to clipboard
 */
export function copyToClipboard(text: string): void {
  if (!text) return

  // Encode text as base64
  const base64 = Buffer.from(text).toString("base64")

  // Send OSC 52 escape sequence
  // \x1b]52;c; = OSC 52 clipboard set
  // \x07 = BEL (string terminator)
  process.stdout.write(`\x1b]52;c;${base64}\x07`)
}
