// Package server provides the HTTP server for exposing the Harness to clients.
// It includes REST endpoints for prompt submission and cancellation, and
// an SSE endpoint for real-time event streaming.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/user/harness/pkg/harness"
	"github.com/user/harness/pkg/log"
)

// UserPromptLogger is a callback for logging user prompts.
type UserPromptLogger func(content string)

// Server wraps a Harness and exposes it over HTTP.
type Server struct {
	harness *harness.Harness
	addr    string
	logger  log.Logger

	// Optional callback to log user prompts for agent interaction logging
	userPromptLogger UserPromptLogger

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
// If logger is nil, a NopLogger is used.
func NewServer(h *harness.Harness, addr string, logger log.Logger) *Server {
	if logger == nil {
		logger = log.NopLogger{}
	}
	return &Server{
		harness: h,
		addr:    addr,
		logger:  logger,
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
	start := time.Now()
	s.logger.Info("http", "Request received",
		log.F("method", r.Method),
		log.F("path", r.URL.Path),
		log.F("content_length", r.ContentLength),
	)

	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Warn("http", "Request validation failed",
			log.F("method", r.Method),
			log.F("path", r.URL.Path),
			log.F("error", "invalid request body"),
		)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		s.logger.Warn("http", "Request validation failed",
			log.F("method", r.Method),
			log.F("path", r.URL.Path),
			log.F("error", "content is required"),
		)
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	// Log user prompt to agent log if logger is set
	if s.userPromptLogger != nil {
		s.userPromptLogger(req.Content)
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

	duration := time.Since(start)
	s.logger.Info("http", "Response sent",
		log.F("method", r.Method),
		log.F("path", r.URL.Path),
		log.F("status", http.StatusOK),
		log.F("duration_ms", duration.Milliseconds()),
	)
	w.WriteHeader(http.StatusOK)
}

// HandleCancel handles POST /cancel requests.
func (s *Server) HandleCancel(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("http", "Cancel requested",
		log.F("method", r.Method),
		log.F("path", r.URL.Path),
	)
	s.harness.Cancel()
	w.WriteHeader(http.StatusOK)
}

// addClient registers a new SSE client and returns it.
func (s *Server) addClient(remoteAddr string) *sseClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	client := &sseClient{
		id:     s.nextID,
		events: make(chan []byte, 100), // Buffer to prevent blocking
	}
	s.clients[client] = struct{}{}
	s.logger.Info("sse", "Client connected",
		log.F("client_id", client.id),
		log.F("remote_addr", remoteAddr),
	)
	return client
}

// removeClient unregisters an SSE client.
func (s *Server) removeClient(client *sseClient, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
	close(client.events)
	s.logger.Info("sse", "Client disconnected",
		log.F("client_id", client.id),
		log.F("duration_s", int(duration.Seconds())),
	)
}

// EventHandler returns an EventHandler that broadcasts events to SSE clients.
func (s *Server) EventHandler() harness.EventHandler {
	return &sseEventHandler{server: s}
}

// SetUserPromptLogger sets a callback that will be called with user prompts
// when they are submitted. This allows logging user prompts to the agent log.
func (s *Server) SetUserPromptLogger(logger UserPromptLogger) {
	s.userPromptLogger = logger
}
