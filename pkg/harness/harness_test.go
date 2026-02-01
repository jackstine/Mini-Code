package harness

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/user/harness/pkg/tool"
)

// MockEventHandler records all events for testing.
type MockEventHandler struct {
	mu          sync.Mutex
	TextEvents  []string
	ToolCalls   []struct{ ID, Name string; Input json.RawMessage }
	ToolResults []struct{ ID, Result string; IsError bool }
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

func TestNewHarness_RequiresAPIKey(t *testing.T) {
	_, err := NewHarness(Config{}, nil, nil)
	if err == nil {
		t.Error("expected error for missing APIKey")
	}
}

func TestNewHarness_ValidConfig(t *testing.T) {
	h, err := NewHarness(Config{APIKey: "test-key"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("expected harness to be non-nil")
	}
}

func TestNewHarness_WithTools(t *testing.T) {
	tools := []tool.Tool{
		&MockTool{name: "test_tool", description: "A test tool"},
	}
	h, err := NewHarness(Config{APIKey: "test-key"}, tools, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h.tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(h.tools))
	}
	if _, ok := h.tools["test_tool"]; !ok {
		t.Error("tool 'test_tool' should be registered")
	}
}

func TestNewHarness_WithNilHandler(t *testing.T) {
	// Nil handler should be valid
	h, err := NewHarness(Config{APIKey: "test-key"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.handler != nil {
		t.Error("handler should be nil when not provided")
	}
}

func TestNewHarness_WithHandler(t *testing.T) {
	handler := &MockEventHandler{}
	h, err := NewHarness(Config{APIKey: "test-key"}, nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.handler != handler {
		t.Error("handler should be set")
	}
}

func TestHarness_Cancel_WhenIdle(t *testing.T) {
	h, _ := NewHarness(Config{APIKey: "test-key"}, nil, nil)
	// Should not panic when no prompt is running
	h.Cancel()
}

func TestHarness_ConcurrencyControl(t *testing.T) {
	h, _ := NewHarness(Config{APIKey: "test-key"}, nil, nil)

	// Simulate a running prompt by setting the running flag
	h.mu.Lock()
	h.running = true
	_, cancel := context.WithCancel(context.Background())
	h.cancelFunc = cancel
	h.mu.Unlock()

	// Try to start another prompt - should fail immediately
	err := h.Prompt(context.Background(), "test")
	if err != ErrPromptInProgress {
		t.Errorf("expected ErrPromptInProgress, got %v", err)
	}

	// Clean up
	h.mu.Lock()
	h.running = false
	h.cancelFunc = nil
	h.mu.Unlock()
	cancel()
}

func TestHarness_Messages(t *testing.T) {
	h, _ := NewHarness(Config{APIKey: "test-key"}, nil, nil)

	// Initially no messages
	msgs := h.Messages()
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages initially, got %d", len(msgs))
	}
}

func TestToolToParam(t *testing.T) {
	mockTool := &MockTool{
		name:        "test_tool",
		description: "Test description",
	}

	param := toolToParam(mockTool)

	if param.OfTool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}
	if param.OfTool.Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got %q", param.OfTool.Name)
	}
	// Description is param.Opt[string], use Value() to get the value
	if param.OfTool.Description.Value != "Test description" {
		t.Errorf("expected description 'Test description', got %q", param.OfTool.Description.Value)
	}
}

func TestHarness_ExecuteTool_UnknownTool(t *testing.T) {
	h, _ := NewHarness(Config{APIKey: "test-key"}, nil, nil)

	call := ToolCall{
		ID:    "test-id",
		Name:  "unknown_tool",
		Input: json.RawMessage(`{}`),
	}

	result, err := h.executeTool(context.Background(), call)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestHarness_ExecuteTool_Success(t *testing.T) {
	tools := []tool.Tool{
		&MockTool{
			name:        "test_tool",
			description: "A test tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				return `{"success":true}`, nil
			},
		},
	}
	h, _ := NewHarness(Config{APIKey: "test-key"}, tools, nil)

	call := ToolCall{
		ID:    "test-id",
		Name:  "test_tool",
		Input: json.RawMessage(`{"value":"test"}`),
	}

	result, err := h.executeTool(context.Background(), call)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != `{"success":true}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestHarness_ExecuteTools_FailFast(t *testing.T) {
	callOrder := []string{}
	mu := sync.Mutex{}

	tools := []tool.Tool{
		&MockTool{
			name:        "tool1",
			description: "First tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				mu.Lock()
				callOrder = append(callOrder, "tool1")
				mu.Unlock()
				return "", &mockError{"tool1 error"}
			},
		},
		&MockTool{
			name:        "tool2",
			description: "Second tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				mu.Lock()
				callOrder = append(callOrder, "tool2")
				mu.Unlock()
				return `{"success":true}`, nil
			},
		},
	}

	handler := &MockEventHandler{}
	h, _ := NewHarness(Config{APIKey: "test-key"}, tools, handler)

	calls := []ToolCall{
		{ID: "id1", Name: "tool1", Input: json.RawMessage(`{}`)},
		{ID: "id2", Name: "tool2", Input: json.RawMessage(`{}`)},
	}

	results, err := h.executeTools(context.Background(), calls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 result (fail-fast after first error)
	if len(results) != 1 {
		t.Errorf("expected 1 result (fail-fast), got %d", len(results))
	}

	// Only tool1 should have been called
	mu.Lock()
	if len(callOrder) != 1 || callOrder[0] != "tool1" {
		t.Errorf("expected only tool1 to be called, got %v", callOrder)
	}
	mu.Unlock()

	// Check that handler received the error result
	if len(handler.ToolResults) != 1 {
		t.Errorf("expected 1 tool result, got %d", len(handler.ToolResults))
	}
	if !handler.ToolResults[0].IsError {
		t.Error("expected first result to be an error")
	}
}

func TestHarness_ExecuteTools_ContextCancellation(t *testing.T) {
	tools := []tool.Tool{
		&MockTool{
			name:        "slow_tool",
			description: "A slow tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(10 * time.Second):
					return `{"done":true}`, nil
				}
			},
		},
	}

	h, _ := NewHarness(Config{APIKey: "test-key"}, tools, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	calls := []ToolCall{
		{ID: "id1", Name: "slow_tool", Input: json.RawMessage(`{}`)},
	}

	_, err := h.executeTools(ctx, calls)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// mockError is a simple error type for testing.
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
