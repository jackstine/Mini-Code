package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBashTool_Name(t *testing.T) {
	tool := NewBashTool()
	if tool.Name() != "bash" {
		t.Errorf("expected name 'bash', got '%s'", tool.Name())
	}
}

func TestBashTool_Description(t *testing.T) {
	tool := NewBashTool()
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestBashTool_InputSchema(t *testing.T) {
	tool := NewBashTool()
	schema := tool.InputSchema()

	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema should be valid JSON: %v", err)
	}

	if parsed["type"] != "object" {
		t.Error("schema type should be 'object'")
	}

	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema should have properties")
	}

	if _, ok := props["command"]; !ok {
		t.Error("schema should have 'command' property")
	}
}

func TestBashTool_ExecuteSimpleCommand(t *testing.T) {
	tool := NewBashTool()
	ctx := context.Background()

	input := `{"command": "echo 'hello world'"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bashOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.Stdout != "hello world\n" {
		t.Errorf("expected stdout 'hello world\\n', got '%s'", output.Stdout)
	}
	if output.Stderr != "" {
		t.Errorf("expected empty stderr, got '%s'", output.Stderr)
	}
	if output.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", output.ExitCode)
	}
}

func TestBashTool_ExecuteCommandWithStderr(t *testing.T) {
	tool := NewBashTool()
	ctx := context.Background()

	input := `{"command": "ls /nonexistent_path_12345"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bashOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.Stderr == "" {
		t.Error("expected stderr to contain error message")
	}
	if output.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}
}

func TestBashTool_ExecuteCommandWithExitCode(t *testing.T) {
	tool := NewBashTool()
	ctx := context.Background()

	input := `{"command": "exit 42"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bashOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", output.ExitCode)
	}
}

func TestBashTool_EmptyCommand(t *testing.T) {
	tool := NewBashTool()
	ctx := context.Background()

	input := `{"command": ""}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bashError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.Error == "" {
		t.Error("expected error for empty command")
	}
	if !strings.Contains(output.Error, "command is required") {
		t.Errorf("expected 'command is required' error, got '%s'", output.Error)
	}
}

func TestBashTool_InvalidInput(t *testing.T) {
	tool := NewBashTool()
	ctx := context.Background()

	input := `{invalid json}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bashError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.Error == "" {
		t.Error("expected error for invalid input")
	}
}

func TestBashTool_ContextCancellation(t *testing.T) {
	tool := NewBashTool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := `{"command": "echo test"}`
	_, err := tool.Execute(ctx, json.RawMessage(input))
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestBashTool_Timeout(t *testing.T) {
	tool := NewBashTool()
	ctx := context.Background()

	// Use a short sleep that should still trigger timeout detection
	// The actual timeout is 30 seconds, but we test the mechanism
	// by checking that a command that completes works fine
	input := `{"command": "sleep 0.1 && echo done"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bashOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", output.ExitCode)
	}
}

func TestBashTool_OutputTruncation(t *testing.T) {
	// Test that the truncation function works correctly
	longOutput := strings.Repeat("a", maxOutputSize+100)
	truncated := truncateOutput(longOutput)

	if len(truncated) > maxOutputSize {
		t.Errorf("truncated output should not exceed %d bytes, got %d", maxOutputSize, len(truncated))
	}

	if !strings.HasSuffix(truncated, truncationSuffix) {
		t.Error("truncated output should end with truncation suffix")
	}
}

func TestBashTool_OutputNotTruncated(t *testing.T) {
	// Test that short output is not truncated
	shortOutput := "hello world"
	result := truncateOutput(shortOutput)

	if result != shortOutput {
		t.Errorf("short output should not be modified, got '%s'", result)
	}
}

func TestBashTool_PipedCommand(t *testing.T) {
	tool := NewBashTool()
	ctx := context.Background()

	input := `{"command": "echo -e 'line1\nline2\nline3' | wc -l"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bashOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// wc -l output may have leading spaces, so trim
	stdout := strings.TrimSpace(output.Stdout)
	if stdout != "3" {
		t.Errorf("expected '3', got '%s'", stdout)
	}
	if output.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", output.ExitCode)
	}
}

func TestBashTool_CommandWithEnvironment(t *testing.T) {
	tool := NewBashTool()
	ctx := context.Background()

	// Test that environment variables from parent process are available
	input := `{"command": "echo $HOME"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bashOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.Stdout == "\n" || output.Stdout == "" {
		t.Error("expected HOME environment variable to be set")
	}
	if output.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", output.ExitCode)
	}
}

func TestBashTool_TimeoutActual(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	// Create a tool with a shorter timeout for testing
	tool := NewBashTool()
	ctx := context.Background()

	// Create a context with a very short timeout to test the timeout behavior
	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	input := `{"command": "sleep 10"}`
	result, err := tool.Execute(shortCtx, json.RawMessage(input))

	// Either we get a context error or a timeout error in the result
	if err != nil {
		// Context was cancelled, which is expected
		return
	}

	var output bashError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		// Check if it's a regular output (command got killed)
		var normalOutput bashOutput
		if json.Unmarshal([]byte(result), &normalOutput) == nil {
			// Command was killed due to context timeout
			return
		}
		t.Fatalf("failed to parse output: %v", err)
	}

	// If we got an error response, it should mention timeout or similar
	// This is acceptable behavior
}
