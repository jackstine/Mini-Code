package harness_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/user/harness/pkg/harness"
	"github.com/user/harness/pkg/testutil"
	"github.com/user/harness/pkg/tool"
)

// MockEventHandler records all events for testing.
type MockEventHandler struct {
	mu              sync.Mutex
	TextEvents      []string
	ToolCalls       []struct{ ID, Name string; Input json.RawMessage }
	ToolResults     []struct{ ID, Result string; IsError bool }
	ReasoningEvents []string
}

func (h *MockEventHandler) OnText(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.TextEvents = append(h.TextEvents, text)
}

func (h *MockEventHandler) OnToolCall(id string, name string, input json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolCalls = append(h.ToolCalls, struct{ ID, Name string; Input json.RawMessage }{id, name, input})
}

func (h *MockEventHandler) OnToolResult(id string, result string, isError bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolResults = append(h.ToolResults, struct{ ID, Result string; IsError bool }{id, result, isError})
}

func (h *MockEventHandler) OnReasoning(content string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ReasoningEvents = append(h.ReasoningEvents, content)
}

// MockTool is a simple tool for testing.
type MockTool struct {
	name        string
	description string
	executeFunc func(ctx context.Context, input json.RawMessage) (string, error)
}

func (t *MockTool) Name() string { return t.name }
func (t *MockTool) Description() string { return t.description }
func (t *MockTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}}}`)
}
func (t *MockTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	if t.executeFunc != nil {
		return t.executeFunc(ctx, input)
	}
	return `{"result":"mock result"}`, nil
}

// TestIntegration_TextOnlyResponse tests that a simple text response
// terminates the loop after one turn.
func TestIntegration_TextOnlyResponse(t *testing.T) {
	// Setup mock streamer
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Hello, I am Claude!"))

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		nil,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Hi!")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify events
	if len(handler.TextEvents) != 1 {
		t.Errorf("expected 1 text event, got %d", len(handler.TextEvents))
	}
	if len(handler.TextEvents) > 0 && handler.TextEvents[0] != "Hello, I am Claude!" {
		t.Errorf("expected text 'Hello, I am Claude!', got %q", handler.TextEvents[0])
	}

	// Verify only one API call was made
	if len(mockStreamer.RecordedParams) != 1 {
		t.Errorf("expected 1 API call, got %d", len(mockStreamer.RecordedParams))
	}
}

// TestIntegration_SingleToolCall tests that a single tool call is executed
// and the result is sent back to the model.
func TestIntegration_SingleToolCall(t *testing.T) {
	// Setup mock streamer with two responses:
	// 1. Tool call response
	// 2. Final text response
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"test_tool",
		map[string]string{"value": "test input"},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Tool result received!"))

	// Create mock tool
	tools := []tool.Tool{
		&MockTool{
			name:        "test_tool",
			description: "A test tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				return `{"output": "tool executed"}`, nil
			},
		},
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Use the tool")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify tool call event
	if len(handler.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(handler.ToolCalls))
	}
	if len(handler.ToolCalls) > 0 {
		if handler.ToolCalls[0].Name != "test_tool" {
			t.Errorf("expected tool name 'test_tool', got %q", handler.ToolCalls[0].Name)
		}
	}

	// Verify tool result event
	if len(handler.ToolResults) != 1 {
		t.Errorf("expected 1 tool result, got %d", len(handler.ToolResults))
	}
	if len(handler.ToolResults) > 0 {
		if handler.ToolResults[0].IsError {
			t.Error("expected tool result to not be an error")
		}
	}

	// Verify final text response
	if len(handler.TextEvents) != 1 {
		t.Errorf("expected 1 text event, got %d", len(handler.TextEvents))
	}

	// Verify two API calls were made (tool call + final response)
	if len(mockStreamer.RecordedParams) != 2 {
		t.Errorf("expected 2 API calls, got %d", len(mockStreamer.RecordedParams))
	}
}

// TestIntegration_ToolCallFailFast tests that when a tool fails,
// subsequent tools are not executed.
func TestIntegration_ToolCallFailFast(t *testing.T) {
	// Setup mock streamer
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.MultiToolResponse([]struct{ ID, Name string; Input any }{
		{ID: "tool_1", Name: "failing_tool", Input: map[string]string{}},
		{ID: "tool_2", Name: "second_tool", Input: map[string]string{}},
	}))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Handled the error"))

	// Track tool execution
	executedTools := []string{}
	mu := sync.Mutex{}

	// Create mock tools
	tools := []tool.Tool{
		&MockTool{
			name:        "failing_tool",
			description: "A tool that fails",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				mu.Lock()
				executedTools = append(executedTools, "failing_tool")
				mu.Unlock()
				return "", errors.New("tool execution failed")
			},
		},
		&MockTool{
			name:        "second_tool",
			description: "Second tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				mu.Lock()
				executedTools = append(executedTools, "second_tool")
				mu.Unlock()
				return `{"success": true}`, nil
			},
		},
	}

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Execute both tools")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify only the first tool was executed (fail-fast)
	mu.Lock()
	if len(executedTools) != 1 || executedTools[0] != "failing_tool" {
		t.Errorf("expected only 'failing_tool' to be executed, got %v", executedTools)
	}
	mu.Unlock()

	// Verify tool result shows error
	if len(handler.ToolResults) != 1 {
		t.Errorf("expected 1 tool result, got %d", len(handler.ToolResults))
	}
	if len(handler.ToolResults) > 0 && !handler.ToolResults[0].IsError {
		t.Error("expected tool result to be an error")
	}
}

// TestIntegration_MaxTurnsEnforced tests that the loop terminates
// when MaxTurns is reached.
func TestIntegration_MaxTurnsEnforced(t *testing.T) {
	// Setup mock streamer that always returns tool calls
	mockStreamer := testutil.NewMockMessageStreamer()
	// Add 5 tool call responses (more than MaxTurns)
	for i := 0; i < 5; i++ {
		mockStreamer.AddResponse(testutil.SingleToolResponse(
			"tool_1",
			"infinite_tool",
			map[string]string{},
		))
	}

	// Create mock tool
	tools := []tool.Tool{
		&MockTool{
			name:        "infinite_tool",
			description: "Tool that keeps being called",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				return `{"continue": true}`, nil
			},
		},
	}

	// Create harness with MaxTurns = 2
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model", MaxTurns: 2},
		tools,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Keep going")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify exactly 2 API calls were made (MaxTurns = 2)
	if len(mockStreamer.RecordedParams) != 2 {
		t.Errorf("expected 2 API calls (MaxTurns), got %d", len(mockStreamer.RecordedParams))
	}
}

// TestIntegration_ThinkingBlock tests that thinking blocks emit reasoning events.
func TestIntegration_ThinkingBlock(t *testing.T) {
	// Setup mock streamer with thinking + text response
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.ThinkingResponse(
		"Let me think about this...",
		"Here is my answer.",
	))

	// Create harness with mock
	handler := &MockEventHandler{}
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		nil,
		handler,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Think about this")
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}

	// Verify reasoning event
	if len(handler.ReasoningEvents) != 1 {
		t.Errorf("expected 1 reasoning event, got %d", len(handler.ReasoningEvents))
	}
	if len(handler.ReasoningEvents) > 0 && handler.ReasoningEvents[0] != "Let me think about this..." {
		t.Errorf("expected reasoning 'Let me think about this...', got %q", handler.ReasoningEvents[0])
	}

	// Verify text event
	if len(handler.TextEvents) != 1 {
		t.Errorf("expected 1 text event, got %d", len(handler.TextEvents))
	}
}

// TestIntegration_ContextCancellation tests that context cancellation
// stops the agent loop.
func TestIntegration_ContextCancellation(t *testing.T) {
	// Setup mock streamer
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"slow_tool",
		map[string]string{},
	))

	// Create mock tool that checks context
	tools := []tool.Tool{
		&MockTool{
			name:        "slow_tool",
			description: "A slow tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				// Simulate the context being checked
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				default:
					return `{"done": true}`, nil
				}
			},
		},
	}

	// Create harness with mock
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		nil,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Run prompt with cancelled context
	err = h.Prompt(ctx, "Do something slow")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

// TestIntegration_StreamError tests that stream errors are properly propagated.
func TestIntegration_StreamError(t *testing.T) {
	// Setup mock streamer that returns an error
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.ErrorResponse(errors.New("API rate limit exceeded")))

	// Create harness with mock
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		nil,
		nil,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// Run prompt
	err = h.Prompt(context.Background(), "Hello")
	if err == nil {
		t.Error("expected error from stream")
	}
	if err != nil && err.Error() != "API rate limit exceeded" {
		t.Errorf("expected 'API rate limit exceeded' error, got %v", err)
	}
}

// TestIntegration_ConversationHistory tests that messages are properly
// accumulated in conversation history.
func TestIntegration_ConversationHistory(t *testing.T) {
	// Setup mock streamer
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.TextOnlyResponse("First response"))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Second response"))

	// Create harness with mock
	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		nil,
		nil,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	// First prompt
	err = h.Prompt(context.Background(), "First message")
	if err != nil {
		t.Fatalf("first prompt failed: %v", err)
	}

	// Check messages after first prompt
	msgs := h.Messages()
	if len(msgs) != 2 { // user + assistant
		t.Errorf("expected 2 messages after first prompt, got %d", len(msgs))
	}

	// Second prompt
	err = h.Prompt(context.Background(), "Second message")
	if err != nil {
		t.Fatalf("second prompt failed: %v", err)
	}

	// Check messages after second prompt
	msgs = h.Messages()
	if len(msgs) != 4 { // user + assistant + user + assistant
		t.Errorf("expected 4 messages after second prompt, got %d", len(msgs))
	}
}
