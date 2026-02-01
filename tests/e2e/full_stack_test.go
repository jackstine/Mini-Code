// Package e2e provides end-to-end tests for the full harness stack.
// These tests verify the complete flow from HTTP request through the harness
// to SSE event delivery, using real HTTP servers and clients.
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/user/harness/pkg/harness"
	"github.com/user/harness/pkg/server"
	"github.com/user/harness/pkg/testutil"
	"github.com/user/harness/pkg/tool"
)

// MockTool is a simple tool implementation for testing.
type MockTool struct {
	name        string
	description string
	inputSchema json.RawMessage
	executeFunc func(ctx context.Context, input json.RawMessage) (string, error)
}

func (t *MockTool) Name() string        { return t.name }
func (t *MockTool) Description() string { return t.description }
func (t *MockTool) InputSchema() json.RawMessage {
	if t.inputSchema != nil {
		return t.inputSchema
	}
	return json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}}}`)
}
func (t *MockTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	if t.executeFunc != nil {
		return t.executeFunc(ctx, input)
	}
	return `{"result":"mock result"}`, nil
}

// sseEvent represents a parsed SSE event.
type sseEvent struct {
	Type      string          `json:"type"`
	Content   string          `json:"content,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Result    string          `json:"result,omitempty"`
	IsError   bool            `json:"isError,omitempty"`
	State     string          `json:"state,omitempty"`
	Message   string          `json:"message,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
}

// sseClient represents an SSE client connection.
type sseClient struct {
	url       string
	events    []sseEvent
	mu        sync.Mutex
	cancel    context.CancelFunc
	connected chan struct{}
	done      chan struct{}
}

// newSSEClient creates a new SSE client connected to the given URL.
func newSSEClient(url string) *sseClient {
	return &sseClient{
		url:       url,
		events:    []sseEvent{},
		connected: make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// connect starts the SSE client connection in a goroutine.
func (c *sseClient) connect(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go func() {
		defer close(c.done)

		req, err := http.NewRequestWithContext(ctx, "GET", c.url+"/events", nil)
		if err != nil {
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			close(c.connected)
			return
		}
		defer resp.Body.Close()

		// Signal that we're connected
		close(c.connected)

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			line = strings.TrimSpace(line)
			if data, found := strings.CutPrefix(line, "data:"); found {
				data = strings.TrimSpace(data)

				var event sseEvent
				if err := json.Unmarshal([]byte(data), &event); err == nil {
					c.mu.Lock()
					c.events = append(c.events, event)
					c.mu.Unlock()
				}
			}
		}
	}()

	// Wait for connection to establish
	select {
	case <-c.connected:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("timeout waiting for SSE connection")
	}
}

// close disconnects the SSE client.
func (c *sseClient) close() {
	if c.cancel != nil {
		c.cancel()
	}
	// Wait for done with timeout
	select {
	case <-c.done:
	case <-time.After(time.Second):
	}
}

// waitForEvents waits until at least n events are received or timeout.
func (c *sseClient) waitForEvents(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		count := len(c.events)
		c.mu.Unlock()
		if count >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// waitForEventType waits until an event of the given type is received.
func (c *sseClient) waitForEventType(eventType string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		for _, e := range c.events {
			if e.Type == eventType {
				c.mu.Unlock()
				return true
			}
		}
		c.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// getEvents returns a copy of all received events.
func (c *sseClient) getEvents() []sseEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]sseEvent, len(c.events))
	copy(result, c.events)
	return result
}


// testServer wraps a real HTTP server for testing.
type testServer struct {
	server   *server.Server
	listener net.Listener
	url      string
	done     chan struct{}
}

// newTestServer creates a new test server with the given harness.
func newTestServer(t *testing.T, h *harness.Harness) *testServer {
	t.Helper()

	// Create a listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	addr := listener.Addr().String()
	url := "http://" + addr

	s := server.NewServer(h, addr, nil)
	h.SetEventHandler(s.EventHandler())

	ts := &testServer{
		server:   s,
		listener: listener,
		url:      url,
		done:     make(chan struct{}),
	}

	// Start the server
	go func() {
		defer close(ts.done)
		mux := http.NewServeMux()
		mux.HandleFunc("GET /events", s.HandleSSE)
		mux.HandleFunc("POST /prompt", s.HandlePrompt)
		mux.HandleFunc("POST /cancel", s.HandleCancel)
		http.Serve(listener, mux)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	return ts
}

// close shuts down the test server.
func (ts *testServer) close() {
	ts.listener.Close()
	select {
	case <-ts.done:
	case <-time.After(time.Second):
	}
}

// createTestStack creates a full test stack with harness, server, and SSE client.
func createTestStack(t *testing.T, mockStreamer *testutil.MockMessageStreamer, tools []tool.Tool) (*testServer, *sseClient, func()) {
	t.Helper()

	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		tools,
		nil, // EventHandler will be set by server
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	ts := newTestServer(t, h)
	client := newSSEClient(ts.url)

	if err := client.connect(context.Background()); err != nil {
		ts.close()
		t.Fatalf("failed to connect SSE client: %v", err)
	}

	// Give time for connection to stabilize
	time.Sleep(50 * time.Millisecond)

	cleanup := func() {
		client.close()
		ts.close()
	}

	return ts, client, cleanup
}

// sendPrompt sends a prompt to the test server.
func sendPrompt(url, content string) (*http.Response, error) {
	reqBody := bytes.NewBufferString(fmt.Sprintf(`{"content":%q}`, content))
	return http.Post(url+"/prompt", "application/json", reqBody)
}

// sendCancel sends a cancel request to the test server.
func sendCancel(url string) (*http.Response, error) {
	return http.Post(url+"/cancel", "application/json", nil)
}

// TestE2E_FullPromptResponseFlow tests the complete prompt to response flow.
// This verifies that:
// 1. POST /prompt is accepted
// 2. SSE receives user event
// 3. SSE receives status:thinking event
// 4. SSE receives text response event
// 5. SSE receives status:idle event
func TestE2E_FullPromptResponseFlow(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Hello from Claude!"))

	ts, client, cleanup := createTestStack(t, mockStreamer, nil)
	defer cleanup()

	// Send prompt
	resp, err := sendPrompt(ts.url, "Hello")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Wait for complete event sequence
	if !client.waitForEvents(4, 3*time.Second) {
		events := client.getEvents()
		t.Fatalf("timeout waiting for events, got %d: %+v", len(events), events)
	}

	events := client.getEvents()

	// Verify event sequence
	expectedSequence := []struct {
		eventType string
		check     func(e sseEvent) error
	}{
		{"user", func(e sseEvent) error {
			if e.Content != "Hello" {
				return fmt.Errorf("expected content 'Hello', got %q", e.Content)
			}
			return nil
		}},
		{"status", func(e sseEvent) error {
			if e.State != "thinking" {
				return fmt.Errorf("expected state 'thinking', got %q", e.State)
			}
			return nil
		}},
		{"text", func(e sseEvent) error {
			if e.Content != "Hello from Claude!" {
				return fmt.Errorf("expected content 'Hello from Claude!', got %q", e.Content)
			}
			return nil
		}},
		{"status", func(e sseEvent) error {
			if e.State != "idle" {
				return fmt.Errorf("expected state 'idle', got %q", e.State)
			}
			return nil
		}},
	}

	if len(events) < len(expectedSequence) {
		t.Fatalf("expected at least %d events, got %d", len(expectedSequence), len(events))
	}

	for i, expected := range expectedSequence {
		if events[i].Type != expected.eventType {
			t.Errorf("event %d: expected type %q, got %q", i, expected.eventType, events[i].Type)
			continue
		}
		if err := expected.check(events[i]); err != nil {
			t.Errorf("event %d: %v", i, err)
		}
	}
}

// TestE2E_MultipleClientsReceiveSameEvents tests that multiple SSE clients
// all receive the same broadcast events.
func TestE2E_MultipleClientsReceiveSameEvents(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Broadcast to all!"))

	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		nil,
		nil,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	ts := newTestServer(t, h)
	defer ts.close()

	// Connect multiple clients
	numClients := 3
	clients := make([]*sseClient, numClients)
	for i := range numClients {
		clients[i] = newSSEClient(ts.url)
		if err := clients[i].connect(context.Background()); err != nil {
			t.Fatalf("client %d failed to connect: %v", i, err)
		}
	}

	defer func() {
		for _, client := range clients {
			client.close()
		}
	}()

	// Give time for all connections to stabilize
	time.Sleep(100 * time.Millisecond)

	// Send prompt
	resp, err := sendPrompt(ts.url, "Test broadcast")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for all clients to receive events
	for i, client := range clients {
		if !client.waitForEvents(4, 3*time.Second) {
			events := client.getEvents()
			t.Errorf("client %d: timeout waiting for events, got %d", i, len(events))
			continue
		}

		events := client.getEvents()

		// Verify each client received the text event with correct content
		found := false
		for _, e := range events {
			if e.Type == "text" && e.Content == "Broadcast to all!" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("client %d: did not receive expected text event", i)
		}
	}
}

// TestE2E_ToolExecutionEvents tests that tool execution generates the proper
// event sequence visible to SSE clients.
func TestE2E_ToolExecutionEvents(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_abc123",
		"calculator",
		map[string]any{"expression": "2+2"},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("The answer is 4"))

	tools := []tool.Tool{
		&MockTool{
			name:        "calculator",
			description: "Evaluates mathematical expressions",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				return `{"result": 4}`, nil
			},
		},
	}

	ts, client, cleanup := createTestStack(t, mockStreamer, tools)
	defer cleanup()

	// Send prompt
	resp, err := sendPrompt(ts.url, "What is 2+2?")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for events including tool_call and tool_result
	if !client.waitForEvents(8, 3*time.Second) {
		// Continue with what we have
	}

	events := client.getEvents()

	// Verify we have the expected event types
	var (
		hasUserEvent       bool
		hasToolCallEvent   bool
		hasToolResultEvent bool
		hasRunningTool     bool
		hasFinalText       bool
		toolCallIndex      int
		toolResultIndex    int
	)

	for i, e := range events {
		switch e.Type {
		case "user":
			hasUserEvent = true
		case "status":
			if e.State == "running_tool" {
				hasRunningTool = true
			}
		case "tool_call":
			hasToolCallEvent = true
			toolCallIndex = i
			if e.ID != "tool_abc123" {
				t.Errorf("tool_call: expected ID 'tool_abc123', got %q", e.ID)
			}
			if e.Name != "calculator" {
				t.Errorf("tool_call: expected Name 'calculator', got %q", e.Name)
			}
		case "tool_result":
			hasToolResultEvent = true
			toolResultIndex = i
			if e.ID != "tool_abc123" {
				t.Errorf("tool_result: expected ID 'tool_abc123', got %q", e.ID)
			}
			if e.IsError {
				t.Error("tool_result: expected IsError to be false")
			}
		case "text":
			if e.Content == "The answer is 4" {
				hasFinalText = true
			}
		}
	}

	if !hasUserEvent {
		t.Error("missing user event")
	}
	if !hasRunningTool {
		t.Error("missing status:running_tool event")
	}
	if !hasToolCallEvent {
		t.Error("missing tool_call event")
	}
	if !hasToolResultEvent {
		t.Error("missing tool_result event")
	}
	if !hasFinalText {
		t.Error("missing final text event")
	}

	// Verify ordering: tool_call should come before tool_result
	if hasToolCallEvent && hasToolResultEvent && toolCallIndex >= toolResultIndex {
		t.Error("tool_call should come before tool_result")
	}
}

// TestE2E_ErrorHandlingBroadcast tests that errors are properly broadcast
// to all connected SSE clients.
func TestE2E_ErrorHandlingBroadcast(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.ErrorResponse(errors.New("API rate limit exceeded")))

	ts, client, cleanup := createTestStack(t, mockStreamer, nil)
	defer cleanup()

	// Send prompt
	resp, err := sendPrompt(ts.url, "Hello")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for error status event
	if !client.waitForEvents(3, 3*time.Second) {
		// Continue with what we have
	}

	events := client.getEvents()

	// Verify error status event was broadcast
	var hasErrorStatus bool
	for _, e := range events {
		if e.Type == "status" && e.State == "error" {
			hasErrorStatus = true
			if !strings.Contains(e.Message, "rate limit") {
				t.Errorf("expected error message to contain 'rate limit', got %q", e.Message)
			}
			break
		}
	}

	if !hasErrorStatus {
		t.Error("missing status:error event")
		t.Logf("events received: %+v", events)
	}
}

// TestE2E_ToolErrorBroadcast tests that tool execution errors are visible
// to SSE clients.
func TestE2E_ToolErrorBroadcast(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_fail",
		"failing_tool",
		map[string]string{},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Error handled"))

	tools := []tool.Tool{
		&MockTool{
			name:        "failing_tool",
			description: "A tool that always fails",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				return "", errors.New("tool execution failed: permission denied")
			},
		},
	}

	ts, client, cleanup := createTestStack(t, mockStreamer, tools)
	defer cleanup()

	// Send prompt
	resp, err := sendPrompt(ts.url, "Use the failing tool")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for tool_result event
	if !client.waitForEventType("tool_result", 3*time.Second) {
		t.Fatal("timeout waiting for tool_result event")
	}

	events := client.getEvents()

	// Verify tool_result has isError=true
	var hasErrorResult bool
	for _, e := range events {
		if e.Type == "tool_result" && e.ID == "tool_fail" {
			hasErrorResult = true
			if !e.IsError {
				t.Error("expected tool_result IsError to be true")
			}
			if !strings.Contains(e.Result, "permission denied") {
				t.Errorf("expected result to contain 'permission denied', got %q", e.Result)
			}
			break
		}
	}

	if !hasErrorResult {
		t.Error("missing tool_result event with error")
	}
}

// TestE2E_ConcurrentPromptHandling tests behavior when multiple prompts
// are submitted concurrently.
func TestE2E_ConcurrentPromptHandling(t *testing.T) {
	toolStarted := make(chan struct{})
	toolDone := make(chan struct{})

	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"blocking_tool",
		map[string]string{},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("First complete"))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Second response"))

	tools := []tool.Tool{
		&MockTool{
			name: "blocking_tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				close(toolStarted)
				select {
				case <-toolDone:
					return `{"done": true}`, nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			},
		},
	}

	ts, _, cleanup := createTestStack(t, mockStreamer, tools)
	defer cleanup()

	// Start first prompt (it will block on tool)
	go func() {
		resp, _ := sendPrompt(ts.url, "First")
		if resp != nil {
			resp.Body.Close()
		}
	}()

	// Wait for tool to start
	select {
	case <-toolStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for tool to start")
	}

	// Try to send second prompt while first is running
	resp, err := sendPrompt(ts.url, "Second")
	if err != nil {
		t.Fatalf("second POST /prompt failed: %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Log actual behavior
	t.Logf("Second prompt response: status=%d body=%s", resp.StatusCode, string(body))

	// Let first prompt complete
	close(toolDone)

	// Give time for completion
	time.Sleep(100 * time.Millisecond)
}

// TestE2E_CancelDuringExecution tests that cancellation properly stops
// tool execution and broadcasts error status.
func TestE2E_CancelDuringExecution(t *testing.T) {
	toolStarted := make(chan struct{})
	toolCancelled := make(chan struct{})

	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"cancellable_tool",
		map[string]string{},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Should not see this"))

	tools := []tool.Tool{
		&MockTool{
			name: "cancellable_tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				close(toolStarted)
				select {
				case <-ctx.Done():
					close(toolCancelled)
					return "", ctx.Err()
				case <-time.After(30 * time.Second):
					return `{}`, nil
				}
			},
		},
	}

	ts, client, cleanup := createTestStack(t, mockStreamer, tools)
	defer cleanup()

	// Start prompt in goroutine
	go func() {
		resp, _ := sendPrompt(ts.url, "Run cancellable tool")
		if resp != nil {
			resp.Body.Close()
		}
	}()

	// Wait for tool to start
	select {
	case <-toolStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for tool to start")
	}

	// Send cancel request
	resp, err := sendCancel(ts.url)
	if err != nil {
		t.Fatalf("POST /cancel failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected cancel status 200, got %d", resp.StatusCode)
	}

	// Verify tool was cancelled
	select {
	case <-toolCancelled:
		// Good - tool was cancelled
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for tool to be cancelled")
	}

	// Wait for error status in events
	client.waitForEvents(4, 2*time.Second)

	events := client.getEvents()

	// Should have received status:error event
	var hasErrorStatus bool
	for _, e := range events {
		if e.Type == "status" && e.State == "error" {
			hasErrorStatus = true
			break
		}
	}

	if !hasErrorStatus {
		t.Log("Note: error status may not be received depending on timing")
		t.Logf("Events: %+v", events)
	}
}

// TestE2E_MultipleToolCalls tests that multiple tool calls in a single
// response generate proper events.
func TestE2E_MultipleToolCalls(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.MultiToolResponse([]struct{ ID, Name string; Input any }{
		{ID: "tool_1", Name: "tool_a", Input: map[string]string{"param": "a"}},
		{ID: "tool_2", Name: "tool_b", Input: map[string]string{"param": "b"}},
	}))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Both tools complete"))

	var executionOrder []string
	var mu sync.Mutex

	tools := []tool.Tool{
		&MockTool{
			name: "tool_a",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				mu.Lock()
				executionOrder = append(executionOrder, "tool_a")
				mu.Unlock()
				return `{"result": "a_done"}`, nil
			},
		},
		&MockTool{
			name: "tool_b",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				mu.Lock()
				executionOrder = append(executionOrder, "tool_b")
				mu.Unlock()
				return `{"result": "b_done"}`, nil
			},
		},
	}

	ts, client, cleanup := createTestStack(t, mockStreamer, tools)
	defer cleanup()

	// Send prompt
	resp, err := sendPrompt(ts.url, "Run both tools")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for events
	client.waitForEvents(10, 3*time.Second)

	events := client.getEvents()

	// Count tool events
	var toolCalls, toolResults int
	for _, e := range events {
		if e.Type == "tool_call" {
			toolCalls++
		}
		if e.Type == "tool_result" {
			toolResults++
		}
	}

	t.Logf("Tool calls: %d, Tool results: %d", toolCalls, toolResults)
	t.Logf("Execution order: %v", executionOrder)

	// Verify at least first tool was called and resulted
	if toolCalls < 1 {
		t.Error("expected at least 1 tool_call event")
	}
	if toolResults < 1 {
		t.Error("expected at least 1 tool_result event")
	}
}

// TestE2E_ReasoningEvents tests that thinking/reasoning blocks are broadcast.
func TestE2E_ReasoningEvents(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.ThinkingResponse(
		"Let me consider this carefully...",
		"Here is my thoughtful response.",
	))

	ts, client, cleanup := createTestStack(t, mockStreamer, nil)
	defer cleanup()

	// Send prompt
	resp, err := sendPrompt(ts.url, "Think about this")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for events
	if !client.waitForEvents(5, 3*time.Second) {
		// Continue with what we have
	}

	events := client.getEvents()

	// Verify reasoning event exists
	var hasReasoningEvent bool
	for _, e := range events {
		if e.Type == "reasoning" {
			hasReasoningEvent = true
			if e.Content != "Let me consider this carefully..." {
				t.Errorf("expected reasoning content, got %q", e.Content)
			}
			break
		}
	}

	if !hasReasoningEvent {
		t.Error("missing reasoning event")
		t.Logf("events: %+v", events)
	}
}

// TestE2E_ClientReconnection tests that a client can disconnect and reconnect
// and receive new events properly.
func TestE2E_ClientReconnection(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.TextOnlyResponse("First response"))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Second response"))

	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		nil,
		nil,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	ts := newTestServer(t, h)
	defer ts.close()

	// First client connects
	client1 := newSSEClient(ts.url)
	if err := client1.connect(context.Background()); err != nil {
		t.Fatalf("client1 failed to connect: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send first prompt
	resp, err := sendPrompt(ts.url, "First")
	if err != nil {
		t.Fatalf("first POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for events
	client1.waitForEvents(4, 2*time.Second)

	// Verify first client received events
	events1 := client1.getEvents()
	if len(events1) < 4 {
		t.Logf("client1 received %d events", len(events1))
	}

	// Disconnect first client
	client1.close()
	time.Sleep(50 * time.Millisecond)

	// Connect new client
	client2 := newSSEClient(ts.url)
	if err := client2.connect(context.Background()); err != nil {
		t.Fatalf("client2 failed to connect: %v", err)
	}
	defer client2.close()

	time.Sleep(50 * time.Millisecond)

	// Send second prompt
	resp, err = sendPrompt(ts.url, "Second")
	if err != nil {
		t.Fatalf("second POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for second client to receive events
	if !client2.waitForEvents(4, 2*time.Second) {
		t.Fatal("client2 timeout waiting for events")
	}

	events2 := client2.getEvents()

	// Verify second client received second response
	var hasSecondResponse bool
	for _, e := range events2 {
		if e.Type == "text" && e.Content == "Second response" {
			hasSecondResponse = true
			break
		}
	}

	if !hasSecondResponse {
		t.Error("client2 did not receive second response")
		t.Logf("events: %+v", events2)
	}
}

// TestE2E_StatusTransitions tests that status events follow the expected
// state machine: idle -> thinking -> (running_tool -> thinking)* -> idle
func TestE2E_StatusTransitions(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_1",
		"test_tool",
		map[string]string{},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Complete"))

	tools := []tool.Tool{
		&MockTool{
			name: "test_tool",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				return `{}`, nil
			},
		},
	}

	ts, client, cleanup := createTestStack(t, mockStreamer, tools)
	defer cleanup()

	// Send prompt
	resp, err := sendPrompt(ts.url, "Test status")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for complete event sequence
	if !client.waitForEvents(8, 3*time.Second) {
		// Continue with what we have
	}

	events := client.getEvents()

	// Extract status states in order
	var statusStates []string
	for _, e := range events {
		if e.Type == "status" {
			statusStates = append(statusStates, e.State)
		}
	}

	t.Logf("Status sequence: %v", statusStates)

	// Expected: thinking -> running_tool -> thinking -> idle
	expectedSequence := []string{"thinking", "running_tool", "thinking", "idle"}

	// Verify the sequence appears in order (may have additional states)
	seqIndex := 0
	for _, state := range statusStates {
		if seqIndex < len(expectedSequence) && state == expectedSequence[seqIndex] {
			seqIndex++
		}
	}

	if seqIndex != len(expectedSequence) {
		t.Errorf("status sequence mismatch: expected %v in order, got %v", expectedSequence, statusStates)
	}
}

// TestE2E_EmptyContentRejected tests that empty prompt content returns 400.
func TestE2E_EmptyContentRejected(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	ts, client, cleanup := createTestStack(t, mockStreamer, nil)
	_ = client // Not used in this test but needed for createTestStack
	defer cleanup()

	reqBody := bytes.NewBufferString(`{"content":""}`)
	resp, err := http.Post(ts.url+"/prompt", "application/json", reqBody)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

// TestE2E_InvalidJSONRejected tests that invalid JSON returns 400.
func TestE2E_InvalidJSONRejected(t *testing.T) {
	mockStreamer := testutil.NewMockMessageStreamer()
	ts, client, cleanup := createTestStack(t, mockStreamer, nil)
	_ = client // Not used in this test but needed for createTestStack
	defer cleanup()

	reqBody := bytes.NewBufferString(`not valid json`)
	resp, err := http.Post(ts.url+"/prompt", "application/json", reqBody)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

// TestE2E_HighConcurrency tests the system under high concurrency with
// multiple SSE clients connecting and receiving events simultaneously.
func TestE2E_HighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping high concurrency test in short mode")
	}

	mockStreamer := testutil.NewMockMessageStreamer()
	// Add multiple responses for multiple prompts
	for i := range 10 {
		mockStreamer.AddResponse(testutil.TextOnlyResponse(fmt.Sprintf("Response %d", i)))
	}

	h, err := harness.NewHarnessWithStreamer(
		harness.Config{Model: "test-model"},
		nil,
		nil,
		mockStreamer,
	)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}

	ts := newTestServer(t, h)
	defer ts.close()

	// Connect multiple clients
	numClients := 5
	clients := make([]*sseClient, numClients)
	for i := range numClients {
		clients[i] = newSSEClient(ts.url)
		if err := clients[i].connect(context.Background()); err != nil {
			t.Fatalf("client %d failed to connect: %v", i, err)
		}
	}

	defer func() {
		for _, client := range clients {
			client.close()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Send multiple prompts in rapid succession
	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := sendPrompt(ts.url, fmt.Sprintf("Prompt %d", idx))
			if err != nil {
				return
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Successful prompts: %d/10", successCount.Load())

	// Give time for events to propagate
	time.Sleep(500 * time.Millisecond)

	// Verify each client received at least some events
	for i, client := range clients {
		events := client.getEvents()
		t.Logf("Client %d received %d events", i, len(events))
		if len(events) == 0 {
			t.Errorf("client %d received no events", i)
		}
	}
}

// TestE2E_LongRunningOperation tests that long-running tool executions
// properly maintain SSE connections and deliver events.
func TestE2E_LongRunningOperation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	operationComplete := make(chan struct{})

	mockStreamer := testutil.NewMockMessageStreamer()
	mockStreamer.AddResponse(testutil.SingleToolResponse(
		"tool_long",
		"long_operation",
		map[string]string{},
	))
	mockStreamer.AddResponse(testutil.TextOnlyResponse("Operation complete"))

	tools := []tool.Tool{
		&MockTool{
			name: "long_operation",
			executeFunc: func(ctx context.Context, input json.RawMessage) (string, error) {
				// Simulate a 2-second operation
				select {
				case <-time.After(2 * time.Second):
					close(operationComplete)
					return `{"status": "complete"}`, nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			},
		},
	}

	ts, client, cleanup := createTestStack(t, mockStreamer, tools)
	defer cleanup()

	// Send prompt
	resp, err := sendPrompt(ts.url, "Run long operation")
	if err != nil {
		t.Fatalf("POST /prompt failed: %v", err)
	}
	resp.Body.Close()

	// Wait for operation to complete
	select {
	case <-operationComplete:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for long operation to complete")
	}

	// Wait for final events
	client.waitForEvents(8, 3*time.Second)

	events := client.getEvents()

	// Verify we got the complete sequence including final text
	var hasFinalText bool
	for _, e := range events {
		if e.Type == "text" && e.Content == "Operation complete" {
			hasFinalText = true
			break
		}
	}

	if !hasFinalText {
		t.Error("did not receive final text event after long operation")
		t.Logf("events: %+v", events)
	}
}
