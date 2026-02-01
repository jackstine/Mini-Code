package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// rotatingWriter handles file rotation for agent logs.
type rotatingWriter struct {
	filePath string
	maxSize  int64
	maxFiles int
	file     *os.File
	size     int64
}

// newRotatingWriter creates a new rotating writer.
func newRotatingWriter(filePath string, maxSize int64, maxFiles int) (*rotatingWriter, error) {
	// Apply defaults
	if maxSize <= 0 {
		maxSize = DefaultMaxSize
	}
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFiles
	}

	rw := &rotatingWriter{
		filePath: filePath,
		maxSize:  maxSize,
		maxFiles: maxFiles,
	}

	// Open the file
	if err := rw.openFile(); err != nil {
		return nil, err
	}

	return rw, nil
}

// Write writes data to the file, rotating if necessary.
func (rw *rotatingWriter) Write(p []byte) (n int, err error) {
	// Check if we need to rotate before writing
	if rw.size+int64(len(p)) > rw.maxSize {
		if err := rw.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = rw.file.Write(p)
	rw.size += int64(n)
	return n, err
}

// Close closes the file.
func (rw *rotatingWriter) Close() error {
	if rw.file != nil {
		return rw.file.Close()
	}
	return nil
}

// openFile opens the log file, creating directories if necessary.
func (rw *rotatingWriter) openFile() error {
	// Ensure directory exists
	dir := filepath.Dir(rw.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file for appending
	file, err := os.OpenFile(rw.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	rw.file = file
	rw.size = info.Size()
	return nil
}

// rotate rotates the current log file.
func (rw *rotatingWriter) rotate() error {
	// Close current file
	if rw.file != nil {
		rw.file.Close()
	}

	// Generate new filename with timestamp
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05")
	rotatedPath := fmt.Sprintf("%s.%s", rw.filePath, timestamp)

	// Rename current file
	if err := os.Rename(rw.filePath, rotatedPath); err != nil && !os.IsNotExist(err) {
		// If rename fails and it's not because file doesn't exist, return error
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	// Clean up old files
	if err := rw.cleanup(); err != nil {
		// Log but don't fail on cleanup errors
		// (In a real system we'd use the logger here, but we're the logger!)
	}

	// Open new file
	return rw.openFile()
}

// cleanup removes old rotated files if there are too many.
func (rw *rotatingWriter) cleanup() error {
	dir := filepath.Dir(rw.filePath)
	base := filepath.Base(rw.filePath)

	// Find all rotated files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var rotatedFiles []string
	for _, entry := range entries {
		name := entry.Name()
		// Match files like "agent.log.2024-01-15T10-00-00"
		if strings.HasPrefix(name, base+".") && name != base {
			rotatedFiles = append(rotatedFiles, filepath.Join(dir, name))
		}
	}

	// If we have more than maxFiles rotated files, delete oldest
	if len(rotatedFiles) > rw.maxFiles {
		// Sort by name (timestamp suffix makes this chronological)
		sort.Strings(rotatedFiles)

		// Delete oldest files
		toDelete := len(rotatedFiles) - rw.maxFiles
		for i := 0; i < toDelete; i++ {
			os.Remove(rotatedFiles[i])
		}
	}

	return nil
}
