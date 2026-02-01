package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// ListDirTool implements the Tool interface for listing directory contents.
// It uses the system's ls command to provide detailed file information.
type ListDirTool struct{}

// listDirInput defines the expected input parameters for the list_dir tool.
type listDirInput struct {
	Path string `json:"path"`
}

// listDirOutput defines the success response format.
type listDirOutput struct {
	Entries string `json:"entries"`
}

// listDirError defines the error response format.
type listDirError struct {
	Error string `json:"error"`
}

// NewListDirTool creates a new ListDirTool instance.
func NewListDirTool() *ListDirTool {
	return &ListDirTool{}
}

// Name returns the tool identifier.
func (t *ListDirTool) Name() string {
	return "list_dir"
}

// Description returns a human-readable description of the tool.
func (t *ListDirTool) Description() string {
	return "List directory contents with detailed metadata"
}

// InputSchema returns the JSON Schema for the tool's input parameters.
func (t *ListDirTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path to list"}
		},
		"required": ["path"]
	}`)
}

// Execute lists the contents of the specified directory using ls -al.
func (t *ListDirTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params listDirInput
	if err := json.Unmarshal(input, &params); err != nil {
		return formatListDirError("invalid input: " + err.Error()), nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate path
	if params.Path == "" {
		return formatListDirError("path is required"), nil
	}

	// Check if path exists and get file info
	info, err := os.Stat(params.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return formatListDirError("path not found"), nil
		}
		if errors.Is(err, os.ErrPermission) {
			return formatListDirError("permission denied"), nil
		}
		return formatListDirError(err.Error()), nil
	}

	// Check if path is a directory
	if !info.IsDir() {
		return formatListDirError("not a directory"), nil
	}

	// Execute ls -al command
	cmd := exec.CommandContext(ctx, "ls", "-al", params.Path)
	output, err := cmd.Output()
	if err != nil {
		// Check for context cancellation
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// Check if it's an exit error with stderr
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(stderr, "Permission denied") {
				return formatListDirError("permission denied"), nil
			}
			if stderr != "" {
				return formatListDirError(stderr), nil
			}
		}
		return formatListDirError("failed to list directory: " + err.Error()), nil
	}

	// Return the raw ls output
	return formatListDirSuccess(strings.TrimSuffix(string(output), "\n")), nil
}

// formatListDirSuccess formats a successful list_dir response.
func formatListDirSuccess(entries string) string {
	output := listDirOutput{Entries: entries}
	data, _ := json.Marshal(output)
	return string(data)
}

// formatListDirError formats an error response.
func formatListDirError(msg string) string {
	output := listDirError{Error: msg}
	data, _ := json.Marshal(output)
	return string(data)
}
