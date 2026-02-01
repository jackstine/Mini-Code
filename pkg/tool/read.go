package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ReadTool implements the Tool interface for reading file contents.
// It supports optional line range specification for partial file reads.
type ReadTool struct{}

// readInput defines the expected input parameters for the read tool.
type readInput struct {
	Path      string `json:"path"`
	StartLine *int   `json:"start_line,omitempty"`
	EndLine   *int   `json:"end_line,omitempty"`
}

// readOutput defines the success response format.
type readOutput struct {
	Content string `json:"content"`
}

// readError defines the error response format.
type readError struct {
	Error string `json:"error"`
}

// NewReadTool creates a new ReadTool instance.
func NewReadTool() *ReadTool {
	return &ReadTool{}
}

// Name returns the tool identifier.
func (t *ReadTool) Name() string {
	return "read"
}

// Description returns a human-readable description of the tool.
func (t *ReadTool) Description() string {
	return "Read file contents, optionally specifying a line range"
}

// InputSchema returns the JSON Schema for the tool's input parameters.
func (t *ReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Absolute or relative file path"},
			"start_line": {"type": "integer", "description": "First line to read (1-indexed)"},
			"end_line": {"type": "integer", "description": "Last line to read (inclusive)"}
		},
		"required": ["path"]
	}`)
}

// Execute reads the specified file and returns its contents.
// Supports optional start_line and end_line parameters for partial reads.
func (t *ReadTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params readInput
	if err := json.Unmarshal(input, &params); err != nil {
		return formatReadError("invalid input: " + err.Error()), nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate path
	if params.Path == "" {
		return formatReadError("path is required"), nil
	}

	// Check if path exists and get file info
	info, err := os.Stat(params.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return formatReadError("file not found"), nil
		}
		if errors.Is(err, os.ErrPermission) {
			return formatReadError("permission denied"), nil
		}
		return formatReadError(err.Error()), nil
	}

	// Check if path is a directory
	if info.IsDir() {
		return formatReadError("path is a directory"), nil
	}

	// Validate line range parameters
	startLine := 1
	if params.StartLine != nil {
		startLine = *params.StartLine
		if startLine < 1 {
			return formatReadError("start_line must be at least 1"), nil
		}
	}

	if params.EndLine != nil {
		if params.StartLine != nil && *params.StartLine > *params.EndLine {
			return formatReadError("start_line cannot be greater than end_line"), nil
		}
		if *params.EndLine < 1 {
			return formatReadError("end_line must be at least 1"), nil
		}
	}

	// Read the file
	file, err := os.Open(params.Path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return formatReadError("permission denied"), nil
		}
		return formatReadError(err.Error()), nil
	}
	defer file.Close()

	// Read lines with optional range
	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++

		// Check context periodically
		if lineNum%1000 == 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}
		}

		// Skip lines before start_line
		if lineNum < startLine {
			continue
		}

		// Stop after end_line
		if params.EndLine != nil && lineNum > *params.EndLine {
			break
		}

		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return formatReadError("error reading file: " + err.Error()), nil
	}

	// Check if start_line exceeds file length
	// Only error if start_line was explicitly provided (not the default)
	if params.StartLine != nil && lineNum < startLine {
		return formatReadError(fmt.Sprintf("start_line %d exceeds file length of %d lines", startLine, lineNum)), nil
	}

	// Join lines and return
	content := strings.Join(lines, "\n")
	return formatReadSuccess(content), nil
}

// formatReadSuccess formats a successful read response.
func formatReadSuccess(content string) string {
	output := readOutput{Content: content}
	data, _ := json.Marshal(output)
	return string(data)
}

// formatReadError formats an error response.
func formatReadError(msg string) string {
	output := readError{Error: msg}
	data, _ := json.Marshal(output)
	return string(data)
}
