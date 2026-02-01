import { createSignal } from "solid-js"
import type { StatusEvent } from "../schemas/events"

export type StatusState = "idle" | "thinking" | "running_tool" | "error"

const [status, setStatus] = createSignal<StatusState>("idle")
const [statusMessage, setStatusMessage] = createSignal<string>("")
const [currentTool, setCurrentTool] = createSignal<string>("")

/**
 * Handle a status event from SSE.
 */
export function handleStatusEvent(event: StatusEvent) {
  setStatus(event.state)
  setStatusMessage(event.message ?? "")
}

/**
 * Set status to "running_tool" with the tool name.
 * Called when a tool_call event is received.
 */
export function setRunningTool(toolName: string) {
  setStatus("running_tool")
  setCurrentTool(toolName)
  setStatusMessage(`Running: ${toolName}...`)
}

/**
 * Set status to "thinking".
 * Called when waiting for agent response.
 */
export function setThinking() {
  setStatus("thinking")
  setCurrentTool("")
  setStatusMessage("Thinking...")
}

/**
 * Set status to "idle".
 * Called when agent finishes responding.
 */
export function setIdle() {
  setStatus("idle")
  setCurrentTool("")
  setStatusMessage("")
}

/**
 * Set status to "error" with a message.
 */
export function setError(message: string) {
  setStatus("error")
  setStatusMessage(`Error: ${message}`)
}

export { status, statusMessage, currentTool }
