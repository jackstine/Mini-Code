package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Helper to create a temporary test file with given content
func createTestFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "read_test_*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatalf("failed to write to temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

// Helper to parse tool output
func parseReadOutput(t *testing.T, output string) (content string, errMsg string) {
	t.Helper()
	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}
	return result["content"], result["error"]
}

func TestReadTool_Name(t *testing.T) {
	tool := NewReadTool()
	if name := tool.Name(); name != "read" {
		t.Errorf("expected name 'read', got %q", name)
	}
}

func TestReadTool_Description(t *testing.T) {
	tool := NewReadTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
}

func TestReadTool_InputSchema(t *testing.T) {
	tool := NewReadTool()
	schema := tool.InputSchema()
	if len(schema) == 0 {
		t.Error("input schema should not be empty")
	}

	// Verify it's valid JSON
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		t.Errorf("input schema is not valid JSON: %v", err)
	}

	// Verify required fields
	if schemaMap["type"] != "object" {
		t.Error("schema type should be 'object'")
	}
	props, ok := schemaMap["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema should have properties")
	}
	if _, ok := props["path"]; !ok {
		t.Error("schema should have 'path' property")
	}
	if _, ok := props["start_line"]; !ok {
		t.Error("schema should have 'start_line' property")
	}
	if _, ok := props["end_line"]; !ok {
		t.Error("schema should have 'end_line' property")
	}
}

func TestReadTool_ReadEntireFile(t *testing.T) {
	tool := NewReadTool()
	content := "line1\nline2\nline3"
	path := createTestFile(t, content)
	defer os.Remove(path)

	input, _ := json.Marshal(map[string]string{"path": path})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotContent, gotErr := parseReadOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}
	if gotContent != content {
		t.Errorf("expected content %q, got %q", content, gotContent)
	}
}

func TestReadTool_ReadWithStartLineOnly(t *testing.T) {
	tool := NewReadTool()
	content := "line1\nline2\nline3"
	path := createTestFile(t, content)
	defer os.Remove(path)

	startLine := 2
	input, _ := json.Marshal(map[string]interface{}{"path": path, "start_line": startLine})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotContent, gotErr := parseReadOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}
	expected := "line2\nline3"
	if gotContent != expected {
		t.Errorf("expected content %q, got %q", expected, gotContent)
	}
}

func TestReadTool_ReadWithEndLineOnly(t *testing.T) {
	tool := NewReadTool()
	content := "line1\nline2\nline3"
	path := createTestFile(t, content)
	defer os.Remove(path)

	endLine := 2
	input, _ := json.Marshal(map[string]interface{}{"path": path, "end_line": endLine})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotContent, gotErr := parseReadOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}
	expected := "line1\nline2"
	if gotContent != expected {
		t.Errorf("expected content %q, got %q", expected, gotContent)
	}
}

func TestReadTool_ReadSpecificRange(t *testing.T) {
	tool := NewReadTool()
	content := "line1\nline2\nline3\nline4\nline5"
	path := createTestFile(t, content)
	defer os.Remove(path)

	startLine := 2
	endLine := 4
	input, _ := json.Marshal(map[string]interface{}{"path": path, "start_line": startLine, "end_line": endLine})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotContent, gotErr := parseReadOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}
	expected := "line2\nline3\nline4"
	if gotContent != expected {
		t.Errorf("expected content %q, got %q", expected, gotContent)
	}
}

func TestReadTool_FileNotFound(t *testing.T) {
	tool := NewReadTool()
	input, _ := json.Marshal(map[string]string{"path": "/nonexistent/file/path.txt"})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseReadOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for nonexistent file")
	}
	if gotErr != "file not found" {
		t.Errorf("expected 'file not found' error, got %q", gotErr)
	}
}

func TestReadTool_PathIsDirectory(t *testing.T) {
	tool := NewReadTool()
	dir, err := os.MkdirTemp("", "read_test_dir")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	input, _ := json.Marshal(map[string]string{"path": dir})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseReadOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for directory path")
	}
	if gotErr != "path is a directory" {
		t.Errorf("expected 'path is a directory' error, got %q", gotErr)
	}
}

func TestReadTool_StartLineLessThanOne(t *testing.T) {
	tool := NewReadTool()
	content := "line1\nline2"
	path := createTestFile(t, content)
	defer os.Remove(path)

	startLine := 0
	input, _ := json.Marshal(map[string]interface{}{"path": path, "start_line": startLine})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseReadOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for start_line < 1")
	}
}

func TestReadTool_StartLineGreaterThanEndLine(t *testing.T) {
	tool := NewReadTool()
	content := "line1\nline2\nline3"
	path := createTestFile(t, content)
	defer os.Remove(path)

	startLine := 5
	endLine := 2
	input, _ := json.Marshal(map[string]interface{}{"path": path, "start_line": startLine, "end_line": endLine})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseReadOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for start_line > end_line")
	}
}

func TestReadTool_StartLineExceedsFileLength(t *testing.T) {
	tool := NewReadTool()
	content := "line1\nline2\nline3"
	path := createTestFile(t, content)
	defer os.Remove(path)

	startLine := 9999
	input, _ := json.Marshal(map[string]interface{}{"path": path, "start_line": startLine})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseReadOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for start_line > file length")
	}
}

func TestReadTool_EmptyFile(t *testing.T) {
	tool := NewReadTool()
	path := createTestFile(t, "")
	defer os.Remove(path)

	input, _ := json.Marshal(map[string]string{"path": path})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotContent, gotErr := parseReadOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}
	if gotContent != "" {
		t.Errorf("expected empty content, got %q", gotContent)
	}
}

func TestReadTool_SingleLine(t *testing.T) {
	tool := NewReadTool()
	content := "single line without newline"
	path := createTestFile(t, content)
	defer os.Remove(path)

	startLine := 1
	endLine := 1
	input, _ := json.Marshal(map[string]interface{}{"path": path, "start_line": startLine, "end_line": endLine})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotContent, gotErr := parseReadOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}
	if gotContent != content {
		t.Errorf("expected content %q, got %q", content, gotContent)
	}
}

func TestReadTool_ContextCancellation(t *testing.T) {
	tool := NewReadTool()
	path := createTestFile(t, "line1\nline2")
	defer os.Remove(path)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input, _ := json.Marshal(map[string]string{"path": path})
	_, err := tool.Execute(ctx, input)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestReadTool_InvalidJSON(t *testing.T) {
	tool := NewReadTool()
	output, err := tool.Execute(context.Background(), []byte("invalid json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseReadOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for invalid JSON input")
	}
}

func TestReadTool_EmptyPath(t *testing.T) {
	tool := NewReadTool()
	input, _ := json.Marshal(map[string]string{"path": ""})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseReadOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for empty path")
	}
}

func TestReadTool_RelativePath(t *testing.T) {
	tool := NewReadTool()
	// Create a file in the current directory
	content := "relative path content"
	tmpDir, err := os.MkdirTemp("", "read_test_rel")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Change to the temp directory and use relative path
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	input, _ := json.Marshal(map[string]string{"path": "test.txt"})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotContent, gotErr := parseReadOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}
	if gotContent != content {
		t.Errorf("expected content %q, got %q", content, gotContent)
	}
}

func TestReadTool_EndLineBeyondFileLength(t *testing.T) {
	tool := NewReadTool()
	content := "line1\nline2\nline3"
	path := createTestFile(t, content)
	defer os.Remove(path)

	// end_line beyond file length should just read to end
	endLine := 100
	input, _ := json.Marshal(map[string]interface{}{"path": path, "end_line": endLine})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotContent, gotErr := parseReadOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}
	if gotContent != content {
		t.Errorf("expected content %q, got %q", content, gotContent)
	}
}
