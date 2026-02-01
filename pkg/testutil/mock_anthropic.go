// Package testutil provides testing utilities for the harness package.
package testutil

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/user/harness/pkg/harness"
)

// MockStreamWithMessage implements harness.StreamIterator and provides
// a pre-built message for testing.
type MockStreamWithMessage struct {
	events  []anthropic.MessageStreamEventUnion
	index   int
	current anthropic.MessageStreamEventUnion
	err     error
}

// createContentBlockStartEvent creates a properly formatted ContentBlockStartEvent
// by marshaling to JSON and unmarshaling back, which populates the RawJSON field.
func createContentBlockStartEvent(index int64, block anthropic.ContentBlockUnion) (anthropic.MessageStreamEventUnion, error) {
	// Build raw JSON for the content block based on its type
	var contentBlockJSON []byte
	var err error

	switch block.Type {
	case "text":
		contentBlockJSON, err = json.Marshal(map[string]any{
			"type": "text",
			"text": block.Text,
		})
	case "tool_use":
		// Need to handle Input which is json.RawMessage
		var inputAny any
		if len(block.Input) > 0 {
			json.Unmarshal(block.Input, &inputAny)
		} else {
			inputAny = map[string]any{}
		}
		contentBlockJSON, err = json.Marshal(map[string]any{
			"type":  "tool_use",
			"id":    block.ID,
			"name":  block.Name,
			"input": inputAny,
		})
	case "thinking":
		contentBlockJSON, err = json.Marshal(map[string]any{
			"type":     "thinking",
			"thinking": block.Thinking,
		})
	default:
		return anthropic.MessageStreamEventUnion{}, fmt.Errorf("unsupported block type: %s", block.Type)
	}

	if err != nil {
		return anthropic.MessageStreamEventUnion{}, err
	}

	// Build the complete event JSON
	eventJSON, err := json.Marshal(map[string]any{
		"type":          "content_block_start",
		"index":         index,
		"content_block": json.RawMessage(contentBlockJSON),
	})
	if err != nil {
		return anthropic.MessageStreamEventUnion{}, err
	}

	// Unmarshal to get proper struct with RawJSON populated
	var event anthropic.MessageStreamEventUnion
	if err := json.Unmarshal(eventJSON, &event); err != nil {
		return anthropic.MessageStreamEventUnion{}, err
	}

	return event, nil
}

// NewMockStreamWithMessage creates a stream that returns events
// which, when processed, will accumulate into the provided message.
func NewMockStreamWithMessage(msg anthropic.Message) *MockStreamWithMessage {
	var events []anthropic.MessageStreamEventUnion

	// 1. MessageStartEvent - initialize the message (with empty content)
	msgStartJSON, _ := json.Marshal(map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":          msg.ID,
			"type":        "message",
			"role":        "assistant",
			"content":     []any{},
			"stop_reason": msg.StopReason,
		},
	})
	var msgStartEvent anthropic.MessageStreamEventUnion
	json.Unmarshal(msgStartJSON, &msgStartEvent)
	events = append(events, msgStartEvent)

	// 2. For each content block, emit ContentBlockStartEvent and ContentBlockStopEvent
	for i, block := range msg.Content {
		// ContentBlockStartEvent
		startEvent, err := createContentBlockStartEvent(int64(i), block)
		if err != nil {
			panic(fmt.Sprintf("failed to create content block start event: %v", err))
		}
		events = append(events, startEvent)

		// ContentBlockStopEvent
		stopJSON, _ := json.Marshal(map[string]any{
			"type":  "content_block_stop",
			"index": i,
		})
		var stopEvent anthropic.MessageStreamEventUnion
		json.Unmarshal(stopJSON, &stopEvent)
		events = append(events, stopEvent)
	}

	// 3. MessageStopEvent
	stopJSON, _ := json.Marshal(map[string]any{
		"type": "message_stop",
	})
	var msgStopEvent anthropic.MessageStreamEventUnion
	json.Unmarshal(stopJSON, &msgStopEvent)
	events = append(events, msgStopEvent)

	return &MockStreamWithMessage{
		events: events,
		index:  -1,
	}
}

// NewMockStreamWithError creates a stream that returns an error.
func NewMockStreamWithError(err error) *MockStreamWithMessage {
	return &MockStreamWithMessage{
		err:   err,
		index: -1,
	}
}

func (m *MockStreamWithMessage) Next() bool {
	m.index++
	if m.index < len(m.events) {
		m.current = m.events[m.index]
		return true
	}
	return false
}

func (m *MockStreamWithMessage) Current() anthropic.MessageStreamEventUnion {
	return m.current
}

func (m *MockStreamWithMessage) Err() error {
	return m.err
}

// MockMessageStreamer implements harness.MessageStreamer for testing.
type MockMessageStreamer struct {
	// Responses is a queue of streams to return from NewStreaming.
	Responses []harness.StreamIterator

	// RecordedParams stores all params passed to NewStreaming.
	RecordedParams []anthropic.MessageNewParams

	// currentIndex tracks which response to return next.
	currentIndex int
}

// NewMockMessageStreamer creates a new mock message streamer.
func NewMockMessageStreamer() *MockMessageStreamer {
	return &MockMessageStreamer{
		Responses:      []harness.StreamIterator{},
		RecordedParams: []anthropic.MessageNewParams{},
	}
}

// NewStreaming returns the next configured stream response.
func (m *MockMessageStreamer) NewStreaming(ctx context.Context, params anthropic.MessageNewParams) harness.StreamIterator {
	m.RecordedParams = append(m.RecordedParams, params)

	if m.currentIndex >= len(m.Responses) {
		// Return an empty stream if no more responses configured
		return NewMockStreamWithMessage(anthropic.Message{
			Role:       "assistant",
			Type:       "message",
			StopReason: "end_turn",
		})
	}

	stream := m.Responses[m.currentIndex]
	m.currentIndex++
	return stream
}

// AddResponse adds a mock stream response to the queue.
func (m *MockMessageStreamer) AddResponse(stream harness.StreamIterator) {
	m.Responses = append(m.Responses, stream)
}

// Reset clears all responses and recorded params.
func (m *MockMessageStreamer) Reset() {
	m.Responses = []harness.StreamIterator{}
	m.RecordedParams = []anthropic.MessageNewParams{}
	m.currentIndex = 0
}

// MessageBuilder provides a fluent API for building mock messages.
type MessageBuilder struct {
	content []anthropic.ContentBlockUnion
}

// NewMessageBuilder creates a new MessageBuilder.
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		content: []anthropic.ContentBlockUnion{},
	}
}

// AddText adds a text block to the message.
func (mb *MessageBuilder) AddText(text string) *MessageBuilder {
	mb.content = append(mb.content, anthropic.ContentBlockUnion{
		Type: "text",
		Text: text,
	})
	return mb
}

// AddToolUse adds a tool use block to the message.
func (mb *MessageBuilder) AddToolUse(id, name string, input any) *MessageBuilder {
	// Marshal input to json.RawMessage
	inputJSON, err := json.Marshal(input)
	if err != nil {
		panic("failed to marshal tool input: " + err.Error())
	}
	mb.content = append(mb.content, anthropic.ContentBlockUnion{
		Type:  "tool_use",
		ID:    id,
		Name:  name,
		Input: inputJSON,
	})
	return mb
}

// AddThinking adds a thinking block to the message.
func (mb *MessageBuilder) AddThinking(thinking string) *MessageBuilder {
	mb.content = append(mb.content, anthropic.ContentBlockUnion{
		Type:     "thinking",
		Thinking: thinking,
	})
	return mb
}

// Build returns a MockStreamWithMessage that contains the built message.
func (mb *MessageBuilder) Build() *MockStreamWithMessage {
	return mb.BuildWithStopReason(anthropic.StopReasonEndTurn)
}

// BuildWithToolUse returns a MockStreamWithMessage with tool_use stop reason.
func (mb *MessageBuilder) BuildWithToolUse() *MockStreamWithMessage {
	return mb.BuildWithStopReason(anthropic.StopReasonToolUse)
}

// BuildWithStopReason returns a MockStreamWithMessage with the specified stop reason.
func (mb *MessageBuilder) BuildWithStopReason(stopReason anthropic.StopReason) *MockStreamWithMessage {
	msg := anthropic.Message{
		ID:         "msg_test",
		Type:       "message",
		Role:       "assistant",
		Content:    mb.content,
		StopReason: stopReason,
	}
	return NewMockStreamWithMessage(msg)
}

// Preset fixtures for common test scenarios

// TextOnlyResponse creates a stream that returns a single text response.
func TextOnlyResponse(text string) *MockStreamWithMessage {
	return NewMessageBuilder().AddText(text).Build()
}

// SingleToolResponse creates a stream with one tool call.
func SingleToolResponse(toolID, toolName string, input any) *MockStreamWithMessage {
	return NewMessageBuilder().AddToolUse(toolID, toolName, input).BuildWithToolUse()
}

// TextAndToolResponse creates a stream with text followed by a tool call.
func TextAndToolResponse(text, toolID, toolName string, input any) *MockStreamWithMessage {
	return NewMessageBuilder().
		AddText(text).
		AddToolUse(toolID, toolName, input).
		BuildWithToolUse()
}

// MultiToolResponse creates a stream with multiple tool calls.
func MultiToolResponse(tools []struct{ ID, Name string; Input any }) *MockStreamWithMessage {
	mb := NewMessageBuilder()
	for _, tool := range tools {
		mb.AddToolUse(tool.ID, tool.Name, tool.Input)
	}
	return mb.BuildWithToolUse()
}

// ThinkingResponse creates a stream with a thinking block followed by text.
func ThinkingResponse(thinking, text string) *MockStreamWithMessage {
	return NewMessageBuilder().
		AddThinking(thinking).
		AddText(text).
		Build()
}

// ErrorResponse creates a stream that will return an error.
func ErrorResponse(err error) *MockStreamWithMessage {
	return NewMockStreamWithError(err)
}

// Helper function to marshal input to json.RawMessage
func MustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
