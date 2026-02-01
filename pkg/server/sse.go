package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/user/harness/pkg/log"
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

// HandleSSE handles GET /events SSE connections.
func (s *Server) HandleSSE(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

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
	client := s.addClient(r.RemoteAddr)
	defer func() {
		s.removeClient(client, time.Since(start))
	}()

	// Send initial connection comment to establish the stream
	// This allows HTTP clients to know the connection is established
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

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
			s.logger.Warn("sse", "Event dropped - client buffer full",
				log.F("client_id", client.id),
				log.F("event_type", event.Type),
			)
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

// OnReasoning broadcasts a reasoning event.
func (h *sseEventHandler) OnReasoning(content string) {
	h.server.broadcast(Event{Type: "reasoning", Content: content})
}
