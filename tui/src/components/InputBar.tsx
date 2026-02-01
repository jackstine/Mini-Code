import type { Component } from "solid-js"
import {
  inputText,
  setInputText,
  addToHistory,
  clearInput,
} from "../stores/input"
import { submitPrompt } from "../lib/api"
import { savePrompt } from "../lib/history"
import { setThinking } from "../stores/status"
import { theme } from "../theme"

/**
 * InputBar provides text input for user prompts.
 * Enter submits, Ctrl+Enter adds newline.
 * History navigation is handled by App component.
 */
export const InputBar: Component = () => {
  const handleSubmit = (value: string) => {
    const trimmed = value.trim()
    if (!trimmed) return

    // Add to history and save to disk
    addToHistory(trimmed)
    savePrompt(trimmed)

    // Clear input
    clearInput()

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
      height={3}
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
