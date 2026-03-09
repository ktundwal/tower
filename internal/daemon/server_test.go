package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	srv := NewServer("test-token")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestHealthEndpointNoAuth(t *testing.T) {
	srv := NewServer("test-token")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	// Health endpoint should not require auth
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", rec.Code)
	}
}

func TestHookEndpointRequiresAuth(t *testing.T) {
	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	body := []byte(`{"tool_name":"Read","tool_input":{"file_path":"/tmp/foo"}}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/post-tool-use", bytes.NewReader(body))
	// No Authorization header
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestHookEndpointRejectsWrongToken(t *testing.T) {
	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	body := []byte(`{"tool_name":"Read","tool_input":{"file_path":"/tmp/foo"}}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/post-tool-use", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", rec.Code)
	}
}

func TestHookEndpointAcceptsValidToken(t *testing.T) {
	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	body := []byte(`{"tool_name":"Read","tool_input":{"file_path":"/tmp/foo"}}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/post-tool-use", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	// Async endpoint returns 200 with empty body
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", rec.Code)
	}
}

func TestHookEndpointUnknownSessionReturns200(t *testing.T) {
	srv := NewServer("test-token")
	// Don't register any session

	body := []byte(`{"tool_name":"Read"}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/UNKNOWN/post-tool-use", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	// Per design doc: unknown session ID → 200 empty
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown session, got %d", rec.Code)
	}
}

func TestAsyncHookEndpointsReturn200Empty(t *testing.T) {
	asyncEvents := []string{
		"post-tool-use",
		"post-tool-use-failure",
		"session-start",
		"session-end",
		"stop",
		"notification",
		"subagent-start",
		"subagent-stop",
	}

	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	for _, event := range asyncEvents {
		t.Run(event, func(t *testing.T) {
			body := []byte(`{"tool_name":"Read"}`)
			req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/"+event, bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer test-token")
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d", event, rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Fatalf("expected empty body for %s, got %q", event, rec.Body.String())
			}
		})
	}
}

func TestPreToolUseReturnsDecisionJSON(t *testing.T) {
	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	body := []byte(`{"tool_name":"Read","tool_input":{"file_path":"/tmp/foo"}}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/pre-tool-use", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Should return valid JSON (not empty)
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON response for pre-tool-use: %v", err)
	}
}

func TestPermissionRequestReturnsDecisionJSON(t *testing.T) {
	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	body := []byte(`{
		"session_id":"abc123",
		"hook_event_name":"PermissionRequest",
		"tool_name":"Bash",
		"tool_input":{"command":"rm -rf node_modules"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/permission-request", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON response for permission-request: %v", err)
	}
}

func TestPreToolUseAutoApprovesReadOnly(t *testing.T) {
	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	tests := []struct {
		name     string
		toolName string
		input    map[string]any
	}{
		{"Read file", "Read", map[string]any{"file_path": "/tmp/foo"}},
		{"Glob pattern", "Glob", map[string]any{"pattern": "**/*.go"}},
		{"Grep search", "Grep", map[string]any{"pattern": "TODO"}},
		{"Bash git status", "Bash", map[string]any{"command": "git status"}},
		{"Bash ls", "Bash", map[string]any{"command": "ls -la"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       tt.toolName,
				"tool_input":      tt.input,
			})
			req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/pre-tool-use", bytes.NewReader(payload))
			req.Header.Set("Authorization", "Bearer test-token")
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}

			var result map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
				t.Fatalf("decode: %v", err)
			}

			output := result["hookSpecificOutput"].(map[string]any)
			if output["permissionDecision"] != "allow" {
				t.Fatalf("expected permissionDecision=allow for read-only %q, got %v", tt.name, output["permissionDecision"])
			}
			reason, _ := output["permissionDecisionReason"].(string)
			if reason == "" {
				t.Fatal("expected a reason for auto-approve")
			}
		})
	}
}

func TestPreToolUseDoesNotAutoApproveNonReadOnly(t *testing.T) {
	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	tests := []struct {
		name     string
		toolName string
		input    map[string]any
	}{
		{"Edit file", "Edit", map[string]any{"file_path": "/tmp/foo"}},
		{"Write file", "Write", map[string]any{"file_path": "/tmp/foo"}},
		{"Bash rm", "Bash", map[string]any{"command": "rm -rf node_modules"}},
		{"Bash git push", "Bash", map[string]any{"command": "git push origin main"}},
		{"Unknown tool", "SomeNewTool", map[string]any{"foo": "bar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       tt.toolName,
				"tool_input":      tt.input,
			})
			req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/pre-tool-use", bytes.NewReader(payload))
			req.Header.Set("Authorization", "Bearer test-token")
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}

			var result map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
				t.Fatalf("decode: %v", err)
			}

			output := result["hookSpecificOutput"].(map[string]any)
			// Non-read-only should NOT have permissionDecision=allow.
			if output["permissionDecision"] == "allow" {
				t.Fatalf("should NOT auto-approve non-read-only %q", tt.name)
			}
		})
	}
}

func TestUnknownHookEventReturns404(t *testing.T) {
	srv := NewServer("test-token")
	srv.RegisterSession("SESSION1")

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/SESSION1/bogus-event", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown event, got %d", rec.Code)
	}
}
