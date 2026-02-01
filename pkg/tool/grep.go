package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// GrepTool implements the Tool interface for searching patterns in files.
// It uses the system's grep command with Basic Regular Expressions (BRE).
type GrepTool struct{}

// grepInput defines the expected input parameters for the grep tool.
type grepInput struct {
	Pattern   string `json:"pattern"`
	Path      string `json:"path"`
	Recursive *bool  `json:"recursive,omitempty"`
}

// grepOutput defines the success response format.
type grepOutput struct {
	Matches string `json:"matches"`
}

// grepError defines the error response format.
type grepError struct {
	Error string `json:"error"`
}

// NewGrepTool creates a new GrepTool instance.
func NewGrepTool() *GrepTool {
	return &GrepTool{}
}

// Name returns the tool identifier.
func (t *GrepTool) Name() string {
	return "grep"
}

// Description returns a human-readable description of the tool.
func (t *GrepTool) Description() string {
	return "Search for patterns in files or directories"
}

// InputSchema returns the JSON Schema for the tool's input parameters.
func (t *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Search pattern (BRE regex)"},
			"path": {"type": "string", "description": "File or directory path"},
			"recursive": {"type": "boolean", "description": "Search recursively (default: false)"}
		},
		"required": ["pattern", "path"]
	}`)
}

// Execute searches for the pattern in the specified path.
func (t *GrepTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params grepInput
	if err := json.Unmarshal(input, &params); err != nil {
		return formatGrepError("invalid input: " + err.Error()), nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate parameters
	if params.Pattern == "" {
		return formatGrepError("pattern is required"), nil
	}
	if params.Path == "" {
		return formatGrepError("path is required"), nil
	}

	// Check if path exists
	_, err := os.Stat(params.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return formatGrepError("path not found"), nil
		}
		if errors.Is(err, os.ErrPermission) {
			return formatGrepError("permission denied"), nil
		}
		return formatGrepError(err.Error()), nil
	}

	// Build grep command arguments
	// -n: show line numbers
	args := []string{"-n"}

	// Add recursive flag if requested
	if params.Recursive != nil && *params.Recursive {
		args = append(args, "-r")
	}

	// Add pattern and path
	args = append(args, params.Pattern, params.Path)

	// Execute grep command
	cmd := exec.CommandContext(ctx, "/usr/bin/grep", args...)
	output, err := cmd.Output()

	// Handle errors
	if err != nil {
		// Check for context cancellation
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// grep returns exit code 1 when no matches are found
		// This is not an error condition for us
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Exit code 1 means no matches - return success with empty string
			if exitErr.ExitCode() == 1 {
				return formatGrepSuccess(""), nil
			}

			// Exit code 2 typically means error (invalid regex, etc.)
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(stderr, "Invalid regular expression") ||
				strings.Contains(stderr, "invalid") ||
				strings.Contains(stderr, "illegal") {
				return formatGrepError("invalid pattern: " + stderr), nil
			}
			if strings.Contains(stderr, "Permission denied") {
				return formatGrepError("permission denied"), nil
			}
			if stderr != "" {
				return formatGrepError(stderr), nil
			}
			return formatGrepError("grep failed with exit code " + string(rune('0'+exitErr.ExitCode()))), nil
		}
		return formatGrepError("failed to execute grep: " + err.Error()), nil
	}

	// Return successful matches
	return formatGrepSuccess(strings.TrimSuffix(string(output), "\n")), nil
}

// formatGrepSuccess formats a successful grep response.
func formatGrepSuccess(matches string) string {
	output := grepOutput{Matches: matches}
	data, _ := json.Marshal(output)
	return string(data)
}

// formatGrepError formats an error response.
func formatGrepError(msg string) string {
	output := grepError{Error: msg}
	data, _ := json.Marshal(output)
	return string(data)
}
