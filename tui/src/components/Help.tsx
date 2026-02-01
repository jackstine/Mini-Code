import type { Component } from "solid-js"
import { Show } from "solid-js"
import { useTerminalDimensions } from "@opentui/solid"
import { theme } from "../theme"

interface Props {
  visible: boolean
  onClose: () => void
}

/**
 * Help displays a modal overlay with all keybindings.
 * Dismisses on any keypress.
 */
export const Help: Component<Props> = (props) => {
  const dimensions = useTerminalDimensions()
  const width = 50
  const height = 16

  const left = () => Math.floor((dimensions().width - width) / 2)
  const top = () => Math.floor((dimensions().height - height) / 2)

  const keybindings = [
    ["Enter", "Submit prompt"],
    ["Ctrl+Enter", "Insert newline"],
    ["Ctrl+C", "Cancel / Exit"],
    ["Up/Down", "Prompt history"],
    ["PageUp/Down", "Scroll conversation"],
    ["Home/End", "Scroll to top/bottom"],
    ["Ctrl+U", "Clear input"],
    ["? or F1", "Toggle help"],
    ["Esc", "Close help"],
  ]

  return (
    <Show when={props.visible}>
      <box
        position="absolute"
        left={left()}
        top={top()}
        width={width}
        height={height}
        border={true}
        borderColor={theme.colors.toolName}
        backgroundColor={theme.colors.background}
        padding={1}
        flexDirection="column"
      >
        <text
          content="Help - Keybindings"
          fg={theme.colors.toolName}
          attributes={theme.attributes.bold}
        />
        <text content="" />
        {keybindings.map(([key, action]) => (
          <text
            content={`${key.padEnd(16)}${action}`}
            fg={theme.colors.text}
          />
        ))}
        <text content="" />
        <text content="Press any key to close" fg={theme.colors.textDim} />
      </box>
    </Show>
  )
}
