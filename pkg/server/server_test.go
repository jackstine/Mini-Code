package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/user/harness/pkg/harness"
)

// MockTool for testing
type MockTool struct {
	name        string
	description string
	executeFunc func(ctx context.Context, input json.RawMessage) (string, error)
}

func (t *MockTool) Name() string             { return t.name }
func (t *MockTool) Description() string      { return t.description }
func (t *MockTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}}}`)
}
func (t *MockTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	if t.executeFunc != nil {
		return t.executeFunc(ctx, input)
	}
	return `{"result":"mock result"}`, nil
}

func createTestHarness(t *testing.T) *harness.Harness {
	t.Helper()
	h, err := harness.NewHarness(harness.Config{APIKey: "test-key"}, nil, nil)
	if err != nil {
		t.Fatalf("failed to create harness: %v", err)
	}
	return h
}

func TestNewServer(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")
	if s == nil {
		t.Fatal("expected server to be non-nil")
	}
	if s.harness != h {
		t.Error("server should hold reference to harness")
	}
	if s.addr != ":8080" {
		t.Errorf("expected addr ':8080', got %q", s.addr)
	}
}

func TestServer_HandlePrompt_EmptyContent(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")

	// Create request with empty content
	body := bytes.NewBufferString(`{"content":""}`)
	req := httptest.NewRequest("POST", "/prompt", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handlePrompt(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestServer_HandlePrompt_InvalidJSON(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/prompt", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handlePrompt(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestServer_HandleCancel(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")

	req := httptest.NewRequest("POST", "/cancel", nil)
	rec := httptest.NewRecorder()

	s.handleCancel(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestServer_SSEClientManagement(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")

	// Add a client
	client := s.addClient()
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
	if client.id != 1 {
		t.Errorf("expected client id 1, got %d", client.id)
	}

	// Verify client is registered
	s.mu.RLock()
	if _, ok := s.clients[client]; !ok {
		t.Error("client should be registered")
	}
	s.mu.RUnlock()

	// Add another client
	client2 := s.addClient()
	if client2.id != 2 {
		t.Errorf("expected client id 2, got %d", client2.id)
	}

	// Remove first client
	s.removeClient(client)

	// Verify first client is unregistered
	s.mu.RLock()
	if _, ok := s.clients[client]; ok {
		t.Error("client should be unregistered")
	}
	// Second client should still be registered
	if _, ok := s.clients[client2]; !ok {
		t.Error("client2 should still be registered")
	}
	s.mu.RUnlock()

	// Clean up
	s.removeClient(client2)
}

func TestServer_Broadcast(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")

	// Add a client
	client := s.addClient()
	defer s.removeClient(client)

	// Broadcast an event
	event := Event{Type: "text", Content: "hello world"}
	s.broadcast(event)

	// Wait for event
	select {
	case data := <-client.events:
		var received Event
		if err := json.Unmarshal(data, &received); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		if received.Type != "text" {
			t.Errorf("expected type 'text', got %q", received.Type)
		}
		if received.Content != "hello world" {
			t.Errorf("expected content 'hello world', got %q", received.Content)
		}
		if received.Timestamp == 0 {
			t.Error("timestamp should be set")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestServer_BroadcastToMultipleClients(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")

	// Add multiple clients
	client1 := s.addClient()
	client2 := s.addClient()
	defer s.removeClient(client1)
	defer s.removeClient(client2)

	// Broadcast an event
	event := Event{Type: "text", Content: "broadcast test"}
	s.broadcast(event)

	// Both clients should receive the event
	for i, client := range []*sseClient{client1, client2} {
		select {
		case data := <-client.events:
			var received Event
			if err := json.Unmarshal(data, &received); err != nil {
				t.Fatalf("client %d: failed to unmarshal event: %v", i+1, err)
			}
			if received.Content != "broadcast test" {
				t.Errorf("client %d: expected content 'broadcast test', got %q", i+1, received.Content)
			}
		case <-time.After(time.Second):
			t.Fatalf("client %d: timeout waiting for event", i+1)
		}
	}
}

func TestSSEEventHandler(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")
	handler := s.EventHandler()

	// Add a client to receive events
	client := s.addClient()
	defer s.removeClient(client)

	// Test OnText
	handler.OnText("test text")

	select {
	case data := <-client.events:
		var event Event
		json.Unmarshal(data, &event)
		if event.Type != "text" {
			t.Errorf("expected type 'text', got %q", event.Type)
		}
		if event.Content != "test text" {
			t.Errorf("expected content 'test text', got %q", event.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for text event")
	}

	// Test OnToolCall - should emit status first, then tool_call
	handler.OnToolCall("tool-id-1", "test_tool", json.RawMessage(`{"key":"value"}`))

	// First should be status event
	select {
	case data := <-client.events:
		var event Event
		json.Unmarshal(data, &event)
		if event.Type != "status" {
			t.Errorf("expected type 'status', got %q", event.Type)
		}
		if event.State != "running_tool" {
			t.Errorf("expected state 'running_tool', got %q", event.State)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for status event")
	}

	// Then tool_call event
	select {
	case data := <-client.events:
		var event Event
		json.Unmarshal(data, &event)
		if event.Type != "tool_call" {
			t.Errorf("expected type 'tool_call', got %q", event.Type)
		}
		if event.ID != "tool-id-1" {
			t.Errorf("expected ID 'tool-id-1', got %q", event.ID)
		}
		if event.Name != "test_tool" {
			t.Errorf("expected name 'test_tool', got %q", event.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for tool_call event")
	}

	// Test OnToolResult
	handler.OnToolResult("tool-id-1", "result content", false)

	// First should be tool_result event
	select {
	case data := <-client.events:
		var event Event
		json.Unmarshal(data, &event)
		if event.Type != "tool_result" {
			t.Errorf("expected type 'tool_result', got %q", event.Type)
		}
		if event.ID != "tool-id-1" {
			t.Errorf("expected ID 'tool-id-1', got %q", event.ID)
		}
		if event.Result != "result content" {
			t.Errorf("expected result 'result content', got %q", event.Result)
		}
		if event.IsError {
			t.Error("expected IsError to be false")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for tool_result event")
	}

	// Then status back to thinking
	select {
	case data := <-client.events:
		var event Event
		json.Unmarshal(data, &event)
		if event.Type != "status" {
			t.Errorf("expected type 'status', got %q", event.Type)
		}
		if event.State != "thinking" {
			t.Errorf("expected state 'thinking', got %q", event.State)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for status event")
	}
}

func TestServer_HandleSSE(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")

	// Create a request with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	// Run SSE handler in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.handleSSE(rec, req)
	}()

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event
	s.broadcast(Event{Type: "text", Content: "sse test"})

	// Wait a bit for event to be processed
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop handler
	cancel()
	wg.Wait()

	// Verify response headers
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got %q", ct)
	}

	// Verify event was written
	body := rec.Body.String()
	if !strings.Contains(body, "data:") {
		t.Error("expected SSE data in response body")
	}
	if !strings.Contains(body, "sse test") {
		t.Error("expected 'sse test' in response body")
	}
}

func TestCORSMiddleware(t *testing.T) {
	h := createTestHarness(t)
	s := NewServer(h, ":8080")

	// Create test handler
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test regular request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Allow-Origin '*', got %q", origin)
	}

	// Test OPTIONS preflight
	req = httptest.NewRequest("OPTIONS", "/test", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for OPTIONS, got %d", rec.Code)
	}

	// Just use s to avoid unused variable warning
	_ = s
}
