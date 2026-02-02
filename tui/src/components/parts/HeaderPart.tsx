import type { Component } from "solid-js"
import { theme } from "../../theme"

/**
 * HeaderPart displays the application welcome banner.
 * Renders the header content in blue with preserved whitespace.
 * Displayed once at the top of the conversation on startup.
 */
export const HeaderPart: Component<{ content: string }> = (props) => {
  return (
    <box marginBottom={1}>
      <text content={props.content} fg={theme.colors.header} />
    </box>
  )
}
