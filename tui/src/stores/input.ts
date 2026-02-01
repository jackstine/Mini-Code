import { createSignal } from "solid-js"

const MAX_HISTORY = 100

const [inputText, setInputText] = createSignal("")
const [history, setHistory] = createSignal<string[]>([])
const [historyIndex, setHistoryIndex] = createSignal(-1)

// Temporary storage for current input when navigating history
let tempInput = ""

/**
 * Add a prompt to history after submission.
 * Maintains FIFO with max 100 entries.
 */
export function addToHistory(prompt: string) {
  if (!prompt.trim()) return

  setHistory(prev => {
    const newHistory = [...prev, prompt]
    if (newHistory.length > MAX_HISTORY) {
      newHistory.shift() // Remove oldest (FIFO)
    }
    return newHistory
  })
  setHistoryIndex(-1)
  tempInput = ""
}

/**
 * Navigate up in history (older prompts).
 * Saves current input when first navigating.
 */
export function navigateHistoryUp() {
  const h = history()
  const idx = historyIndex()

  if (h.length === 0) return

  if (idx === -1) {
    // Save current input before navigating
    tempInput = inputText()
  }

  if (idx < h.length - 1) {
    const newIdx = idx + 1
    setHistoryIndex(newIdx)
    setInputText(h[h.length - 1 - newIdx])
  }
}

/**
 * Navigate down in history (newer prompts).
 * Restores original input when reaching the end.
 */
export function navigateHistoryDown() {
  const idx = historyIndex()

  if (idx > 0) {
    const newIdx = idx - 1
    setHistoryIndex(newIdx)
    setInputText(history()[history().length - 1 - newIdx])
  } else if (idx === 0) {
    // Restore original input
    setHistoryIndex(-1)
    setInputText(tempInput)
    tempInput = ""
  }
}

/**
 * Clear the current input text.
 */
export function clearInput() {
  setInputText("")
  setHistoryIndex(-1)
  tempInput = ""
}

/**
 * Initialize history from persisted storage.
 */
export function initHistory(prompts: string[]) {
  setHistory(prompts.slice(-MAX_HISTORY))
}

export { inputText, setInputText, history }
