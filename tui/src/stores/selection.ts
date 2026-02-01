import { createSignal } from "solid-js"

/**
 * Selection store for managing text selection in the conversation.
 * Tracks selected text for copy operations.
 */

// Current selected text (empty string when no selection)
const [selectedText, setSelectedText] = createSignal<string>("")

/**
 * Update the current selection with new text.
 * Called by the useSelectionHandler hook when selection changes.
 */
export function updateSelection(text: string) {
  setSelectedText(text)
}

/**
 * Clear the current selection.
 * Called when Escape is pressed or selection is canceled.
 */
export function clearSelection() {
  setSelectedText("")
}

/**
 * Get the currently selected text.
 * Returns empty string if no selection.
 */
export function getSelectedText(): string {
  return selectedText()
}

/**
 * Check if there is an active selection.
 */
export function hasSelection(): boolean {
  return selectedText().length > 0
}

export { selectedText }
