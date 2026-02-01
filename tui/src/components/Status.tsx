import type { Component } from "solid-js"
import { Show } from "solid-js"
import { status, statusMessage } from "../stores/status"
import { theme } from "../theme"

/**
 * Status displays the current agent state.
 * Only shown when agent is active (not idle).
 */
export const Status: Component = () => {
  const isVisible = () => status() !== "idle"

  return (
    <Show when={isVisible()}>
      <box width="100%" height={1} paddingLeft={1}>
        <text
          content={statusMessage()}
          fg={status() === "error" ? theme.colors.error : theme.colors.status}
          attributes={theme.attributes.bold}
        />
      </box>
    </Show>
  )
}
