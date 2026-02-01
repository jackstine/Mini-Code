import { render } from "@opentui/solid"
import { App } from "./App"

/**
 * Entry point for the Harness TUI.
 * Renders the App component with OpenTUI.
 */
render(App, {
  targetFps: 30,
  exitOnCtrlC: false, // We handle Ctrl+C manually for cancel/exit
})
