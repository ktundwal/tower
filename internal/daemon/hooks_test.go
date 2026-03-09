package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAsyncHookRecordsEvent(t *testing.T) {
	srv := NewServer("tok")
	srv.RegisterSession("S1")

	body := []byte(`{
		"session_id": "abc123",
		"hook_event_name": "PostToolUse",
		"tool_name": "Read",
		"tool_input": {"file_path": "/tmp/foo"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/S1/post-tool-use", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	events := srv.ReceivedEvents("S1")
	if len(events) != 1 {
		t.Fatalf("expected 1 recorded event, got %d", len(events))
	}
	if events[0].HookEventName != "PostToolUse" {
		t.Fatalf("expected PostToolUse, got %q", events[0].HookEventName)
	}
	if events[0].ToolName != "Read" {
		t.Fatalf("expected Read, got %q", events[0].ToolName)
	}
}

func TestSyncHookRecordsEvent(t *testing.T) {
	srv := NewServer("tok")
	srv.RegisterSession("S1")

	body := []byte(`{
		"session_id": "abc123",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "git status"}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/S1/pre-tool-use", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	events := srv.ReceivedEvents("S1")
	if len(events) != 1 {
		t.Fatalf("expected 1 recorded event, got %d", len(events))
	}
	if events[0].ToolName != "Bash" {
		t.Fatalf("expected Bash, got %q", events[0].ToolName)
	}
}

func TestMultipleEventsAccumulatePerSession(t *testing.T) {
	srv := NewServer("tok")
	srv.RegisterSession("S1")
	srv.RegisterSession("S2")

	postEvent := func(sessionID, eventPath, toolName string) {
		body, _ := json.Marshal(map[string]any{
			"hook_event_name": "PostToolUse",
			"tool_name":       toolName,
		})
		req := httptest.NewRequest(http.MethodPost, "/hooks/"+sessionID+"/"+eventPath, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer tok")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
	}

	postEvent("S1", "post-tool-use", "Read")
	postEvent("S1", "post-tool-use", "Write")
	postEvent("S2", "post-tool-use", "Bash")

	if len(srv.ReceivedEvents("S1")) != 2 {
		t.Fatalf("expected 2 events for S1, got %d", len(srv.ReceivedEvents("S1")))
	}
	if len(srv.ReceivedEvents("S2")) != 1 {
		t.Fatalf("expected 1 event for S2, got %d", len(srv.ReceivedEvents("S2")))
	}
}

func TestUnknownSessionDoesNotRecordEvent(t *testing.T) {
	srv := NewServer("tok")
	// No session registered

	body := []byte(`{"hook_event_name":"PostToolUse","tool_name":"Read"}`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/UNKNOWN/post-tool-use", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(srv.ReceivedEvents("UNKNOWN")) != 0 {
		t.Fatalf("expected 0 events for unknown session, got %d", len(srv.ReceivedEvents("UNKNOWN")))
	}
}

func TestMalformedBodyReturns400(t *testing.T) {
	srv := NewServer("tok")
	srv.RegisterSession("S1")

	body := []byte(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/hooks/S1/post-tool-use", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed body, got %d", rec.Code)
	}
}
