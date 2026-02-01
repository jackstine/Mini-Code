package log

import (
	"encoding/json"
	"testing"
)

// mockEventHandler records calls for testing
type mockEventHandler struct {
	textCalls      []string
	toolCallCalls  []toolCallRecord
	toolResultCalls []toolResultRecord
	reasoningCalls []string
}

type toolCallRecord struct {
	id    string
	name  string
	input json.RawMessage
}

type toolResultRecord struct {
	id      string
	result  string
	isError bool
}

func (m *mockEventHandler) OnText(text string) {
	m.textCalls = append(m.textCalls, text)
}

func (m *mockEventHandler) OnToolCall(id string, name string, input json.RawMessage) {
	m.toolCallCalls = append(m.toolCallCalls, toolCallRecord{id, name, input})
}

func (m *mockEventHandler) OnToolResult(id string, result string, isError bool) {
	m.toolResultCalls = append(m.toolResultCalls, toolResultRecord{id, result, isError})
}

func (m *mockEventHandler) OnReasoning(content string) {
	m.reasoningCalls = append(m.reasoningCalls, content)
}

// mockAgentLogger records calls for testing
type mockAgentLogger struct {
	userCalls       []string
	assistantCalls  []string
	toolCallCalls   []toolCallRecord
	toolResultCalls []toolResultRecord
}

func (m *mockAgentLogger) LogUser(content string) {
	m.userCalls = append(m.userCalls, content)
}

func (m *mockAgentLogger) LogAssistant(content string) {
	m.assistantCalls = append(m.assistantCalls, content)
}

func (m *mockAgentLogger) LogToolCall(id, name string, input json.RawMessage) {
	m.toolCallCalls = append(m.toolCallCalls, toolCallRecord{id, name, input})
}

func (m *mockAgentLogger) LogToolResult(id string, result string, isError bool) {
	m.toolResultCalls = append(m.toolResultCalls, toolResultRecord{id, result, isError})
}

func (m *mockAgentLogger) Close() error {
	return nil
}

func TestLoggingEventHandlerDelegates(t *testing.T) {
	wrapped := &mockEventHandler{}
	agentLogger := &mockAgentLogger{}
	handler := NewLoggingEventHandler(wrapped, agentLogger)

	// Test OnText
	handler.OnText("Hello")
	if len(wrapped.textCalls) != 1 || wrapped.textCalls[0] != "Hello" {
		t.Errorf("OnText not delegated: %v", wrapped.textCalls)
	}
	if len(agentLogger.assistantCalls) != 1 || agentLogger.assistantCalls[0] != "Hello" {
		t.Errorf("OnText not logged: %v", agentLogger.assistantCalls)
	}

	// Test OnToolCall
	input := json.RawMessage(`{"path": "/test"}`)
	handler.OnToolCall("id1", "read", input)
	if len(wrapped.toolCallCalls) != 1 {
		t.Errorf("OnToolCall not delegated: %v", wrapped.toolCallCalls)
	}
	if len(agentLogger.toolCallCalls) != 1 {
		t.Errorf("OnToolCall not logged: %v", agentLogger.toolCallCalls)
	}

	// Test OnToolResult
	handler.OnToolResult("id1", "content", false)
	if len(wrapped.toolResultCalls) != 1 {
		t.Errorf("OnToolResult not delegated: %v", wrapped.toolResultCalls)
	}
	if len(agentLogger.toolResultCalls) != 1 {
		t.Errorf("OnToolResult not logged: %v", agentLogger.toolResultCalls)
	}

	// Test OnReasoning (not logged to agent logger)
	handler.OnReasoning("thinking...")
	if len(wrapped.reasoningCalls) != 1 || wrapped.reasoningCalls[0] != "thinking..." {
		t.Errorf("OnReasoning not delegated: %v", wrapped.reasoningCalls)
	}
}

func TestLoggingEventHandlerNilWrapped(t *testing.T) {
	agentLogger := &mockAgentLogger{}
	handler := NewLoggingEventHandler(nil, agentLogger)

	// Should not panic with nil wrapped
	handler.OnText("Hello")
	handler.OnToolCall("id1", "read", json.RawMessage(`{}`))
	handler.OnToolResult("id1", "content", false)
	handler.OnReasoning("thinking...")

	// Agent logger should still receive events
	if len(agentLogger.assistantCalls) != 1 {
		t.Errorf("expected 1 assistant call, got %d", len(agentLogger.assistantCalls))
	}
}

func TestLoggingEventHandlerNilLogger(t *testing.T) {
	wrapped := &mockEventHandler{}
	handler := NewLoggingEventHandler(wrapped, nil)

	// Should not panic with nil logger
	handler.OnText("Hello")
	handler.OnToolCall("id1", "read", json.RawMessage(`{}`))
	handler.OnToolResult("id1", "content", false)
	handler.OnReasoning("thinking...")

	// Wrapped handler should still receive events
	if len(wrapped.textCalls) != 1 {
		t.Errorf("expected 1 text call, got %d", len(wrapped.textCalls))
	}
}

func TestLoggingEventHandlerLogUserPrompt(t *testing.T) {
	agentLogger := &mockAgentLogger{}
	handler := NewLoggingEventHandler(nil, agentLogger)

	handler.LogUserPrompt("What's in the file?")

	if len(agentLogger.userCalls) != 1 || agentLogger.userCalls[0] != "What's in the file?" {
		t.Errorf("LogUserPrompt not logged: %v", agentLogger.userCalls)
	}
}
