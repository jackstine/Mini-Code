package log

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// AgentLogger defines the interface for logging agent interactions.
type AgentLogger interface {
	// LogUser logs a user prompt.
	LogUser(content string)
	// LogAssistant logs an assistant response.
	LogAssistant(content string)
	// LogToolCall logs a tool call from the assistant.
	LogToolCall(id, name string, input json.RawMessage)
	// LogToolResult logs a tool execution result.
	LogToolResult(id string, result string, isError bool)
	// Close closes the agent logger and any open files.
	Close() error
}

// agentLogger is the concrete implementation of AgentLogger.
type agentLogger struct {
	mu      sync.Mutex
	config  AgentLogConfig
	writer  *rotatingWriter
	format  Format
}

// NewAgentLogger creates a new AgentLogger with the given configuration.
// Returns nil if FilePath is empty (disabled).
func NewAgentLogger(config AgentLogConfig) AgentLogger {
	if config.FilePath == "" {
		return nil
	}

	writer, err := newRotatingWriter(config.FilePath, config.MaxSize, config.MaxFiles)
	if err != nil {
		// If we can't create the file, return nil (disabled)
		return nil
	}

	return &agentLogger{
		config: config,
		writer: writer,
		format: config.Format,
	}
}

// LogUser logs a user prompt.
func (l *agentLogger) LogUser(content string) {
	l.log("user", "", "", content, nil, false)
}

// LogAssistant logs an assistant response.
func (l *agentLogger) LogAssistant(content string) {
	l.log("assistant", "", "", content, nil, false)
}

// LogToolCall logs a tool call from the assistant.
func (l *agentLogger) LogToolCall(id, name string, input json.RawMessage) {
	l.log("tool_call", id, name, "", input, false)
}

// LogToolResult logs a tool execution result.
func (l *agentLogger) LogToolResult(id string, result string, isError bool) {
	l.log("tool_result", id, "", result, nil, isError)
}

// Close closes the agent logger.
func (l *agentLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.writer != nil {
		return l.writer.Close()
	}
	return nil
}

// log writes a log entry.
func (l *agentLogger) log(eventType, id, name, content string, input json.RawMessage, isError bool) {
	timestamp := time.Now().UTC()

	var output string
	if l.format == FormatJSON {
		output = l.formatJSON(timestamp, eventType, id, name, content, input, isError)
	} else {
		output = l.formatText(timestamp, eventType, id, name, content, input, isError)
	}

	l.mu.Lock()
	io.WriteString(l.writer, output)
	l.mu.Unlock()
}

// formatText formats an agent log entry as text.
func (l *agentLogger) formatText(timestamp time.Time, eventType, id, name, content string, input json.RawMessage, isError bool) string {
	ts := timestamp.Format(time.RFC3339Nano)

	switch eventType {
	case "user":
		return fmt.Sprintf("=== %s USER ===\n%s\n\n", ts, content)
	case "assistant":
		return fmt.Sprintf("=== %s ASSISTANT ===\n%s\n\n", ts, content)
	case "tool_call":
		return fmt.Sprintf("=== %s TOOL_CALL [%s] id=%s ===\n%s\n\n", ts, name, id, string(input))
	case "tool_result":
		status := "success"
		if isError {
			status = "error"
		}
		return fmt.Sprintf("=== %s TOOL_RESULT [%s] %s ===\n%s\n\n", ts, id, status, content)
	default:
		return ""
	}
}

// formatJSON formats an agent log entry as JSON (NDJSON).
func (l *agentLogger) formatJSON(timestamp time.Time, eventType, id, name, content string, input json.RawMessage, isError bool) string {
	entry := map[string]any{
		"timestamp": timestamp.Format(time.RFC3339Nano),
		"type":      eventType,
	}

	switch eventType {
	case "user", "assistant":
		entry["content"] = content
	case "tool_call":
		entry["id"] = id
		entry["name"] = name
		// Parse input to include as object, not string
		var inputObj any
		if json.Unmarshal(input, &inputObj) == nil {
			entry["input"] = inputObj
		} else {
			entry["input"] = string(input)
		}
	case "tool_result":
		entry["id"] = id
		entry["success"] = !isError
		entry["result"] = content
	}

	data, _ := json.Marshal(entry)
	return string(data) + "\n"
}

// NopAgentLogger is an agent logger that does nothing. Useful for testing.
type NopAgentLogger struct{}

func (NopAgentLogger) LogUser(content string)                                  {}
func (NopAgentLogger) LogAssistant(content string)                             {}
func (NopAgentLogger) LogToolCall(id, name string, input json.RawMessage)      {}
func (NopAgentLogger) LogToolResult(id string, result string, isError bool)    {}
func (NopAgentLogger) Close() error                                            { return nil }
