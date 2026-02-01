import { existsSync, mkdirSync, readFileSync, writeFileSync } from "fs"
import { homedir } from "os"
import { join } from "path"
import { initHistory, history } from "../stores/input"

const HARNESS_DIR = join(homedir(), ".harness")
const HISTORY_FILE = join(HARNESS_DIR, "prompt_history")
const MAX_HISTORY = 100

/**
 * Ensure the ~/.harness directory exists.
 */
function ensureDir() {
  if (!existsSync(HARNESS_DIR)) {
    mkdirSync(HARNESS_DIR, { recursive: true })
  }
}

/**
 * Load prompt history from disk.
 * Called on startup to restore previous session's history.
 */
export function loadHistory(): void {
  try {
    if (existsSync(HISTORY_FILE)) {
      const content = readFileSync(HISTORY_FILE, "utf-8")
      const lines = content.split("\n").filter(line => line.trim())
      initHistory(lines.slice(-MAX_HISTORY))
    }
  } catch (err) {
    console.error("Failed to load history:", err)
  }
}

/**
 * Save a single prompt to history file.
 * Appends to file and trims if over limit.
 */
export function savePrompt(prompt: string): void {
  if (!prompt.trim()) return

  try {
    ensureDir()

    // Read existing history
    let lines: string[] = []
    if (existsSync(HISTORY_FILE)) {
      const content = readFileSync(HISTORY_FILE, "utf-8")
      lines = content.split("\n").filter(line => line.trim())
    }

    // Add new prompt
    lines.push(prompt)

    // Trim to max (FIFO - remove oldest)
    if (lines.length > MAX_HISTORY) {
      lines = lines.slice(-MAX_HISTORY)
    }

    // Write back
    writeFileSync(HISTORY_FILE, lines.join("\n") + "\n")
  } catch (err) {
    console.error("Failed to save prompt to history:", err)
  }
}

/**
 * Save all current history to disk.
 * Called periodically or on shutdown.
 */
export function saveAllHistory(): void {
  try {
    ensureDir()
    const h = history()
    if (h.length > 0) {
      writeFileSync(HISTORY_FILE, h.join("\n") + "\n")
    }
  } catch (err) {
    console.error("Failed to save history:", err)
  }
}
