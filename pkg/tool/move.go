package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MoveTool implements the Tool interface for moving/renaming files and directories.
type MoveTool struct{}

// moveInput defines the expected input parameters for the move tool.
type moveInput struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// moveOutput defines the success response format.
type moveOutput struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// moveError defines the error response format.
type moveError struct {
	Error string `json:"error"`
}

// NewMoveTool creates a new MoveTool instance.
func NewMoveTool() *MoveTool {
	return &MoveTool{}
}

// Name returns the tool identifier.
func (t *MoveTool) Name() string {
	return "move"
}

// Description returns a human-readable description of the tool.
func (t *MoveTool) Description() string {
	return "Move or rename a file or directory"
}

// InputSchema returns the JSON Schema for the tool's input parameters.
func (t *MoveTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"source": {"type": "string", "description": "Path to the file or directory to move"},
			"destination": {"type": "string", "description": "Target path (new location or new name)"}
		},
		"required": ["source", "destination"]
	}`)
}

// Execute moves or renames the specified file or directory.
func (t *MoveTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params moveInput
	if err := json.Unmarshal(input, &params); err != nil {
		return formatMoveError("invalid input: " + err.Error()), nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate paths
	if params.Source == "" {
		return formatMoveError("source is required"), nil
	}
	if params.Destination == "" {
		return formatMoveError("destination is required"), nil
	}

	// Resolve to absolute paths
	srcAbs, err := filepath.Abs(params.Source)
	if err != nil {
		return formatMoveError("invalid source path: " + err.Error()), nil
	}
	dstAbs, err := filepath.Abs(params.Destination)
	if err != nil {
		return formatMoveError("invalid destination path: " + err.Error()), nil
	}

	// Check source exists
	srcInfo, err := os.Stat(srcAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return formatMoveError(fmt.Sprintf("source not found: %s", params.Source)), nil
		}
		if errors.Is(err, os.ErrPermission) {
			return formatMoveError(fmt.Sprintf("permission denied: cannot read %s", params.Source)), nil
		}
		return formatMoveError(err.Error()), nil
	}

	// If destination is existing directory, move into it
	if dstInfo, err := os.Stat(dstAbs); err == nil && dstInfo.IsDir() {
		dstAbs = filepath.Join(dstAbs, filepath.Base(srcAbs))
	}

	// Check source and destination are not the same
	if srcAbs == dstAbs {
		return formatMoveError("source and destination are the same"), nil
	}

	// Prevent moving directory into itself
	if srcInfo.IsDir() && strings.HasPrefix(dstAbs, srcAbs+string(os.PathSeparator)) {
		return formatMoveError("cannot move directory into itself"), nil
	}

	// Check if destination exists and is a non-empty directory
	if dstInfo, err := os.Stat(dstAbs); err == nil && dstInfo.IsDir() {
		empty, err := isDirEmpty(dstAbs)
		if err != nil {
			return formatMoveError("failed to check destination: " + err.Error()), nil
		}
		if !empty {
			return formatMoveError(fmt.Sprintf("cannot overwrite non-empty directory: %s", params.Destination)), nil
		}
		// Remove empty directory before move
		if err := os.Remove(dstAbs); err != nil {
			return formatMoveError("failed to remove empty directory: " + err.Error()), nil
		}
	}

	// Create parent directories
	dstDir := filepath.Dir(dstAbs)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return formatMoveError(fmt.Sprintf("permission denied: cannot write to %s", dstDir)), nil
		}
		return formatMoveError("cannot create directory: " + err.Error()), nil
	}

	// Attempt rename (works for same filesystem)
	if err := os.Rename(srcAbs, dstAbs); err != nil {
		// Cross-filesystem: copy then delete
		if linkErr, ok := err.(*os.LinkError); ok && linkErr.Err.Error() == "cross-device link" {
			if err := copyRecursive(srcAbs, dstAbs, srcInfo); err != nil {
				return formatMoveError("failed to copy: " + err.Error()), nil
			}
			if err := os.RemoveAll(srcAbs); err != nil {
				return formatMoveError("failed to remove source after copy: " + err.Error()), nil
			}
		} else {
			if errors.Is(err, os.ErrPermission) {
				return formatMoveError(fmt.Sprintf("permission denied: cannot write to %s", params.Destination)), nil
			}
			return formatMoveError("failed to move: " + err.Error()), nil
		}
	}

	return formatMoveSuccess(srcAbs, dstAbs), nil
}

// isDirEmpty checks if a directory is empty.
func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// copyRecursive copies a file or directory recursively.
func copyRecursive(src, dst string, srcInfo os.FileInfo) error {
	if srcInfo.IsDir() {
		return copyDir(src, dst, srcInfo)
	}
	return copyFile(src, dst, srcInfo)
}

// copyDir copies a directory recursively.
func copyDir(src, dst string, srcInfo os.FileInfo) error {
	// Create destination directory with same permissions
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if err := copyRecursive(srcPath, dstPath, info); err != nil {
			return err
		}
	}

	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string, srcInfo os.FileInfo) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// formatMoveSuccess formats a successful move response.
func formatMoveSuccess(source, destination string) string {
	output := moveOutput{
		Source:      source,
		Destination: destination,
	}
	data, _ := json.Marshal(output)
	return string(data)
}

// formatMoveError formats an error response.
func formatMoveError(msg string) string {
	output := moveError{Error: msg}
	data, _ := json.Marshal(output)
	return string(data)
}
