package daemon

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"tower/internal/contracts"
)

// validHookEvents is the set of hook event types the daemon accepts.
var validHookEvents = map[string]bool{
	"pre-tool-use":          true,
	"permission-request":    true,
	"post-tool-use":         true,
	"post-tool-use-failure": true,
	"session-start":         true,
	"session-end":           true,
	"stop":                  true,
	"notification":          true,
	"subagent-start":        true,
	"subagent-stop":         true,
}

// syncHookEvents are events where Claude blocks waiting for a decision response.
var syncHookEvents = map[string]bool{
	"pre-tool-use":       true,
	"permission-request": true,
}

// Server is the Tower daemon HTTP server that receives hook events from
// managed Claude Code sessions and serves health/status endpoints.
type Server struct {
	mux      *http.ServeMux
	token    string
	mu       sync.RWMutex
	sessions map[string]struct{}
	events   map[string][]HookEvent
}

// NewServer creates a daemon HTTP server with the given auth token.
func NewServer(token string) *Server {
	s := &Server{
		mux:      http.NewServeMux(),
		token:    token,
		sessions: make(map[string]struct{}),
		events:   make(map[string][]HookEvent),
	}
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /hooks/{sessionID}/{eventType}", s.handleHook)
	return s
}

// RegisterSession adds a session ID to the set of known managed sessions.
func (s *Server) RegisterSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = struct{}{}
}

// UnregisterSession removes a session ID from the known set.
func (s *Server) UnregisterSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *Server) hasSession(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sessions[sessionID]
	return ok
}

// ReceivedEvents returns all hook events recorded for a session.
func (s *Server) ReceivedEvents(sessionID string) []HookEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events[sessionID]
}

func (s *Server) recordEvent(sessionID string, event HookEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[sessionID] = append(s.events[sessionID], event)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	// Auth check: all hook endpoints require a valid Bearer token.
	if !s.validAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.PathValue("sessionID")
	eventType := r.PathValue("eventType")

	// Unknown event type → 404.
	if !validHookEvents[eventType] {
		http.NotFound(w, r)
		return
	}

	// Unknown session → 200 empty per design doc section 6.4.
	if !s.hasSession(sessionID) {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse body.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	event, err := parseHookEvent(body)
	if err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	s.recordEvent(sessionID, event)

	// Sync events return a decision JSON. Async events return 200 empty.
	if syncHookEvents[eventType] {
		s.handleSyncHook(w, sessionID, eventType, event)
		return
	}

	// Async: acknowledge and return.
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSyncHook(w http.ResponseWriter, _ string, eventType string, event HookEvent) {
	w.Header().Set("Content-Type", "application/json")

	switch eventType {
	case "pre-tool-use":
		risk := ClassifyRisk(event.ToolName, event.ToolInput)
		if risk == contracts.RiskClassReadOnly {
			// Batch auto-approve: tell Claude to skip the permission dialog.
			json.NewEncoder(w).Encode(map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName":            "PreToolUse",
					"permissionDecision":       "allow",
					"permissionDecisionReason": "Auto-approved by Tower batch policy (read-only)",
				},
			})
		} else {
			// Non-read-only: pass through, let PermissionRequest handle it.
			json.NewEncoder(w).Encode(map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName": "PreToolUse",
				},
			})
		}
	case "permission-request":
		// Stub: allow all. Real implementation will consult policy + cockpit.
		json.NewEncoder(w).Encode(map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName": "PermissionRequest",
				"decision": map[string]any{
					"behavior": "allow",
				},
			},
		})
	}
}

func (s *Server) validAuth(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	return strings.TrimPrefix(auth, "Bearer ") == s.token
}
