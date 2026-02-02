package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMoveTool_Name(t *testing.T) {
	tool := NewMoveTool()
	if tool.Name() != "move" {
		t.Errorf("expected name 'move', got '%s'", tool.Name())
	}
}

func TestMoveTool_Description(t *testing.T) {
	tool := NewMoveTool()
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestMoveTool_InputSchema(t *testing.T) {
	tool := NewMoveTool()
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

	if _, ok := props["source"]; !ok {
		t.Error("schema should have 'source' property")
	}
	if _, ok := props["destination"]; !ok {
		t.Error("schema should have 'destination' property")
	}
}

func TestMoveTool_RenameFile(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	// Create temp directory and file
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "old.txt")
	dstPath := filepath.Join(tmpDir, "new.txt")

	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{"source": "` + srcPath + `", "destination": "` + dstPath + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify source no longer exists
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("source file should no longer exist")
	}

	// Verify destination exists with correct content
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("expected content 'content', got '%s'", string(content))
	}
}

func TestMoveTool_MoveToDirectory(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	// Create temp directory structure
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "file.txt")
	dstDir := filepath.Join(tmpDir, "subdir")
	dstPath := filepath.Join(dstDir, "file.txt")

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("failed to create destination dir: %v", err)
	}
	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Move into existing directory (should preserve filename)
	input := `{"source": "` + srcPath + `", "destination": "` + dstDir + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify destination path
	if output.Destination != dstPath {
		t.Errorf("expected destination '%s', got '%s'", dstPath, output.Destination)
	}

	// Verify file moved
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("source file should no longer exist")
	}
	if _, err := os.Stat(dstPath); err != nil {
		t.Error("destination file should exist")
	}
}

func TestMoveTool_CreateParentDirectories(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "file.txt")
	dstPath := filepath.Join(tmpDir, "nested", "dirs", "file.txt")

	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{"source": "` + srcPath + `", "destination": "` + dstPath + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file moved to nested path
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("expected content 'content', got '%s'", string(content))
	}
}

func TestMoveTool_MoveDirectory(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "srcdir")
	dstDir := filepath.Join(tmpDir, "dstdir")

	// Create source directory with file
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{"source": "` + srcDir + `", "destination": "` + dstDir + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify source no longer exists
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("source directory should no longer exist")
	}

	// Verify destination exists with file
	content, err := os.ReadFile(filepath.Join(dstDir, "file.txt"))
	if err != nil {
		t.Fatalf("failed to read file in destination: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("expected content 'content', got '%s'", string(content))
	}
}

func TestMoveTool_OverwriteFile(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.txt")
	dstPath := filepath.Join(tmpDir, "dst.txt")

	if err := os.WriteFile(srcPath, []byte("new content"), 0644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}
	if err := os.WriteFile(dstPath, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create destination: %v", err)
	}

	input := `{"source": "` + srcPath + `", "destination": "` + dstPath + `"}`
	_, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify content was overwritten
	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(content) != "new content" {
		t.Errorf("expected content 'new content', got '%s'", string(content))
	}
}

func TestMoveTool_SourceNotFound(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "dst.txt")

	input := `{"source": "` + srcPath + `", "destination": "` + dstPath + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "source not found") {
		t.Errorf("expected 'source not found' error, got '%s'", output.Error)
	}
}

func TestMoveTool_SourceEqualsDestination(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")

	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{"source": "` + filePath + `", "destination": "` + filePath + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "same") {
		t.Errorf("expected 'same' error, got '%s'", output.Error)
	}
}

func TestMoveTool_MoveIntoSelf(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "srcdir")
	dstDir := filepath.Join(srcDir, "subdir")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	input := `{"source": "` + srcDir + `", "destination": "` + dstDir + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "into itself") {
		t.Errorf("expected 'into itself' error, got '%s'", output.Error)
	}
}

func TestMoveTool_NonEmptyDirectoryError(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "file.txt")
	dstDir := filepath.Join(tmpDir, "nonempty")

	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("failed to create destination dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "existing.txt"), []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	// Try to move file to a path that is a non-empty directory
	// This should move into the directory, not error
	input := `{"source": "` + srcPath + `", "destination": "` + dstDir + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// File should be moved into the directory
	expectedDst := filepath.Join(dstDir, "file.txt")
	if output.Destination != expectedDst {
		t.Errorf("expected destination '%s', got '%s'", expectedDst, output.Destination)
	}
}

func TestMoveTool_EmptySource(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	input := `{"source": "", "destination": "/tmp/dst"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "source is required") {
		t.Errorf("expected 'source is required' error, got '%s'", output.Error)
	}
}

func TestMoveTool_EmptyDestination(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	input := `{"source": "/tmp/src", "destination": ""}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "destination is required") {
		t.Errorf("expected 'destination is required' error, got '%s'", output.Error)
	}
}

func TestMoveTool_ContextCancellation(t *testing.T) {
	tool := NewMoveTool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := `{"source": "src.txt", "destination": "dst.txt"}`
	_, err := tool.Execute(ctx, json.RawMessage(input))
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestMoveTool_AbsolutePaths(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.txt")
	dstPath := filepath.Join(tmpDir, "dst.txt")

	if err := os.WriteFile(srcPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{"source": "` + srcPath + `", "destination": "` + dstPath + `"}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify returned paths are absolute
	if !filepath.IsAbs(output.Source) {
		t.Errorf("expected absolute source path, got '%s'", output.Source)
	}
	if !filepath.IsAbs(output.Destination) {
		t.Errorf("expected absolute destination path, got '%s'", output.Destination)
	}
}

func TestMoveTool_InvalidInput(t *testing.T) {
	tool := NewMoveTool()
	ctx := context.Background()

	input := `{invalid json}`
	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output moveError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.Error == "" {
		t.Error("expected error for invalid input")
	}
}
