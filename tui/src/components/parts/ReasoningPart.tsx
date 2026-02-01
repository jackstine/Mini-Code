import type { Component } from "solid-js"
import { theme } from "../../theme"

interface Props {
  content: string
}

/**
 * ReasoningPart displays agent thinking/reasoning blocks.
 * Styled with dimmed gray and italic to distinguish from main output.
 */
export const ReasoningPart: Component<Props> = (props) => (
  <box flexDirection="column" marginBottom={1}>
    <text
      content="Thinking:"
      fg={theme.colors.reasoning}
      attributes={theme.attributes.dim}
    />
    <text
      content={props.content}
      fg={theme.colors.reasoning}
      attributes={theme.attributes.dim | theme.attributes.italic}
      selectable
    />
  </box>
)
