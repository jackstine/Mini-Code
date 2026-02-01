package harness

import "encoding/json"

// EventHandler defines the interface for receiving events from the Harness agent loop.
// Implementations can use this to stream events to clients (e.g., via SSE).
// A nil EventHandler is valid - the harness will operate silently.
type EventHandler interface {
	// OnText is called when the agent produces a text response.
	// text contains the complete text content of a content block.
	OnText(text string)

	// OnToolCall is called when the agent requests a tool execution.
	// id is the unique identifier for this tool use.
	// name is the name of the tool being called.
	// input is the JSON-encoded input parameters.
	OnToolCall(id string, name string, input json.RawMessage)

	// OnToolResult is called when a tool execution completes.
	// id matches the id from the corresponding OnToolCall.
	// result is the tool's output (or error message if isError is true).
	// isError indicates whether the result represents an error.
	OnToolResult(id string, result string, isError bool)
}
