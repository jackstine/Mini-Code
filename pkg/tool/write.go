package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	// defaultFilePermissions is the permission mode for new files.
	defaultFilePermissions = 0644
	// defaultDirPermissions is the permission mode for new directories.
	defaultDirPermissions = 0755
)

// WriteTool implements the Tool interface for writing file contents.
type WriteTool struct{}

// writeInput defines the expected input parameters for the write tool.
type writeInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    string `json:"mode,omitempty"`
}

// writeOutput defines the success response format.
type writeOutput struct {
	BytesWritten int    `json:"bytesWritten"`
	Path         string `json:"path"`
}

// writeError defines the error response format.
type writeError struct {
	Error string `json:"error"`
}

// NewWriteTool creates a new WriteTool instance.
func NewWriteTool() *WriteTool {
	return &WriteTool{}
}

// Name returns the tool identifier.
func (t *WriteTool) Name() string {
	return "write"
}

// Description returns a human-readable description of the tool.
func (t *WriteTool) Description() string {
	return "Write content to a file, creating or overwriting as needed"
}

// InputSchema returns the JSON Schema for the tool's input parameters.
func (t *WriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Absolute or relative file path"},
			"content": {"type": "string", "description": "Content to write to the file"},
			"mode": {"type": "string", "enum": ["overwrite", "append"], "description": "Write mode: overwrite (default) or append"}
		},
		"required": ["path", "content"]
	}`)
}

// Execute writes content to the specified file.
func (t *WriteTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params writeInput
	if err := json.Unmarshal(input, &params); err != nil {
		return formatWriteError("invalid input: " + err.Error()), nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate path
	if params.Path == "" {
		return formatWriteError("path is required"), nil
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(params.Path)
	if err != nil {
		return formatWriteError("invalid path: " + err.Error()), nil
	}

	// Check if path is a directory
	info, err := os.Stat(absPath)
	if err == nil && info.IsDir() {
		return formatWriteError("path is a directory"), nil
	}

	// Determine write mode
	mode := params.Mode
	if mode == "" {
		mode = "overwrite"
	}
	if mode != "overwrite" && mode != "append" {
		return formatWriteError("mode must be 'overwrite' or 'append'"), nil
	}

	// Create parent directories if needed
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, defaultDirPermissions); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return formatWriteError("permission denied creating directories"), nil
		}
		return formatWriteError("failed to create directories: " + err.Error()), nil
	}

	// Get existing file permissions if file exists
	fileMode := os.FileMode(defaultFilePermissions)
	if info != nil && !info.IsDir() {
		fileMode = info.Mode()
	}

	var bytesWritten int

	if mode == "append" {
		// Append mode: open existing file or create new one
		f, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, defaultFilePermissions)
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				return formatWriteError("permission denied"), nil
			}
			return formatWriteError("failed to open file: " + err.Error()), nil
		}
		defer f.Close()

		n, err := f.WriteString(params.Content)
		if err != nil {
			return formatWriteError("failed to write: " + err.Error()), nil
		}
		bytesWritten = n
	} else {
		// Overwrite mode: atomic write using temp file + rename
		bytesWritten, err = atomicWrite(absPath, params.Content, fileMode)
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				return formatWriteError("permission denied"), nil
			}
			return formatWriteError(err.Error()), nil
		}
	}

	return formatWriteSuccess(bytesWritten, absPath), nil
}

// atomicWrite writes content to a temporary file and renames it to the target path.
func atomicWrite(path, content string, perm os.FileMode) (int, error) {
	dir := filepath.Dir(path)

	// Create temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, ".write-*.tmp")
	if err != nil {
		return 0, err
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
	n, err := tmpFile.WriteString(content)
	if err != nil {
		tmpFile.Close()
		return 0, err
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return 0, err
	}

	// Set permissions
	if err := os.Chmod(tmpPath, perm); err != nil {
		return 0, err
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return 0, err
	}

	success = true
	return n, nil
}

// formatWriteSuccess formats a successful write response.
func formatWriteSuccess(bytesWritten int, path string) string {
	output := writeOutput{
		BytesWritten: bytesWritten,
		Path:         path,
	}
	data, _ := json.Marshal(output)
	return string(data)
}

// formatWriteError formats an error response.
func formatWriteError(msg string) string {
	output := writeError{Error: msg}
	data, _ := json.Marshal(output)
	return string(data)
}
