package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentLoggerDisabledWhenNoPath(t *testing.T) {
	logger := NewAgentLogger(AgentLogConfig{
		FilePath: "",
	})

	if logger != nil {
		t.Error("expected nil logger when FilePath is empty")
	}
}

func TestAgentLoggerTextFormat(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "agent.log")

	logger := NewAgentLogger(AgentLogConfig{
		FilePath: logPath,
		Format:   FormatText,
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Close()

	// Log events
	logger.LogUser("What's in config.json?")
	logger.LogAssistant("I'll read that file for you.")
	logger.LogToolCall("toolu_123", "read", json.RawMessage(`{"path": "/config.json"}`))
	logger.LogToolResult("toolu_123", "port=8080", false)
	logger.LogToolResult("toolu_456", "file not found", true)

	// Close to flush
	logger.Close()

	// Read file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	output := string(content)

	// Check user message
	if !strings.Contains(output, "USER ===") {
		t.Errorf("expected USER marker in output: %s", output)
	}
	if !strings.Contains(output, "What's in config.json?") {
		t.Errorf("expected user content in output: %s", output)
	}

	// Check assistant message
	if !strings.Contains(output, "ASSISTANT ===") {
		t.Errorf("expected ASSISTANT marker in output: %s", output)
	}
	if !strings.Contains(output, "I'll read that file for you.") {
		t.Errorf("expected assistant content in output: %s", output)
	}

	// Check tool call
	if !strings.Contains(output, "TOOL_CALL [read] id=toolu_123") {
		t.Errorf("expected TOOL_CALL marker in output: %s", output)
	}

	// Check tool result
	if !strings.Contains(output, "TOOL_RESULT [toolu_123] success") {
		t.Errorf("expected success TOOL_RESULT marker in output: %s", output)
	}
	if !strings.Contains(output, "TOOL_RESULT [toolu_456] error") {
		t.Errorf("expected error TOOL_RESULT marker in output: %s", output)
	}
}

func TestAgentLoggerJSONFormat(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "agent.log")

	logger := NewAgentLogger(AgentLogConfig{
		FilePath: logPath,
		Format:   FormatJSON,
	})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Close()

	// Log events
	logger.LogUser("Hello")
	logger.LogAssistant("Hi there")
	logger.LogToolCall("toolu_1", "read", json.RawMessage(`{"path": "/test.txt"}`))
	logger.LogToolResult("toolu_1", "test content", false)

	// Close to flush
	logger.Close()

	// Read file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Parse each line as JSON
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// Check user entry
	var userEntry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &userEntry); err != nil {
		t.Fatalf("failed to parse user entry: %v", err)
	}
	if userEntry["type"] != "user" {
		t.Errorf("expected type user, got %v", userEntry["type"])
	}
	if userEntry["content"] != "Hello" {
		t.Errorf("expected content Hello, got %v", userEntry["content"])
	}

	// Check tool_call entry
	var toolCallEntry map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &toolCallEntry); err != nil {
		t.Fatalf("failed to parse tool_call entry: %v", err)
	}
	if toolCallEntry["type"] != "tool_call" {
		t.Errorf("expected type tool_call, got %v", toolCallEntry["type"])
	}
	if toolCallEntry["name"] != "read" {
		t.Errorf("expected name read, got %v", toolCallEntry["name"])
	}

	// Check tool_result entry
	var toolResultEntry map[string]any
	if err := json.Unmarshal([]byte(lines[3]), &toolResultEntry); err != nil {
		t.Fatalf("failed to parse tool_result entry: %v", err)
	}
	if toolResultEntry["type"] != "tool_result" {
		t.Errorf("expected type tool_result, got %v", toolResultEntry["type"])
	}
	if toolResultEntry["success"] != true {
		t.Errorf("expected success true, got %v", toolResultEntry["success"])
	}
}

func TestNopAgentLogger(t *testing.T) {
	logger := NopAgentLogger{}

	// Should not panic
	logger.LogUser("test")
	logger.LogAssistant("test")
	logger.LogToolCall("id", "name", json.RawMessage(`{}`))
	logger.LogToolResult("id", "result", false)

	if err := logger.Close(); err != nil {
		t.Errorf("NopAgentLogger.Close() should return nil, got %v", err)
	}
}
