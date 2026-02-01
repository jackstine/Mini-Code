import { createStore, produce } from "solid-js/store"
import type { Event } from "../schemas/events"

/**
 * Part represents a single item in the conversation history.
 * Parts are displayed in order and never removed during a session.
 */
export type Part =
  | { type: "user"; content: string; timestamp: number }
  | { type: "text"; content: string; timestamp: number }
  | { type: "tool"; id: string; name: string; input: unknown; result: string | null; isError: boolean; timestamp: number }
  | { type: "reasoning"; content: string; timestamp: number }

const [parts, setParts] = createStore<Part[]>([])

/**
 * Handle an incoming SSE event and update the conversation state.
 * Tool results update existing tool parts in-place by matching ID.
 */
export function handleEvent(event: Event) {
  switch (event.type) {
    case "user":
      setParts(produce(p => p.push({
        type: "user",
        content: event.content,
        timestamp: event.timestamp
      })))
      break

    case "text":
      setParts(produce(p => p.push({
        type: "text",
        content: event.content,
        timestamp: event.timestamp
      })))
      break

    case "tool_call":
      setParts(produce(p => p.push({
        type: "tool",
        id: event.id,
        name: event.name,
        input: event.input,
        result: null,
        isError: false,
        timestamp: event.timestamp
      })))
      break

    case "tool_result":
      // Update existing tool part with result by matching ID
      setParts(
        part => part.type === "tool" && part.id === event.id,
        produce((part) => {
          if (part.type === "tool") {
            part.result = event.result
            part.isError = event.isError
          }
        })
      )
      break

    case "reasoning":
      setParts(produce(p => p.push({
        type: "reasoning",
        content: event.content,
        timestamp: event.timestamp
      })))
      break

    // Status events are handled separately by the status store
    case "status":
      break
  }
}

/**
 * Clear all parts from the conversation.
 * Used when starting a new session.
 */
export function clearConversation() {
  setParts([])
}

export { parts }
