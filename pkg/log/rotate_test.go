package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatingWriterCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	rw, err := newRotatingWriter(logPath, DefaultMaxSize, DefaultMaxFiles)
	if err != nil {
		t.Fatalf("failed to create rotating writer: %v", err)
	}
	defer rw.Close()

	// File should exist
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created")
	}
}

func TestRotatingWriterCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "subdir", "nested", "test.log")

	rw, err := newRotatingWriter(logPath, DefaultMaxSize, DefaultMaxFiles)
	if err != nil {
		t.Fatalf("failed to create rotating writer: %v", err)
	}
	defer rw.Close()

	// File should exist
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created in nested directory")
	}
}

func TestRotatingWriterWrites(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	rw, err := newRotatingWriter(logPath, DefaultMaxSize, DefaultMaxFiles)
	if err != nil {
		t.Fatalf("failed to create rotating writer: %v", err)
	}

	// Write data
	data := []byte("hello world\n")
	n, err := rw.Write(data)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}

	rw.Close()

	// Read back
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestRotatingWriterRotates(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Use small max size to trigger rotation
	maxSize := int64(100)
	rw, err := newRotatingWriter(logPath, maxSize, 5)
	if err != nil {
		t.Fatalf("failed to create rotating writer: %v", err)
	}

	// Write data to exceed max size
	data := make([]byte, 50)
	for i := range data {
		data[i] = 'a'
	}
	data = append(data, '\n')

	// Write twice - second write should trigger rotation
	rw.Write(data)
	rw.Write(data)
	rw.Write(data) // This should rotate

	rw.Close()

	// Check for rotated files
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	var logFiles []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "test.log") {
			logFiles = append(logFiles, entry.Name())
		}
	}

	// Should have current file and at least one rotated file
	if len(logFiles) < 2 {
		t.Errorf("expected at least 2 log files after rotation, got %d: %v", len(logFiles), logFiles)
	}
}

func TestRotatingWriterCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Use very small max size and max files
	maxSize := int64(50)
	maxFiles := 2
	rw, err := newRotatingWriter(logPath, maxSize, maxFiles)
	if err != nil {
		t.Fatalf("failed to create rotating writer: %v", err)
	}

	// Write enough to trigger multiple rotations
	data := make([]byte, 60)
	for i := range data {
		data[i] = 'a'
	}

	// Write 5 times to trigger several rotations
	for i := 0; i < 5; i++ {
		rw.Write(data)
	}

	rw.Close()

	// Count rotated files
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	var rotatedFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "test.log.") {
			rotatedFiles = append(rotatedFiles, name)
		}
	}

	// Should have at most maxFiles rotated files
	if len(rotatedFiles) > maxFiles {
		t.Errorf("expected at most %d rotated files, got %d: %v", maxFiles, len(rotatedFiles), rotatedFiles)
	}
}

func TestRotatingWriterDefaultValues(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Pass 0 for maxSize and maxFiles to test defaults
	rw, err := newRotatingWriter(logPath, 0, 0)
	if err != nil {
		t.Fatalf("failed to create rotating writer: %v", err)
	}
	defer rw.Close()

	if rw.maxSize != DefaultMaxSize {
		t.Errorf("expected default maxSize %d, got %d", DefaultMaxSize, rw.maxSize)
	}
	if rw.maxFiles != DefaultMaxFiles {
		t.Errorf("expected default maxFiles %d, got %d", DefaultMaxFiles, rw.maxFiles)
	}
}
