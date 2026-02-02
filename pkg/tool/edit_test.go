package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditTool_Name(t *testing.T) {
	tool := NewEditTool()
	if tool.Name() != "edit" {
		t.Errorf("expected name 'edit', got '%s'", tool.Name())
	}
}

func TestEditTool_Description(t *testing.T) {
	tool := NewEditTool()
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestEditTool_InputSchema(t *testing.T) {
	tool := NewEditTool()
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
	if _, ok := props["operations"]; !ok {
		t.Error("schema should have 'operations' property")
	}
}

func createEditTestFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return filePath
}

func TestEditTool_ReplaceSingleLine(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3\nline4\nline5")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 3, "endLine": 3, "content": ["replaced line 3"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file contents
	content, _ := os.ReadFile(filePath)
	expected := "line1\nline2\nreplaced line 3\nline4\nline5"
	if string(content) != expected {
		t.Errorf("expected content '%s', got '%s'", expected, string(content))
	}
}

func TestEditTool_ReplaceMultipleLines(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3\nline4\nline5")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 2, "endLine": 4, "content": ["new line A", "new line B"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file contents
	content, _ := os.ReadFile(filePath)
	expected := "line1\nnew line A\nnew line B\nline5"
	if string(content) != expected {
		t.Errorf("expected content '%s', got '%s'", expected, string(content))
	}

	if output.NewLineCount != 4 {
		t.Errorf("expected 4 lines, got %d", output.NewLineCount)
	}
}

func TestEditTool_InsertAtBeginning(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "insert", "afterLine": 0, "content": ["header1", "header2"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file contents
	content, _ := os.ReadFile(filePath)
	expected := "header1\nheader2\nline1\nline2\nline3"
	if string(content) != expected {
		t.Errorf("expected content '%s', got '%s'", expected, string(content))
	}
}

func TestEditTool_InsertInMiddle(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "insert", "afterLine": 2, "content": ["inserted"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file contents
	content, _ := os.ReadFile(filePath)
	expected := "line1\nline2\ninserted\nline3"
	if string(content) != expected {
		t.Errorf("expected content '%s', got '%s'", expected, string(content))
	}
}

func TestEditTool_DeleteSingleLine(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3\nline4\nline5")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "delete", "startLine": 3, "endLine": 3}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file contents
	content, _ := os.ReadFile(filePath)
	expected := "line1\nline2\nline4\nline5"
	if string(content) != expected {
		t.Errorf("expected content '%s', got '%s'", expected, string(content))
	}
}

func TestEditTool_DeleteRange(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3\nline4\nline5")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "delete", "startLine": 2, "endLine": 4}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file contents
	content, _ := os.ReadFile(filePath)
	expected := "line1\nline5"
	if string(content) != expected {
		t.Errorf("expected content '%s', got '%s'", expected, string(content))
	}
}

func TestEditTool_MultipleNonOverlappingOperations(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 2, "endLine": 2, "content": ["replaced2"]},
			{"op": "delete", "startLine": 5, "endLine": 6},
			{"op": "insert", "afterLine": 8, "content": ["inserted"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify file contents - operations applied in reverse order for correct line numbers
	content, _ := os.ReadFile(filePath)
	expected := "line1\nreplaced2\nline3\nline4\nline7\nline8\ninserted\nline9\nline10"
	if string(content) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(content))
	}
}

func TestEditTool_OverlapDetection(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3\nline4\nline5")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 2, "endLine": 4, "content": ["replaced"]},
			{"op": "delete", "startLine": 3, "endLine": 5}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "overlap") {
		t.Errorf("expected overlap error, got '%s'", output.Error)
	}
}

func TestEditTool_InvalidLineNumber(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 0, "endLine": 1, "content": ["replaced"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "must be >= 1") {
		t.Errorf("expected line number error, got '%s'", output.Error)
	}
}

func TestEditTool_LineOutOfRange(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 5, "endLine": 6, "content": ["replaced"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "out of range") {
		t.Errorf("expected out of range error, got '%s'", output.Error)
	}
}

func TestEditTool_StartLineGreaterThanEndLine(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 3, "endLine": 1, "content": ["replaced"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "startLine") && !strings.Contains(output.Error, "endLine") {
		t.Errorf("expected invalid range error, got '%s'", output.Error)
	}
}

func TestEditTool_FileNotFound(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.txt")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 1, "endLine": 1, "content": ["replaced"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "file not found") {
		t.Errorf("expected file not found error, got '%s'", output.Error)
	}
}

func TestEditTool_PathIsDirectory(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	tmpDir := t.TempDir()

	input := `{
		"path": "` + tmpDir + `",
		"operations": [
			{"op": "replace", "startLine": 1, "endLine": 1, "content": ["replaced"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "directory") {
		t.Errorf("expected directory error, got '%s'", output.Error)
	}
}

func TestEditTool_NoOperations(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3")

	input := `{
		"path": "` + filePath + `",
		"operations": []
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "no operations") {
		t.Errorf("expected no operations error, got '%s'", output.Error)
	}
}

func TestEditTool_UnknownOperation(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "unknown", "startLine": 1, "endLine": 1}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editError
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if !strings.Contains(output.Error, "unknown operation") {
		t.Errorf("expected unknown operation error, got '%s'", output.Error)
	}
}

func TestEditTool_ContextCancellation(t *testing.T) {
	tool := NewEditTool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := `{
		"path": "test.txt",
		"operations": [
			{"op": "replace", "startLine": 1, "endLine": 1, "content": ["replaced"]}
		]
	}`

	_, err := tool.Execute(ctx, json.RawMessage(input))
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestEditTool_PreservesPermissions(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "perms.txt")
	if err := os.WriteFile(filePath, []byte("line1\nline2\nline3"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 2, "endLine": 2, "content": ["replaced"]}
		]
	}`

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

func TestEditTool_AbsolutePath(t *testing.T) {
	tool := NewEditTool()
	ctx := context.Background()

	filePath := createEditTestFile(t, "line1\nline2\nline3")

	input := `{
		"path": "` + filePath + `",
		"operations": [
			{"op": "replace", "startLine": 1, "endLine": 1, "content": ["replaced"]}
		]
	}`

	result, err := tool.Execute(ctx, json.RawMessage(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output editOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	// Verify returned path is absolute
	if !filepath.IsAbs(output.Path) {
		t.Errorf("expected absolute path, got '%s'", output.Path)
	}
}
