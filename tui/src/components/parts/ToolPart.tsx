import type { Component } from "solid-js"
import { Show } from "solid-js"
import { theme } from "../../theme"
import { truncateLines } from "../../lib/markdown"

interface Props {
  name: string
  input: Record<string, unknown>
  result: string | null
  isError: boolean
}

const MAX_LINES = 100

/**
 * ToolPart displays a tool invocation with its input and result.
 * Results are truncated to 100 lines max.
 * Errors are displayed in red.
 */
export const ToolPart: Component<Props> = (props) => {
  const truncated = () => {
    if (props.result === null) return null
    return truncateLines(props.result, MAX_LINES)
  }

  const inputStr = () => JSON.stringify(props.input, null, 2)

  return (
    <box
      flexDirection="column"
      marginBottom={1}
      border={true}
      borderColor={theme.colors.border}
      padding={1}
    >
      <text
        content={`Tool: ${props.name}`}
        fg={theme.colors.toolName}
        attributes={theme.attributes.bold}
      />
      <text content={`Input: ${inputStr()}`} fg={theme.colors.textDim} />
      <Show when={props.result !== null}>
        <text content="Result:" fg={theme.colors.textDim} />
        <text
          content={truncated()?.text ?? ""}
          fg={props.isError ? theme.colors.error : theme.colors.text}
        />
        <Show when={(truncated()?.truncated ?? 0) > 0}>
          <text
            content={`... (${truncated()?.truncated} more lines)`}
            fg={theme.colors.textDim}
          />
        </Show>
      </Show>
      <Show when={props.result === null}>
        <text content="Running..." fg={theme.colors.status} />
      </Show>
    </box>
  )
}
