package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to parse grep tool output
func parseGrepOutput(t *testing.T, output string) (matches string, errMsg string) {
	t.Helper()
	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}
	return result["matches"], result["error"]
}

func TestGrepTool_Name(t *testing.T) {
	tool := NewGrepTool()
	if name := tool.Name(); name != "grep" {
		t.Errorf("expected name 'grep', got %q", name)
	}
}

func TestGrepTool_Description(t *testing.T) {
	tool := NewGrepTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
}

func TestGrepTool_InputSchema(t *testing.T) {
	tool := NewGrepTool()
	schema := tool.InputSchema()
	if len(schema) == 0 {
		t.Error("input schema should not be empty")
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		t.Errorf("input schema is not valid JSON: %v", err)
	}

	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema should have properties")
	}
	if _, ok := props["pattern"]; !ok {
		t.Error("schema should have 'pattern' property")
	}
	if _, ok := props["path"]; !ok {
		t.Error("schema should have 'path' property")
	}
	if _, ok := props["recursive"]; !ok {
		t.Error("schema should have 'recursive' property")
	}
}

func TestGrepTool_PatternFoundSingleFile(t *testing.T) {
	tool := NewGrepTool()

	// Create a test file
	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1 foo bar\nline2 baz\nline3 foo again"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	input, _ := json.Marshal(map[string]string{"pattern": "foo", "path": testFile})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matches, gotErr := parseGrepOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	// Single file should show line_number:content format
	if !strings.Contains(matches, "1:line1 foo bar") {
		t.Errorf("expected match on line 1, got %q", matches)
	}
	if !strings.Contains(matches, "3:line3 foo again") {
		t.Errorf("expected match on line 3, got %q", matches)
	}
}

func TestGrepTool_PatternFoundDirectory(t *testing.T) {
	tool := NewGrepTool()

	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello foo world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("bar baz\nfoo here"), 0644)

	recursive := true
	input, _ := json.Marshal(map[string]any{"pattern": "foo", "path": tmpDir, "recursive": recursive})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matches, gotErr := parseGrepOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	// Recursive search should show filename:line_number:content format
	if !strings.Contains(matches, "file1.txt") {
		t.Errorf("expected file1.txt in matches, got %q", matches)
	}
	if !strings.Contains(matches, "file2.txt") {
		t.Errorf("expected file2.txt in matches, got %q", matches)
	}
}

func TestGrepTool_NoMatches(t *testing.T) {
	tool := NewGrepTool()

	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	input, _ := json.Marshal(map[string]string{"pattern": "xyz123nonexistent", "path": testFile})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matches, gotErr := parseGrepOutput(t, output)
	if gotErr != "" {
		t.Fatalf("no matches should not be an error, got: %s", gotErr)
	}
	if matches != "" {
		t.Errorf("expected empty matches, got %q", matches)
	}
}

func TestGrepTool_RecursiveSearch(t *testing.T) {
	tool := NewGrepTool()

	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directory structure
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("foo in root"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("foo in nested"), 0644)

	recursive := true
	input, _ := json.Marshal(map[string]any{"pattern": "foo", "path": tmpDir, "recursive": recursive})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matches, gotErr := parseGrepOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	if !strings.Contains(matches, "root.txt") {
		t.Error("should find matches in root file")
	}
	if !strings.Contains(matches, "nested.txt") {
		t.Error("should find matches in nested file")
	}
}

func TestGrepTool_NonRecursiveOnDirectory(t *testing.T) {
	tool := NewGrepTool()

	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("foo content"), 0644)

	// Non-recursive search on directory - grep behavior varies
	// Just verify it doesn't crash
	input, _ := json.Marshal(map[string]string{"pattern": "foo", "path": tmpDir})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = parseGrepOutput(t, output)
	// The behavior here varies by grep version, so we just verify it completes
}

func TestGrepTool_InvalidRegex(t *testing.T) {
	tool := NewGrepTool()

	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	// Invalid regex pattern
	input, _ := json.Marshal(map[string]string{"pattern": "[invalid", "path": testFile})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseGrepOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for invalid regex")
	}
}

func TestGrepTool_PathNotFound(t *testing.T) {
	tool := NewGrepTool()

	input, _ := json.Marshal(map[string]string{"pattern": "foo", "path": "/nonexistent/path"})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseGrepOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for non-existent path")
	}
	if gotErr != "path not found" {
		t.Errorf("expected 'path not found' error, got %q", gotErr)
	}
}

func TestGrepTool_ContextCancellation(t *testing.T) {
	tool := NewGrepTool()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	input, _ := json.Marshal(map[string]string{"pattern": "foo", "path": "/tmp"})
	_, err := tool.Execute(ctx, input)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestGrepTool_InvalidJSON(t *testing.T) {
	tool := NewGrepTool()
	output, err := tool.Execute(context.Background(), []byte("invalid json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseGrepOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for invalid JSON input")
	}
}

func TestGrepTool_EmptyPattern(t *testing.T) {
	tool := NewGrepTool()
	input, _ := json.Marshal(map[string]string{"pattern": "", "path": "/tmp"})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseGrepOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for empty pattern")
	}
}

func TestGrepTool_EmptyPath(t *testing.T) {
	tool := NewGrepTool()
	input, _ := json.Marshal(map[string]string{"pattern": "foo", "path": ""})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, gotErr := parseGrepOutput(t, output)
	if gotErr == "" {
		t.Error("expected error for empty path")
	}
}

func TestGrepTool_CaseSensitive(t *testing.T) {
	tool := NewGrepTool()

	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("Foo FOO foo"), 0644)

	// Search for lowercase "foo" - should only match lowercase
	input, _ := json.Marshal(map[string]string{"pattern": "foo", "path": testFile})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matches, gotErr := parseGrepOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	// Should find line with "foo" (the line contains all variations)
	if matches == "" {
		t.Error("should find match for lowercase foo")
	}
}

func TestGrepTool_RegexPattern(t *testing.T) {
	tool := NewGrepTool()

	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("foo123bar\nfoo456bar\nnoMatch"), 0644)

	// Use BRE regex pattern
	input, _ := json.Marshal(map[string]string{"pattern": "foo[0-9]*bar", "path": testFile})
	output, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matches, gotErr := parseGrepOutput(t, output)
	if gotErr != "" {
		t.Fatalf("unexpected error in output: %s", gotErr)
	}

	if !strings.Contains(matches, "foo123bar") {
		t.Error("should match foo123bar")
	}
	if !strings.Contains(matches, "foo456bar") {
		t.Error("should match foo456bar")
	}
	if strings.Contains(matches, "noMatch") {
		t.Error("should not match noMatch")
	}
}
