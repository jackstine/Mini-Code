// Package tool defines the interface and utilities for implementing tools
// that can be used by the Harness AI agent.
package tool

import (
	"context"
	"encoding/json"
)

// Tool defines the interface that all tools must implement to be usable
// by the Harness agent. Tools are independent units that can be called
// by the AI to perform specific operations like reading files, listing
// directories, or searching with grep.
type Tool interface {
	// Name returns the unique identifier for this tool.
	// This name is used by the AI to invoke the tool.
	Name() string

	// Description returns a human-readable description of what the tool does.
	// This helps the AI understand when to use the tool.
	Description() string

	// InputSchema returns the JSON Schema defining the expected input parameters.
	// The schema is used for validation and to inform the AI of expected inputs.
	InputSchema() json.RawMessage

	// Execute runs the tool with the given input and returns the result.
	// The input is JSON that conforms to InputSchema.
	// Returns the tool output as a string (JSON formatted for structured output).
	// Returns an error if the tool execution fails.
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}
