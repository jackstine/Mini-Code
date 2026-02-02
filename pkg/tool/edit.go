package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EditTool implements the Tool interface for line-based file editing.
type EditTool struct{}

// editInput defines the expected input parameters for the edit tool.
type editInput struct {
	Path       string      `json:"path"`
	Operations []Operation `json:"operations"`
}

// Operation represents a single edit operation.
type Operation struct {
	Op        string   `json:"op"`
	StartLine int      `json:"startLine,omitempty"`
	EndLine   int      `json:"endLine,omitempty"`
	AfterLine int      `json:"afterLine,omitempty"`
	Content   []string `json:"content,omitempty"`
}

// editOutput defines the success response format.
type editOutput struct {
	Path         string `json:"path"`
	LinesChanged int    `json:"linesChanged"`
	NewLineCount int    `json:"newLineCount"`
}

// editError defines the error response format.
type editError struct {
	Error string `json:"error"`
}

// NewEditTool creates a new EditTool instance.
func NewEditTool() *EditTool {
	return &EditTool{}
}

// Name returns the tool identifier.
func (t *EditTool) Name() string {
	return "edit"
}

// Description returns a human-readable description of the tool.
func (t *EditTool) Description() string {
	return "Edit a file using line-based operations (replace, insert, delete)"
}

// InputSchema returns the JSON Schema for the tool's input parameters.
func (t *EditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path to edit"},
			"operations": {
				"type": "array",
				"description": "List of edit operations",
				"items": {
					"type": "object",
					"properties": {
						"op": {"type": "string", "enum": ["replace", "insert", "delete"]},
						"startLine": {"type": "integer", "description": "First line (1-indexed) for replace/delete"},
						"endLine": {"type": "integer", "description": "Last line (inclusive) for replace/delete"},
						"afterLine": {"type": "integer", "description": "Insert after this line (0 = beginning)"},
						"content": {"type": "array", "items": {"type": "string"}, "description": "Lines to insert/replace with"}
					},
					"required": ["op"]
				}
			}
		},
		"required": ["path", "operations"]
	}`)
}

// Execute performs the edit operations on the specified file.
func (t *EditTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params editInput
	if err := json.Unmarshal(input, &params); err != nil {
		return formatEditError("invalid input: " + err.Error()), nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate path
	if params.Path == "" {
		return formatEditError("path is required"), nil
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(params.Path)
	if err != nil {
		return formatEditError("invalid path: " + err.Error()), nil
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return formatEditError(fmt.Sprintf("file not found: %s", params.Path)), nil
		}
		if errors.Is(err, os.ErrPermission) {
			return formatEditError(fmt.Sprintf("permission denied: %s", params.Path)), nil
		}
		return formatEditError(err.Error()), nil
	}

	// Check if path is a directory
	if info.IsDir() {
		return formatEditError(fmt.Sprintf("path is a directory: %s", params.Path)), nil
	}

	// Validate operations
	if len(params.Operations) == 0 {
		return formatEditError("no operations provided"), nil
	}

	// Read file into lines
	lines, err := readLines(absPath)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return formatEditError(fmt.Sprintf("permission denied: %s", params.Path)), nil
		}
		return formatEditError("failed to read file: " + err.Error()), nil
	}

	totalLines := len(lines)

	// Validate all operations before applying any
	if err := validateOperations(params.Operations, totalLines); err != nil {
		return formatEditError(err.Error()), nil
	}

	// Check for overlapping operations
	if err := checkOverlaps(params.Operations); err != nil {
		return formatEditError(err.Error()), nil
	}

	// Sort operations by position descending (highest first)
	sortedOps := make([]Operation, len(params.Operations))
	copy(sortedOps, params.Operations)
	sort.Slice(sortedOps, func(i, j int) bool {
		return getOperationPosition(sortedOps[i]) > getOperationPosition(sortedOps[j])
	})

	// Count lines changed
	linesChanged := 0
	for _, op := range sortedOps {
		switch op.Op {
		case "replace":
			linesChanged += (op.EndLine - op.StartLine + 1) + len(op.Content)
		case "insert":
			linesChanged += len(op.Content)
		case "delete":
			linesChanged += op.EndLine - op.StartLine + 1
		}
	}

	// Apply each operation
	for _, op := range sortedOps {
		lines = applyOperation(lines, op)
	}

	// Write atomically
	content := strings.Join(lines, "\n")
	if err := atomicWriteEdit(absPath, content, info.Mode()); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return formatEditError(fmt.Sprintf("permission denied: %s", params.Path)), nil
		}
		return formatEditError("failed to write file: " + err.Error()), nil
	}

	return formatEditSuccess(absPath, linesChanged, len(lines)), nil
}

// readLines reads a file and returns its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Handle empty file
	if lines == nil {
		lines = []string{}
	}

	return lines, nil
}

// validateOperations checks that all operations are valid for the given file.
func validateOperations(ops []Operation, totalLines int) error {
	for i, op := range ops {
		switch op.Op {
		case "replace":
			if op.StartLine < 1 {
				return fmt.Errorf("invalid line number: %d (must be >= 1)", op.StartLine)
			}
			if op.EndLine < op.StartLine {
				return fmt.Errorf("invalid range: startLine %d > endLine %d", op.StartLine, op.EndLine)
			}
			if op.StartLine > totalLines {
				return fmt.Errorf("line %d out of range (file has %d lines)", op.StartLine, totalLines)
			}
			if op.EndLine > totalLines {
				return fmt.Errorf("line %d out of range (file has %d lines)", op.EndLine, totalLines)
			}

		case "insert":
			if op.AfterLine < 0 {
				return fmt.Errorf("invalid line number: %d (must be >= 0)", op.AfterLine)
			}
			if op.AfterLine > totalLines {
				return fmt.Errorf("line %d out of range (file has %d lines)", op.AfterLine, totalLines)
			}

		case "delete":
			if op.StartLine < 1 {
				return fmt.Errorf("invalid line number: %d (must be >= 1)", op.StartLine)
			}
			if op.EndLine < op.StartLine {
				return fmt.Errorf("invalid range: startLine %d > endLine %d", op.StartLine, op.EndLine)
			}
			if op.StartLine > totalLines {
				return fmt.Errorf("line %d out of range (file has %d lines)", op.StartLine, totalLines)
			}
			if op.EndLine > totalLines {
				return fmt.Errorf("line %d out of range (file has %d lines)", op.EndLine, totalLines)
			}

		default:
			return fmt.Errorf("unknown operation: %s (operation %d)", op.Op, i+1)
		}
	}
	return nil
}

// checkOverlaps verifies that no operations affect the same lines.
func checkOverlaps(ops []Operation) error {
	type lineRange struct {
		start int
		end   int
	}

	var ranges []lineRange
	for _, op := range ops {
		switch op.Op {
		case "replace", "delete":
			ranges = append(ranges, lineRange{op.StartLine, op.EndLine})
		case "insert":
			// Insert operations don't overlap with each other at the same afterLine
			// They do overlap if they share an afterLine though
			ranges = append(ranges, lineRange{op.AfterLine, op.AfterLine})
		}
	}

	// Check each pair of ranges for overlap
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			r1, r2 := ranges[i], ranges[j]
			// Two ranges overlap if one starts before the other ends and ends after the other starts
			if r1.start <= r2.end && r1.end >= r2.start {
				return fmt.Errorf("operations overlap at line %d", max(r1.start, r2.start))
			}
		}
	}

	return nil
}

// getOperationPosition returns the line position for sorting (higher = applied first).
func getOperationPosition(op Operation) int {
	switch op.Op {
	case "replace", "delete":
		return op.StartLine
	case "insert":
		return op.AfterLine
	default:
		return 0
	}
}

// applyOperation applies a single operation to the lines.
func applyOperation(lines []string, op Operation) []string {
	switch op.Op {
	case "replace":
		// Remove old lines, insert new
		return splice(lines, op.StartLine-1, op.EndLine-op.StartLine+1, op.Content)

	case "insert":
		// Insert after specified line
		return splice(lines, op.AfterLine, 0, op.Content)

	case "delete":
		// Remove lines, insert nothing
		return splice(lines, op.StartLine-1, op.EndLine-op.StartLine+1, nil)
	}
	return lines
}

// splice removes deleteCount elements at start and inserts new elements.
func splice(lines []string, start, deleteCount int, insert []string) []string {
	result := make([]string, 0, len(lines)-deleteCount+len(insert))
	result = append(result, lines[:start]...)
	result = append(result, insert...)
	result = append(result, lines[start+deleteCount:]...)
	return result
}

// atomicWriteEdit writes content to a temporary file and renames it to the target path.
func atomicWriteEdit(path, content string, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Create temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, ".edit-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write content
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return err
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Set permissions
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	success = true
	return nil
}

// formatEditSuccess formats a successful edit response.
func formatEditSuccess(path string, linesChanged, newLineCount int) string {
	output := editOutput{
		Path:         path,
		LinesChanged: linesChanged,
		NewLineCount: newLineCount,
	}
	data, _ := json.Marshal(output)
	return string(data)
}

// formatEditError formats an error response.
func formatEditError(msg string) string {
	output := editError{Error: msg}
	data, _ := json.Marshal(output)
	return string(data)
}
