import type { Component } from "solid-js"
import { createMemo } from "solid-js"
import { useTerminalDimensions } from "@opentui/solid"
import {
  inputText,
  setInputText,
  addToHistory,
  clearInput,
} from "../stores/input"
import { submitPrompt } from "../lib/api"
import { savePrompt } from "../lib/history"
import { setThinking } from "../stores/status"
import { scrollToBottom } from "../stores/scroll"
import { theme } from "../theme"

const MIN_INPUT_HEIGHT = 1
const MAX_INPUT_HEIGHT = 10

/**
 * Calculate the number of visual lines needed to display text.
 * Accounts for explicit newlines and line wrapping at terminal width.
 */
function calculateInputHeight(content: string, width: number): number {
  if (!content) return MIN_INPUT_HEIGHT

  // Account for border (2 chars) and some padding
  const effectiveWidth = Math.max(width - 4, 10)

  // Split by explicit newlines
  const lines = content.split("\n")
  let totalLines = 0

  for (const line of lines) {
    if (line.length === 0) {
      // Empty line still counts as 1
      totalLines += 1
    } else {
      // Calculate wrapped lines for this line
      const wrappedLines = Math.ceil(line.length / effectiveWidth)
      totalLines += wrappedLines
    }
  }

  return Math.min(Math.max(totalLines, MIN_INPUT_HEIGHT), MAX_INPUT_HEIGHT)
}

/**
 * InputBar provides text input for user prompts.
 * Enter submits, Ctrl+Enter adds newline.
 * History navigation is handled by App component.
 * Dynamically grows from 1-10 lines based on content.
 */
export const InputBar: Component = () => {
  const dimensions = useTerminalDimensions()

  // Calculate height based on content and terminal width
  const inputHeight = createMemo(() => {
    const width = dimensions().width
    return calculateInputHeight(inputText(), width)
  })

  const handleSubmit = (value: string) => {
    const trimmed = value.trim()
    if (!trimmed) return

    // Add to history and save to disk
    addToHistory(trimmed)
    savePrompt(trimmed)

    // Clear input
    clearInput()

    // Immediately scroll to bottom when user sends message
    scrollToBottom()

    // Set status to thinking
    setThinking()

    // Submit to server (fire and forget, errors logged)
    submitPrompt(trimmed).catch(err => {
      console.error("Failed to submit prompt:", err)
    })
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const onSubmitHandler = handleSubmit as any

  return (
    <box
      width="100%"
      height={inputHeight() + 2}
      border={true}
      borderColor={theme.colors.border}
    >
      <input
        placeholder="Enter prompt..."
        value={inputText()}
        onInput={setInputText}
        onSubmit={onSubmitHandler}
        focused={true}
        width="100%"
        backgroundColor={theme.colors.background}
        textColor={theme.colors.text}
        focusedBackgroundColor={theme.colors.background}
        focusedTextColor={theme.colors.userPrompt}
      />
    </box>
  )
}
