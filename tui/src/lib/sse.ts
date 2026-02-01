import { EventSchema, type Event } from "../schemas/events"

type EventCallback = (event: Event) => void

export interface SSEClient {
  disconnect: () => void
}

/**
 * Creates an SSE client that connects to the harness server and
 * emits parsed events. Automatically reconnects on disconnect.
 */
export function createSSEClient(baseUrl: string, onEvent: EventCallback): SSEClient {
  let eventSource: EventSource | null = null
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  let isDisconnecting = false

  function connect() {
    if (isDisconnecting) return

    eventSource = new EventSource(`${baseUrl}/events`)

    eventSource.onopen = () => {
      console.log("SSE connected")
    }

    eventSource.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        const result = EventSchema.safeParse(data)
        if (result.success) {
          onEvent(result.data)
        } else {
          console.error("Invalid event schema:", result.error.format())
        }
      } catch (err) {
        console.error("Failed to parse SSE event:", err)
      }
    }

    eventSource.onerror = () => {
      eventSource?.close()
      eventSource = null

      if (!isDisconnecting) {
        // Auto-reconnect after 1 second
        reconnectTimeout = setTimeout(connect, 1000)
      }
    }
  }

  function disconnect() {
    isDisconnecting = true
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }

  // Start connection
  connect()

  return { disconnect }
}
