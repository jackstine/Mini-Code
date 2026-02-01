package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Event represents a server-sent event.
type Event struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp,omitempty"`

	// For user/text/reasoning events
	Content string `json:"content,omitempty"`

	// For tool_call events
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For tool_result events
	Result  string `json:"result,omitempty"`
	IsError bool   `json:"isError,omitempty"`

	// For status events
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

// handleSSE handles GET /events SSE connections.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Register this client
	client := s.addClient()
	defer s.removeClient(client)

	// Heartbeat ticker - 30 seconds
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case event, ok := <-client.events:
			if !ok {
				return // Channel closed
			}
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// broadcast sends an event to all connected SSE clients.
func (s *Server) broadcast(event Event) {
	event.Timestamp = time.Now().Unix()
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for client := range s.clients {
		select {
		case client.events <- data:
		default:
			// Client buffer full, skip (non-blocking)
		}
	}
}

// sseEventHandler implements harness.EventHandler and broadcasts to SSE clients.
type sseEventHandler struct {
	server *Server
}

// OnText broadcasts a text event.
func (h *sseEventHandler) OnText(text string) {
	h.server.broadcast(Event{Type: "text", Content: text})
}

// OnToolCall broadcasts a tool_call event.
func (h *sseEventHandler) OnToolCall(id string, name string, input json.RawMessage) {
	// Broadcast status: running_tool
	h.server.broadcast(Event{Type: "status", State: "running_tool", Message: name})
	h.server.broadcast(Event{Type: "tool_call", ID: id, Name: name, Input: input})
}

// OnToolResult broadcasts a tool_result event.
func (h *sseEventHandler) OnToolResult(id string, result string, isError bool) {
	h.server.broadcast(Event{Type: "tool_result", ID: id, Result: result, IsError: isError})
	// Set status back to thinking after tool result
	h.server.broadcast(Event{Type: "status", State: "thinking"})
}
