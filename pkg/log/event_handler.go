package log

import (
	"encoding/json"
)

// EventHandler defines the interface that the harness uses for events.
// This is a copy of harness.EventHandler to avoid import cycles.
type EventHandler interface {
	OnText(text string)
	OnToolCall(id string, name string, input json.RawMessage)
	OnToolResult(id string, result string, isError bool)
	OnReasoning(content string)
}

// LoggingEventHandler wraps an EventHandler and logs agent interactions.
// It delegates all events to the wrapped handler while also logging them
// to an AgentLogger for debugging and review purposes.
type LoggingEventHandler struct {
	wrapped     EventHandler
	agentLogger AgentLogger
}

// NewLoggingEventHandler creates a new LoggingEventHandler.
// If agentLogger is nil, events are only passed to the wrapped handler.
// If wrapped is nil, events are only logged.
func NewLoggingEventHandler(wrapped EventHandler, agentLogger AgentLogger) *LoggingEventHandler {
	return &LoggingEventHandler{
		wrapped:     wrapped,
		agentLogger: agentLogger,
	}
}

// OnText handles assistant text events.
func (h *LoggingEventHandler) OnText(text string) {
	if h.agentLogger != nil {
		h.agentLogger.LogAssistant(text)
	}
	if h.wrapped != nil {
		h.wrapped.OnText(text)
	}
}

// OnToolCall handles tool call events from the assistant.
func (h *LoggingEventHandler) OnToolCall(id string, name string, input json.RawMessage) {
	if h.agentLogger != nil {
		h.agentLogger.LogToolCall(id, name, input)
	}
	if h.wrapped != nil {
		h.wrapped.OnToolCall(id, name, input)
	}
}

// OnToolResult handles tool result events.
func (h *LoggingEventHandler) OnToolResult(id string, result string, isError bool) {
	if h.agentLogger != nil {
		h.agentLogger.LogToolResult(id, result, isError)
	}
	if h.wrapped != nil {
		h.wrapped.OnToolResult(id, result, isError)
	}
}

// OnReasoning handles reasoning/thinking events from the assistant.
func (h *LoggingEventHandler) OnReasoning(content string) {
	// Agent logger doesn't capture reasoning (it's an internal thinking process)
	// Only forward to wrapped handler
	if h.wrapped != nil {
		h.wrapped.OnReasoning(content)
	}
}

// LogUserPrompt logs a user prompt to the agent logger.
// This should be called when a user submits a prompt, before the harness processes it.
func (h *LoggingEventHandler) LogUserPrompt(content string) {
	if h.agentLogger != nil {
		h.agentLogger.LogUser(content)
	}
}
