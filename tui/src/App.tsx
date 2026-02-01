import { createSignal, onMount, onCleanup } from "solid-js"
import { useKeyboard, useRenderer } from "@opentui/solid"
import { Conversation } from "./components/Conversation"
import { InputBar } from "./components/InputBar"
import { Status } from "./components/Status"
import { Help } from "./components/Help"
import { createSSEClient } from "./lib/sse"
import { handleEvent } from "./stores/conversation"
import { handleStatusEvent, setRunningTool, setIdle, status } from "./stores/status"
import { cancelAgent } from "./lib/api"
import { loadHistory } from "./lib/history"
import { navigateHistoryUp, navigateHistoryDown, clearInput } from "./stores/input"
import { scrollPageUp, scrollPageDown, scrollToTop, scrollToBottom } from "./stores/scroll"
import { theme } from "./theme"
import type { Event } from "./schemas/events"

/**
 * Main App component.
 * Manages SSE connection, global keybindings, and layout.
 */
export const App = () => {
  const renderer = useRenderer()
  const [showHelp, setShowHelp] = createSignal(false)

  // Handle incoming events from SSE
  const onEvent = (event: Event) => {
    // Update conversation state
    handleEvent(event)

    // Update status based on event type
    switch (event.type) {
      case "status":
        handleStatusEvent(event)
        break
      case "tool_call":
        setRunningTool(event.name)
        break
      case "tool_result":
        // After tool result, wait for next event
        break
      case "text":
        // Text means agent is responding, set idle after
        // (Status will be updated by next status event or end of stream)
        break
    }
  }

  onMount(() => {
    // Load persisted history
    loadHistory()

    // Connect to SSE
    const sse = createSSEClient("http://localhost:8080", onEvent)

    onCleanup(() => {
      sse.disconnect()
    })
  })

  // Global keybindings
  useKeyboard((key) => {
    // Help toggle with ? or F1
    if (key.name === "?" || key.name === "f1") {
      setShowHelp(v => !v)
      return
    }

    // Close help on any key when open
    if (showHelp()) {
      setShowHelp(false)
      return
    }

    // Escape: Close help
    if (key.name === "escape") {
      setShowHelp(false)
      return
    }

    // Ctrl+C: Cancel running agent or exit if idle
    if (key.ctrl && key.name === "c") {
      if (status() !== "idle") {
        cancelAgent().catch(err => console.error("Failed to cancel:", err))
        setIdle()
      } else {
        renderer.destroy()
      }
      return
    }

    // History navigation (Up/Down arrows)
    if (key.name === "up") {
      navigateHistoryUp()
      return
    }

    if (key.name === "down") {
      navigateHistoryDown()
      return
    }

    // Ctrl+U: Clear input
    if (key.ctrl && key.name === "u") {
      clearInput()
      return
    }

    // PageUp: Scroll conversation up by one viewport
    if (key.name === "pageup") {
      scrollPageUp()
      return
    }

    // PageDown: Scroll conversation down by one viewport
    if (key.name === "pagedown") {
      scrollPageDown()
      return
    }

    // Home: Scroll to top of conversation
    if (key.name === "home") {
      scrollToTop()
      return
    }

    // End: Scroll to bottom of conversation
    if (key.name === "end") {
      scrollToBottom()
      return
    }
  })

  return (
    <box
      flexDirection="column"
      width="100%"
      height="100%"
      backgroundColor={theme.colors.background}
    >
      <Conversation />
      <Status />
      <InputBar />
      <Help visible={showHelp()} onClose={() => setShowHelp(false)} />
    </box>
  )
}
