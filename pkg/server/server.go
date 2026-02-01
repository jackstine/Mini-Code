// Package server provides the HTTP server for exposing the Harness to clients.
// It includes REST endpoints for prompt submission and cancellation, and
// an SSE endpoint for real-time event streaming.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/user/harness/pkg/harness"
)

// Server wraps a Harness and exposes it over HTTP.
type Server struct {
	harness *harness.Harness
	addr    string

	// SSE client management
	mu      sync.RWMutex
	clients map[*sseClient]struct{}
	nextID  int
}

// sseClient represents a connected SSE client.
type sseClient struct {
	id     int
	events chan []byte
}

// NewServer creates a new HTTP server for the given harness.
func NewServer(h *harness.Harness, addr string) *Server {
	return &Server{
		harness: h,
		addr:    addr,
		clients: make(map[*sseClient]struct{}),
	}
}

// ListenAndServe starts the HTTP server and blocks until it's shut down.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /events", s.HandleSSE)
	mux.HandleFunc("POST /prompt", s.HandlePrompt)
	mux.HandleFunc("POST /cancel", s.HandleCancel)

	// Add CORS headers middleware
	handler := corsMiddleware(mux)

	return http.ListenAndServe(s.addr, handler)
}

// corsMiddleware adds CORS headers to all responses.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HandlePrompt handles POST /prompt requests.
func (s *Server) HandlePrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	// Broadcast user message event before starting
	s.broadcast(Event{Type: "user", Content: req.Content})

	// Run prompt asynchronously
	// Note: We use context.Background() here because the prompt runs independently
	// of the HTTP request lifecycle. The harness has its own Cancel() method for
	// explicit cancellation via the /cancel endpoint.
	go func() {
		// Broadcast status: thinking
		s.broadcast(Event{Type: "status", State: "thinking"})

		err := s.harness.Prompt(context.Background(), req.Content)
		if err != nil {
			// Broadcast error status
			s.broadcast(Event{Type: "status", State: "error", Message: err.Error()})
		} else {
			// Broadcast idle status
			s.broadcast(Event{Type: "status", State: "idle"})
		}
	}()

	w.WriteHeader(http.StatusOK)
}

// HandleCancel handles POST /cancel requests.
func (s *Server) HandleCancel(w http.ResponseWriter, r *http.Request) {
	s.harness.Cancel()
	w.WriteHeader(http.StatusOK)
}

// addClient registers a new SSE client.
func (s *Server) addClient() *sseClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	client := &sseClient{
		id:     s.nextID,
		events: make(chan []byte, 100), // Buffer to prevent blocking
	}
	s.clients[client] = struct{}{}
	return client
}

// removeClient unregisters an SSE client.
func (s *Server) removeClient(client *sseClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
	close(client.events)
}

// EventHandler returns an EventHandler that broadcasts events to SSE clients.
func (s *Server) EventHandler() harness.EventHandler {
	return &sseEventHandler{server: s}
}
