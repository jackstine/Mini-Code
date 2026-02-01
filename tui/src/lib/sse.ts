import { EventSchema, type Event } from "../schemas/events"

type EventCallback = (event: Event) => void

export interface SSEClient {
  disconnect: () => void
}

/**
 * Creates an SSE client that connects to the harness server and
 * emits parsed events. Automatically reconnects on disconnect.
 *
 * Uses native fetch with streaming instead of EventSource (which
 * is not available in Bun/Node.js runtime).
 */
export function createSSEClient(baseUrl: string, onEvent: EventCallback): SSEClient {
  let abortController: AbortController | null = null
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  let isDisconnecting = false

  async function connect() {
    if (isDisconnecting) return

    abortController = new AbortController()

    try {
      const response = await fetch(`${baseUrl}/events`, {
        signal: abortController.signal,
        headers: {
          'Accept': 'text/event-stream',
          'Cache-Control': 'no-cache',
        },
      })

      if (!response.ok) {
        throw new Error(`SSE connection failed: ${response.status}`)
      }

      if (!response.body) {
        throw new Error('No response body')
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()

        if (done) {
          break
        }

        buffer += decoder.decode(value, { stream: true })

        // Process complete SSE messages (ending with \n\n)
        const messages = buffer.split('\n\n')
        buffer = messages.pop() || '' // Keep incomplete message in buffer

        for (const message of messages) {
          if (!message.trim()) continue

          // Skip heartbeat comments
          if (message.startsWith(':')) continue

          // Parse SSE data lines
          const lines = message.split('\n')
          for (const line of lines) {
            if (line.startsWith('data: ')) {
              const jsonData = line.slice(6) // Remove 'data: ' prefix
              try {
                const data = JSON.parse(jsonData)
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
          }
        }
      }
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        // Connection was intentionally aborted
        return
      }
      console.error("SSE connection error:", err)
    }

    // Auto-reconnect after 1 second if not disconnecting
    if (!isDisconnecting) {
      reconnectTimeout = setTimeout(connect, 1000)
    }
  }

  function disconnect() {
    isDisconnecting = true
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
    if (abortController) {
      abortController.abort()
      abortController = null
    }
  }

  // Start connection
  connect()

  return { disconnect }
}
