package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

const (
	// bashTimeout is the maximum time allowed for command execution.
	bashTimeout = 30 * time.Second
	// maxOutputSize is the maximum size of stdout/stderr in bytes (1 MB).
	maxOutputSize = 1024 * 1024
	// truncationSuffix is appended when output exceeds maxOutputSize.
	truncationSuffix = "... (truncated)"
)

// BashTool implements the Tool interface for executing bash commands.
type BashTool struct{}

// bashInput defines the expected input parameters for the bash tool.
type bashInput struct {
	Command string `json:"command"`
}

// bashOutput defines the success response format.
type bashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// bashError defines the error response format.
type bashError struct {
	Error string `json:"error"`
}

// NewBashTool creates a new BashTool instance.
func NewBashTool() *BashTool {
	return &BashTool{}
}

// Name returns the tool identifier.
func (t *BashTool) Name() string {
	return "bash"
}

// Description returns a human-readable description of the tool.
func (t *BashTool) Description() string {
	return "Execute a bash command and return stdout/stderr"
}

// InputSchema returns the JSON Schema for the tool's input parameters.
func (t *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "The bash command to execute"}
		},
		"required": ["command"]
	}`)
}

// Execute runs the specified bash command and returns the output.
func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params bashInput
	if err := json.Unmarshal(input, &params); err != nil {
		return formatBashError("invalid input: " + err.Error()), nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate command
	if params.Command == "" {
		return formatBashError("command is required"), nil
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, bashTimeout)
	defer cancel()

	// Execute command using /bin/bash -c
	cmd := exec.CommandContext(cmdCtx, "/bin/bash", "-c", params.Command)

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	// Handle timeout
	if cmdCtx.Err() == context.DeadlineExceeded {
		return formatBashError("command timed out after 30 seconds"), nil
	}

	// Check if parent context was cancelled
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Process spawn failure or other error
			return formatBashError("failed to execute command: " + err.Error()), nil
		}
	}

	// Truncate output if necessary
	stdoutStr := truncateOutput(stdout.String())
	stderrStr := truncateOutput(stderr.String())

	return formatBashSuccess(stdoutStr, stderrStr, exitCode), nil
}

// truncateOutput truncates the output if it exceeds maxOutputSize.
func truncateOutput(output string) string {
	if len(output) > maxOutputSize {
		return output[:maxOutputSize-len(truncationSuffix)] + truncationSuffix
	}
	return output
}

// formatBashSuccess formats a successful command response.
func formatBashSuccess(stdout, stderr string, exitCode int) string {
	output := bashOutput{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
	data, _ := json.Marshal(output)
	return string(data)
}

// formatBashError formats an error response.
func formatBashError(msg string) string {
	output := bashError{Error: msg}
	data, _ := json.Marshal(output)
	return string(data)
}
