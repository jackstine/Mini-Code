import type { Component } from "solid-js"
import { theme } from "../../theme"

interface Props {
  content: string
}

/**
 * TextPart displays agent text output.
 * TODO: Add markdown rendering with SyntaxStyle when needed.
 */
export const TextPart: Component<Props> = (props) => {
  return (
    <box flexDirection="column" marginBottom={1}>
      <text content={props.content} fg={theme.colors.text} />
    </box>
  )
}
