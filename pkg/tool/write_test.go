package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTool_Name(t *testing.T) {
	tool := NewWriteTool()
	if tool.Name() != "write" {
		t.Errorf("expected name 'write', got '%s'", tool.Name())
	}
}

func TestWriteTool_Description(t *testing.T) {
	tool := NewWriteTool()
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestWriteTool_InputSchema(t *testing.T) {
	tool := NewWriteTool()
	schema := tool.InputSchema()

	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema should be valid JSON: %v", err)
	}

	if parsed["type"] != "object" {
		t.Error("schema type should be 'object'")
	}

	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema should have properties")
	}

	if _, ok := props["path"]; !ok {
		t.Error("schema should have 'path' property")
	}
	if _, ok := props["content"]; !ok {
		t.Error("schema should have 'content' property")
	}
	if _, ok := props["mode"]; !ok {
		t.Error("schema should have 'mode' property")
	}
}

func TestWriteTool_CreateNewFile(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	// Create temp directory for test
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "newfile.txt")

	input := `{"path": "` + filePath + `", "content": "hello world"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.BytesWritten != 11 {
		t.Errorf("expected 11 bytes written, got %d", output.BytesWritten)
	}
	if output.Path != filePath {
		t.Errorf("expected path '%s', got '%s'", filePath, output.Path)
	}

	// Verify file contents
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected content 'hello world', got '%s'", string(content))
	}
}

func TestWriteTool_OverwriteExistingFile(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	// Create temp directory and file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(filePath, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{"path": "` + filePath + `", "content": "new content"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file was overwritten
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "new content" {
		t.Errorf("expected content 'new content', got '%s'", string(content))
	}
}

func TestWriteTool_AppendToFile(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	// Create temp directory and file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "append.txt")
	if err := os.WriteFile(filePath, []byte("line1\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{"path": "` + filePath + `", "content": "line2\n", "mode": "append"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.BytesWritten != 6 {
		t.Errorf("expected 6 bytes written, got %d", output.BytesWritten)
	}

	// Verify file was appended
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "line1\nline2\n" {
		t.Errorf("expected content 'line1\\nline2\\n', got '%s'", string(content))
	}
}

func TestWriteTool_CreateNestedDirectories(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	// Create temp directory
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nested", "dirs", "file.txt")

	input := `{"path": "` + filePath + `", "content": "nested content"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file exists
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "nested content" {
		t.Errorf("expected content 'nested content', got '%s'", string(content))
	}

	// Verify directory permissions
	nestedDir := filepath.Join(tmpDir, "nested")
	info, err := os.Stat(nestedDir)
	if err != nil {
		t.Fatalf("failed to stat directory: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("expected directory permissions 0755, got %o", info.Mode().Perm())
	}
}

func TestWriteTool_PreserveFilePermissions(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	// Create temp directory and file with specific permissions
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "perms.txt")
	if err := os.WriteFile(filePath, []byte("old"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{"path": "` + filePath + `", "content": "new"}`
	_, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify permissions were preserved
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestWriteTool_EmptyPath(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	input := `{"path": "", "content": "test"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "path is required") {
		t.Errorf("expected 'path is required' error, got '%s'", output.Error)
	}
}

func TestWriteTool_PathIsDirectory(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	// Use temp directory as path
	tmpDir := t.TempDir()

	input := `{"path": "` + tmpDir + `", "content": "test"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "directory") {
		t.Errorf("expected directory error, got '%s'", output.Error)
	}
}

func TestWriteTool_InvalidMode(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	input := `{"path": "` + filePath + `", "content": "test", "mode": "invalid"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "mode") {
		t.Errorf("expected mode error, got '%s'", output.Error)
	}
}

func TestWriteTool_InvalidInput(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	input := `{invalid json}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.Error == "" {
		t.Error("expected error for invalid input")
	}
}

func TestWriteTool_ContextCancellation(t *testing.T) {
	tool := NewWriteTool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := `{"path": "test.txt", "content": "test"}`
	_, err := tool.Execute(ctx, json.RawMessage(input))
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestWriteTool_NewFilePermissions(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	// Create temp directory
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "newperms.txt")

	input := `{"path": "` + filePath + `", "content": "test"}`
	_, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify new file has 0644 permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("expected new file permissions 0644, got %o", info.Mode().Perm())
	}
}

func TestWriteTool_AppendToNonExistent(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	// Create temp directory
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.txt")

	input := `{"path": "` + filePath + `", "content": "first line\n", "mode": "append"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "first line\n" {
		t.Errorf("expected content 'first line\\n', got '%s'", string(content))
	}
}

func TestWriteTool_AbsolutePath(t *testing.T) {
	tool := NewWriteTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "abs.txt")

	input := `{"path": "` + filePath + `", "content": "test"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output writeOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify returned path is absolute
	if !filepath.IsAbs(output.Path) {
		t.Errorf("expected absolute path, got '%s'", output.Path)
	}
}
