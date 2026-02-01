package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to parse list_dir tool output
func parseListDirOutput(t *testing.T, output string) (entries string, errMsg string) {
	t.Helper()
	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}
	return result["entries"], result["error"]
}

func TestListDirTool_Name(t *testing.T) {
	tool := NewListDirTool()
	if name := tool.Name(); name != "list_dir" {
		t.Errorf("expected name 'list_dir', got %q", name)
	}
}

func TestListDirTool_Description(t *testing.T) {
	tool := NewListDirTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
}

func TestListDirTool_InputSchema(t *testing.T) {
	tool := NewListDirTool()
	schema := tool.InputSchema()
	if len(schema) == 0 {
		t.Error("input schema should not be empty")
	}

	// Verify it's valid JSON
	var schemaMap map[string]any
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		t.Errorf("input schema is not valid JSON: %v", err)
	}

	// Verify required fields
	if schemaMap["type"] != "object" {
		t.Error("schema type should be 'object'")
	}
	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema should have properties")
	}
	if _, ok := props["path"]; !ok {
		t.Error("schema should have 'path' property")
	}
}

func TestListDirTool_ValidDirectory(t *testing.T) {
	tool := NewListDirTool()

	// Create a temp directory with some files
	dir, err := os.MkdirTemp("", "list_dir_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create some test files
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0644)

	input, _ := json.Marshal(map[string]string{"path": dir})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, gotErr := parseListDirOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	// Check that output contains expected elements
	if !strings.Contains(entries, "total") {
		t.Error("output should contain 'total' line")
	}
	if !strings.Contains(entries, "file1.txt") {
		t.Error("output should contain 'file1.txt'")
	}
	if !strings.Contains(entries, "file2.txt") {
		t.Error("output should contain 'file2.txt'")
	}
}

func TestListDirTool_IncludesHiddenFiles(t *testing.T) {
	tool := NewListDirTool()

	// Create a temp directory with a hidden file
	dir, err := os.MkdirTemp("", "list_dir_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, ".hidden_file"), []byte("hidden"), 0644)

	input, _ := json.Marshal(map[string]string{"path": dir})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, gotErr := parseListDirOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	if !strings.Contains(entries, ".hidden_file") {
		t.Error("output should include hidden files")
	}
}

func TestListDirTool_ShowsPermissions(t *testing.T) {
	tool := NewListDirTool()

	dir, err := os.MkdirTemp("", "list_dir_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0644)

	input, _ := json.Marshal(map[string]string{"path": dir})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, _ := parseListDirOutput(t, output)

	// ls -al output should show permission strings like "drwx" or "-rw-"
	if !strings.Contains(entries, "rw") {
		t.Error("output should show file permissions")
	}
}

func TestListDirTool_NonExistentPath(t *testing.T) {
	tool := NewListDirTool()

	input, _ := json.Marshal(map[string]string{"path": "/nonexistent/path/that/does/not/exist"})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseListDirOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for non-existent path")
	}
	if gotErr != "path not found" {
		t.Errorf("expected 'path not found' error, got %q", gotErr)
	}
}

func TestListDirTool_PathIsFile(t *testing.T) {
	tool := NewListDirTool()

	// Create a temp file
	f, err := os.CreateTemp("", "list_dir_test_file")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	f.Close()
	defer os.Remove(f.Name())

	input, _ := json.Marshal(map[string]string{"path": f.Name()})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseListDirOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for file path")
	}
	if gotErr != "not a directory" {
		t.Errorf("expected 'not a directory' error, got %q", gotErr)
	}
}

func TestListDirTool_ContextCancellation(t *testing.T) {
	tool := NewListDirTool()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input, _ := json.Marshal(map[string]string{"path": "/tmp"})
	_, err := tool.Execute(ctx, input)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestListDirTool_InvalidJSON(t *testing.T) {
	tool := NewListDirTool()
	output, err := tool.Execute(context.Background(), []byte("invalid json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseListDirOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for invalid JSON input")
	}
}

func TestListDirTool_EmptyPath(t *testing.T) {
	tool := NewListDirTool()
	input, _ := json.Marshal(map[string]string{"path": ""})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseListDirOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for empty path")
	}
}

func TestListDirTool_EmptyDirectory(t *testing.T) {
	tool := NewListDirTool()

	// Create an empty temp directory
	dir, err := os.MkdirTemp("", "list_dir_test_empty")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	input, _ := json.Marshal(map[string]string{"path": dir})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, gotErr := parseListDirOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	// Empty directory still shows . and .. entries with ls -al
	if !strings.Contains(entries, "total") {
		t.Error("output should contain 'total' line even for empty directory")
	}
}

func TestListDirTool_RelativePath(t *testing.T) {
	tool := NewListDirTool()

	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "list_dir_test_rel")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("content"), 0644)

	// Change to the temp directory and use relative path
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	input, _ := json.Marshal(map[string]string{"path": "subdir"})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, gotErr := parseListDirOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	if !strings.Contains(entries, "file.txt") {
		t.Error("output should contain 'file.txt'")
	}
}

func TestListDirTool_CurrentDirectory(t *testing.T) {
	tool := NewListDirTool()

	input, _ := json.Marshal(map[string]string{"path": "."})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, gotErr := parseListDirOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	// Current directory should show something
	if entries == "" {
		t.Error("output should not be empty for current directory")
	}
}
