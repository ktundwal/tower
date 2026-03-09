package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"tower/internal/contracts"
)

// fixtureEvent is the shape of each entry in the live capture fixture.
type fixtureEvent struct {
	Body      json.RawMessage `json:"body"`
	EventSlug string          `json:"event_slug"`
	Method    string          `json:"method"`
	Path      string          `json:"path"`
	RiskClass string          `json:"risk_class"`
}

func loadFixture(t *testing.T) []fixtureEvent {
	t.Helper()
	data, err := os.ReadFile("../../test/fixtures/hooks/live-capture-2026-03-09.json")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	var events []fixtureEvent
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return events
}

// TestFixtureParsesAllEvents verifies HookEvent can deserialize every
// real payload Claude sends without losing fields.
func TestFixtureParsesAllEvents(t *testing.T) {
	fixtures := loadFixture(t)

	for _, f := range fixtures {
		event, err := parseHookEvent(f.Body)
		if err != nil {
			t.Fatalf("parse %s body: %v", f.EventSlug, err)
		}
		if event.HookEventName == "" {
			t.Fatalf("event %s: hook_event_name is empty", f.EventSlug)
		}
	}
}

// TestFixtureRiskClassification verifies ClassifyRisk matches the risk_class
// recorded during the live capture for every event.
func TestFixtureRiskClassification(t *testing.T) {
	fixtures := loadFixture(t)

	for _, f := range fixtures {
		event, _ := parseHookEvent(f.Body)

		// Only tool-related events have meaningful risk classification.
		if event.ToolName == "" {
			continue
		}

		got := ClassifyRisk(event.ToolName, event.ToolInput)
		want := contracts.RiskClass(f.RiskClass)

		if got != want {
			t.Errorf("[%s] %s(%v): got %s, want %s",
				f.EventSlug, event.ToolName, event.ToolInput, got, want)
		}
	}
}

// TestFixtureFieldPreservation verifies that key fields survive the parse
// round-trip. If Claude adds new fields and we capture them in a fixture,
// this test ensures our struct keeps up.
func TestFixtureFieldPreservation(t *testing.T) {
	fixtures := loadFixture(t)

	for _, f := range fixtures {
		event, _ := parseHookEvent(f.Body)

		// All events must have these.
		if event.SessionID == "" {
			t.Errorf("%s: session_id missing", f.EventSlug)
		}

		switch event.HookEventName {
		case "PreToolUse":
			if event.ToolName == "" {
				t.Errorf("PreToolUse: tool_name missing")
			}
			if event.ToolUseID == "" {
				t.Errorf("PreToolUse: tool_use_id missing")
			}
			if event.PermissionMode == "" {
				t.Errorf("PreToolUse: permission_mode missing")
			}

		case "PostToolUse":
			if event.ToolName == "" {
				t.Errorf("PostToolUse: tool_name missing")
			}
			if event.ToolResponse == nil {
				t.Errorf("PostToolUse: tool_response missing")
			}

		case "PermissionRequest":
			if event.ToolName == "" {
				t.Errorf("PermissionRequest: tool_name missing")
			}
			if len(event.PermissionSuggestions) == 0 {
				t.Errorf("PermissionRequest: permission_suggestions missing")
			}

		case "Stop":
			if event.StopHookActive == nil {
				t.Errorf("Stop: stop_hook_active missing")
			}

		case "Notification":
			if event.NotificationType == "" {
				t.Errorf("Notification: notification_type missing")
			}
			if event.Message == "" {
				t.Errorf("Notification: message missing")
			}
		}
	}
}

// TestFixtureServerResponses replays every fixture event through the real
// Server and verifies the HTTP response matches expectations:
//   - sync events (pre-tool-use, permission-request) return decision JSON
//   - async events return 200 with empty body
//   - read-only PreToolUse gets batch auto-approve
func TestFixtureServerResponses(t *testing.T) {
	srv := NewServer("fixture-token")
	srv.RegisterSession("FIXTURE-SESSION")

	fixtures := loadFixture(t)

	for _, f := range fixtures {
		t.Run(f.EventSlug, func(t *testing.T) {
			// Rewrite the fixture path to use our test session ID.
			path := "/hooks/FIXTURE-SESSION/" + f.EventSlug

			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(f.Body))
			req.Header.Set("Authorization", "Bearer fixture-token")
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}

			switch f.EventSlug {
			case "pre-tool-use":
				// Must return JSON with hookSpecificOutput.
				var resp map[string]any
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				output, ok := resp["hookSpecificOutput"].(map[string]any)
				if !ok {
					t.Fatal("missing hookSpecificOutput")
				}

				// Check auto-approve for read-only.
				event, _ := parseHookEvent(f.Body)
				risk := ClassifyRisk(event.ToolName, event.ToolInput)
				if risk == contracts.RiskClassReadOnly {
					if output["permissionDecision"] != "allow" {
						t.Errorf("read-only %s should be auto-approved", event.ToolName)
					}
				} else {
					if output["permissionDecision"] == "allow" {
						t.Errorf("non-read-only %s should NOT be auto-approved", event.ToolName)
					}
				}

			case "permission-request":
				// Must return JSON with decision.
				var resp map[string]any
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				output := resp["hookSpecificOutput"].(map[string]any)
				decision := output["decision"].(map[string]any)
				if decision["behavior"] == nil {
					t.Fatal("permission-request response missing decision.behavior")
				}

			default:
				// Async events: empty body.
				if rec.Body.Len() != 0 {
					t.Errorf("async event %s should return empty body, got %q", f.EventSlug, rec.Body.String())
				}
			}
		})
	}
}
