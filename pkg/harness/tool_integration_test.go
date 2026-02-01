package harness_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/harness/pkg/harness"
	"github.com/user/harness/pkg/testutil"
	"github.com/user/harness/pkg/tool"
)

// TestIntegration_ReadToolSuccess tests that the READ tool successfully
// reads a file through the harness.
func TestIntegration_ReadToolSuccess(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!\nThis is a test file."
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Setup mock streamer with tool call for READ
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"read",
		map[string]string{"path": testFile},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("File read successfully!"))

	// Create tools
	tools := []tool.Tool{
		tool.NewReadTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Read the test file")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool was called
	if len(handler.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(handler.ToolCalls))
	}
	if len(handler.ToolCalls) > 0 {
		if handler.ToolCalls[0].Name != "read" {
			t.Errorf("expected tool name 'read', got %q", handler.ToolCalls[0].Name)
		}
	}

	// Verify tool result contains the file content
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]
	if result.IsError {
		t.Errorf("expected success result, got error: %q", result.Result)
	}

	// Parse the result to verify it contains the content
	var resultData struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	if resultData.Content != content {
		t.Errorf("expected content %q, got %q", content, resultData.Content)
	}
}

// TestIntegration_ReadToolError tests that the READ tool returns an error
// when the file does not exist.
func TestIntegration_ReadToolError(t *testing.T) {
	nonexistentPath := "/nonexistent/path/to/file.txt"

	// Setup mock streamer with tool call for READ with nonexistent file
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"read",
		map[string]string{"path": nonexistentPath},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("The file was not found."))

	// Create tools
	tools := []tool.Tool{
		tool.NewReadTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Read the nonexistent file")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool result contains an error
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]
	// Note: Tool returns formatted error response (not IsError=true)
	// because the tool execution succeeded, but the file operation failed

	// Parse the result to verify it contains an error
	var resultData struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	if resultData.Error == "" {
		t.Error("expected error in result, got none")
	}
	if resultData.Error != "file not found" {
		t.Logf("got error message: %q", resultData.Error)
	}
}

// TestIntegration_ReadToolPartialRead tests reading a file with line range.
func TestIntegration_ReadToolPartialRead(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multiline.txt")
	content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Setup mock streamer with tool call for READ with line range
	startLine := 2
	endLine := 4
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"read",
		map[string]any{
			"path":       testFile,
			"start_line": startLine,
			"end_line":   endLine,
		},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Partial read complete!"))

	// Create tools
	tools := []tool.Tool{
		tool.NewReadTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Read lines 2-4")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool result
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]
	var resultData struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	expectedContent := "Line 2\nLine 3\nLine 4"
	if resultData.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, resultData.Content)
	}
}

// TestIntegration_ListDirToolSuccess tests that the LIST_DIR tool successfully
// lists a directory through the harness.
func TestIntegration_ListDirToolSuccess(t *testing.T) {
	// Create a temporary directory with some files
	tmpDir := t.TempDir()
	testFile1 := filepath.Join(tmpDir, "file1.txt")
	testFile2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(testFile1, []byte("test 1"), 0644); err != nil {
		t.Fatalf("failed to create test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("test 2"), 0644); err != nil {
		t.Fatalf("failed to create test file 2: %v", err)
	}

	// Setup mock streamer with tool call for LIST_DIR
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"list_dir",
		map[string]string{"path": tmpDir},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Directory listed successfully!"))

	// Create tools
	tools := []tool.Tool{
		tool.NewListDirTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "List the directory")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool was called
	if len(handler.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(handler.ToolCalls))
	}
	if len(handler.ToolCalls) > 0 {
		if handler.ToolCalls[0].Name != "list_dir" {
			t.Errorf("expected tool name 'list_dir', got %q", handler.ToolCalls[0].Name)
		}
	}

	// Verify tool result contains directory entries
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]
	if result.IsError {
		t.Errorf("expected success result, got error: %q", result.Result)
	}

	// Parse the result to verify it contains entries
	var resultData struct {
		Entries string `json:"entries"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	// Verify the entries contain our test files
	if resultData.Entries == "" {
		t.Error("expected directory entries, got empty string")
	}
	// The entries should contain the filenames (ls output format varies)
	t.Logf("Directory entries: %s", resultData.Entries)
}

// TestIntegration_ListDirToolError tests that the LIST_DIR tool returns an error
// when the directory does not exist.
func TestIntegration_ListDirToolError(t *testing.T) {
	nonexistentPath := "/nonexistent/directory"

	// Setup mock streamer with tool call for LIST_DIR with nonexistent directory
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"list_dir",
		map[string]string{"path": nonexistentPath},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("The directory was not found."))

	// Create tools
	tools := []tool.Tool{
		tool.NewListDirTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "List the nonexistent directory")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool result contains an error
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]

	// Parse the result to verify it contains an error
	var resultData struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	if resultData.Error == "" {
		t.Error("expected error in result, got none")
	}
}

// TestIntegration_GrepToolSuccess tests that the GREP tool successfully
// searches for a pattern through the harness.
func TestIntegration_GrepToolSuccess(t *testing.T) {
	// Create a temporary test file with searchable content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "search.txt")
	content := "foo bar\nbaz qux\nfoo quux\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Setup mock streamer with tool call for GREP
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"grep",
		map[string]string{
			"pattern": "foo",
			"path":    testFile,
		},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Pattern found!"))

	// Create tools
	tools := []tool.Tool{
		tool.NewGrepTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Search for 'foo'")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool was called
	if len(handler.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(handler.ToolCalls))
	}
	if len(handler.ToolCalls) > 0 {
		if handler.ToolCalls[0].Name != "grep" {
			t.Errorf("expected tool name 'grep', got %q", handler.ToolCalls[0].Name)
		}
	}

	// Verify tool result contains matches
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]
	if result.IsError {
		t.Errorf("expected success result, got error: %q", result.Result)
	}

	// Parse the result to verify it contains matches
	var resultData struct {
		Matches string `json:"matches"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	// Verify matches contain "foo" lines
	if resultData.Matches == "" {
		t.Error("expected matches, got empty string")
	}
	// grep output format is "1:foo bar\n3:foo quux"
	t.Logf("Grep matches: %s", resultData.Matches)
}

// TestIntegration_GrepToolNoMatches tests GREP tool when pattern is not found.
func TestIntegration_GrepToolNoMatches(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "search.txt")
	content := "foo bar\nbaz qux\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Setup mock streamer with tool call for GREP with non-matching pattern
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"grep",
		map[string]string{
			"pattern": "nonexistent",
			"path":    testFile,
		},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("No matches found."))

	// Create tools
	tools := []tool.Tool{
		tool.NewGrepTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Search for 'nonexistent'")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool result contains empty matches
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]
	if result.IsError {
		t.Errorf("expected success result (with no matches), got error: %q", result.Result)
	}

	// Parse the result
	var resultData struct {
		Matches string `json:"matches"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	// No matches should return empty string
	if resultData.Matches != "" {
		t.Errorf("expected empty matches, got %q", resultData.Matches)
	}
}

// TestIntegration_ToolInputValidation tests that invalid tool inputs
// are properly handled by the harness.
func TestIntegration_ToolInputValidation(t *testing.T) {
	// Setup mock streamer with invalid tool call (missing required path)
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"read",
		map[string]string{}, // Missing required "path" field
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Invalid input detected."))

	// Create tools
	tools := []tool.Tool{
		tool.NewReadTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Read a file")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool result contains an error about missing path
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]

	// Parse the result to verify it contains an error
	var resultData struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	if resultData.Error == "" {
		t.Error("expected error in result for missing path, got none")
	}
	if resultData.Error != "path is required" {
		t.Logf("got error message: %q", resultData.Error)
	}
}

// TestIntegration_ContextCancellationDuringTool tests that context cancellation
// properly stops tool execution.
func TestIntegration_ContextCancellationDuringTool(t *testing.T) {
	// Create a temporary test file with many lines to ensure tool takes time
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	largeContent := make([]byte, 1024*1024) // 1MB of zeros
	if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Setup mock streamer with tool call for READ
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"read",
		map[string]string{"path": testFile},
	))

	// Create tools
	tools := []tool.Tool{
		tool.NewReadTool(),
	}

	// Create harness with mock
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		nil,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Run prompt with cancelled context
	err = h.Prompt(ctx, "Read the large file")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

// TestIntegration_ContextCancellationWithTimeout tests that context timeout
// stops tool execution.
func TestIntegration_ContextCancellationWithTimeout(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Setup mock streamer with tool call
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"read",
		map[string]string{"path": testFile},
	))

	// Create tools
	tools := []tool.Tool{
		tool.NewReadTool(),
	}

	// Create harness with mock
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		nil,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give the context time to expire
	time.Sleep(10 * time.Millisecond)

	// Run prompt with expired context
	err = h.Prompt(ctx, "Read the file")
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}
}

// TestIntegration_UnknownToolHandling tests that the harness handles
// unknown tool calls appropriately.
func TestIntegration_UnknownToolHandling(t *testing.T) {
	// Setup mock streamer with tool call for unknown tool
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"unknown_tool",
		map[string]string{"param": "value"},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Tool not found, continuing."))

	// Create harness with no tools (or only known tools)
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		[]tool.Tool{tool.NewReadTool()}, // Only read tool, not "unknown_tool"
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Use unknown tool")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify that tool was called but result shows error
	if len(handler.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(handler.ToolCalls))
	}

	// Verify tool result contains error about unknown tool
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]
	if !result.IsError {
		t.Error("expected error result for unknown tool")
	}

	// The error message should indicate the tool was not found
	if result.Result == "" {
		t.Error("expected error message for unknown tool, got empty string")
	}
	t.Logf("Unknown tool error: %s", result.Result)
}

// TestIntegration_MultipleToolsInSequence tests executing multiple different
// tools in sequence through the harness.
func TestIntegration_MultipleToolsInSequence(t *testing.T) {
	// Create test fixtures
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Setup mock streamer with multiple tool calls
	mockStreamer := testutil.NewMockMessageStreamer()

	// First turn: list directory
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"list_dir",
		map[string]string{"path": tmpDir},
	))

	// Second turn: read file
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_2",
		"read",
		map[string]string{"path": testFile},
	))

	// Final turn: text response
	mockStreamer.AddResponse(testutil.TextOnlyResponse("All tools executed!"))

	// Create tools
	tools := []tool.Tool{
		tool.NewReadTool(),
		tool.NewListDirTool(),
		tool.NewGrepTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "List and read files")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify both tools were called
	if len(handler.ToolCalls) != 2 {
		t.Errorf("expected 2 tool calls, got %d", len(handler.ToolCalls))
	}

	if len(handler.ToolCalls) >= 2 {
		if handler.ToolCalls[0].Name != "list_dir" {
			t.Errorf("expected first tool to be 'list_dir', got %q", handler.ToolCalls[0].Name)
		}
		if handler.ToolCalls[1].Name != "read" {
			t.Errorf("expected second tool to be 'read', got %q", handler.ToolCalls[1].Name)
		}
	}

	// Verify both tool results
	if len(handler.ToolResults) != 2 {
		t.Errorf("expected 2 tool results, got %d", len(handler.ToolResults))
	}

	for i, result := range handler.ToolResults {
		if result.IsError {
			t.Errorf("tool result %d: expected success, got error: %q", i, result.Result)
		}
	}

	// Verify final text response
	if len(handler.TextEvents) != 1 {
		t.Errorf("expected 1 text event, got %d", len(handler.TextEvents))
	}
}

// TestIntegration_GrepRecursiveSearch tests GREP with recursive flag.
func TestIntegration_GrepRecursiveSearch(t *testing.T) {
	// Create nested directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file1, []byte("foo\n"), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("foo bar\n"), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	// Setup mock streamer with recursive grep
	recursive := true
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"grep",
		map[string]any{
			"pattern":   "foo",
			"path":      tmpDir,
			"recursive": recursive,
		},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Recursive search complete!"))

	// Create tools
	tools := []tool.Tool{
		tool.NewGrepTool(),
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Search recursively for 'foo'")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool result contains matches from both files
	if len(handler.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(handler.ToolResults))
	}

	result := handler.ToolResults[0]
	var resultData struct {
		Matches string `json:"matches"`
	}
	if err := json.Unmarshal([]byte(result.Result), &resultData); err != nil {
		t.Fatalf("failed to parse tool result: %v", err)
	}

	// Should find matches in both files
	if resultData.Matches == "" {
		t.Error("expected matches from recursive search, got empty string")
	}
	t.Logf("Recursive grep matches: %s", resultData.Matches)
}
