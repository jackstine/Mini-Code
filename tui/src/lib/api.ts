const BASE_URL = "http://localhost:8080"

/**
 * Submit a prompt to the harness server.
 * The response will be streamed via SSE events.
 */
export async function submitPrompt(content: string): Promise<void> {
  const response = await fetch(`${BASE_URL}/prompt`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  })
  if (!response.ok) {
    throw new Error(`Failed to submit prompt: ${response.statusText}`)
  }
}

/**
 * Cancel the currently running agent.
 * Safe to call when no agent is running (no-op on server).
 */
export async function cancelAgent(): Promise<void> {
  const response = await fetch(`${BASE_URL}/cancel`, {
    method: "POST",
  })
  if (!response.ok) {
    throw new Error(`Failed to cancel: ${response.statusText}`)
  }
}
