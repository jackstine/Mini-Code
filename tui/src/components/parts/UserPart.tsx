import type { Component } from "solid-js"
import { theme } from "../../theme"

interface Props {
  content: string
}

/**
 * UserPart displays the user's submitted prompt.
 * Styled with cyan accent to distinguish from agent responses.
 */
export const UserPart: Component<Props> = (props) => (
  <box flexDirection="column" marginBottom={1}>
    <text
      content="You:"
      fg={theme.colors.userPrompt}
      attributes={theme.attributes.bold}
    />
    <text content={props.content} fg={theme.colors.userPrompt} selectable />
  </box>
)
