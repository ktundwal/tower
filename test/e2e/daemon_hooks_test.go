package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"tower/internal/adapters/claude"
	"tower/internal/daemon"
)

// TestDaemonHookRoundTrip exercises the full flow:
//  1. Start daemon on a real port
//  2. Register a session
//  3. Generate hook config pointing at the daemon
//  4. Simulate Claude's hook POSTs over real HTTP
//  5. Verify sync/async responses and event recording
func TestDaemonHookRoundTrip(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	// 1. Start daemon.
	d, err := daemon.Start(lockPath)
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer d.Stop(context.Background())

	sessionID := "E2E-SESSION-001"
	d.Server().RegisterSession(sessionID)

	// 2. Generate hook config.
	cfg := claude.GenerateHookConfig(sessionID, d.Port())

	// Write it (verifies the file is valid).
	projectDir := filepath.Join(dir, "project")
	_, err = claude.WriteHookConfig(projectDir, cfg)
	if err != nil {
		t.Fatalf("write hook config: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	base := fmt.Sprintf("http://localhost:%d", d.Port())

	// 3. Health check over real HTTP.
	t.Run("health", func(t *testing.T) {
		resp, err := client.Get(base + "/healthz")
		if err != nil {
			t.Fatalf("health: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("health status: %d", resp.StatusCode)
		}
	})

	// 4. Simulate async hook: PostToolUse.
	t.Run("async_post_tool_use", func(t *testing.T) {
		body := mustJSON(t, map[string]any{
			"session_id":      "claude-internal-id",
			"hook_event_name": "PostToolUse",
			"tool_name":       "Read",
			"tool_input":      map[string]any{"file_path": "/tmp/foo.go"},
		})

		resp := doHookPost(t, client, base, sessionID, "post-tool-use", d.Token(), body)
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		events := d.Server().ReceivedEvents(sessionID)
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].ToolName != "Read" {
			t.Fatalf("expected Read, got %q", events[0].ToolName)
		}
	})

	// 5. Simulate sync hook: PreToolUse (read-only) — should batch auto-approve.
	t.Run("sync_pre_tool_use_read_only", func(t *testing.T) {
		body := mustJSON(t, map[string]any{
			"session_id":      "claude-internal-id",
			"hook_event_name": "PreToolUse",
			"tool_name":       "Bash",
			"tool_input":      map[string]any{"command": "git status"},
		})

		resp := doHookPost(t, client, base, sessionID, "pre-tool-use", d.Token(), body)
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		output := result["hookSpecificOutput"].(map[string]any)
		if output["permissionDecision"] != "allow" {
			t.Fatalf("expected auto-approve for read-only, got %v", output["permissionDecision"])
		}
	})

	// 5b. PreToolUse (non-read-only) — should NOT auto-approve.
	t.Run("sync_pre_tool_use_write", func(t *testing.T) {
		body := mustJSON(t, map[string]any{
			"session_id":      "claude-internal-id",
			"hook_event_name": "PreToolUse",
			"tool_name":       "Edit",
			"tool_input":      map[string]any{"file_path": "/tmp/foo"},
		})

		resp := doHookPost(t, client, base, sessionID, "pre-tool-use", d.Token(), body)
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		output := result["hookSpecificOutput"].(map[string]any)
		if output["permissionDecision"] == "allow" {
			t.Fatal("should NOT auto-approve non-read-only Edit")
		}
	})

	// 6. Simulate sync hook: PermissionRequest — should get allow decision.
	t.Run("sync_permission_request", func(t *testing.T) {
		body := mustJSON(t, map[string]any{
			"session_id":      "claude-internal-id",
			"hook_event_name": "PermissionRequest",
			"tool_name":       "Bash",
			"tool_input":      map[string]any{"command": "rm -rf node_modules"},
		})

		resp := doHookPost(t, client, base, sessionID, "permission-request", d.Token(), body)
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		output := result["hookSpecificOutput"].(map[string]any)
		decision := output["decision"].(map[string]any)
		if decision["behavior"] != "allow" {
			t.Fatalf("expected allow, got %v", decision["behavior"])
		}
	})

	// 7. Simulate SessionStart async hook.
	t.Run("async_session_start", func(t *testing.T) {
		body := mustJSON(t, map[string]any{
			"session_id":      "claude-internal-id",
			"hook_event_name": "SessionStart",
		})

		resp := doHookPost(t, client, base, sessionID, "session-start", d.Token(), body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	// 8. Auth failure: wrong token rejected.
	t.Run("auth_failure", func(t *testing.T) {
		body := mustJSON(t, map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Read"})
		resp := doHookPost(t, client, base, sessionID, "post-tool-use", "bad-token", body)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	// 9. Unknown session returns 200 empty.
	t.Run("unknown_session", func(t *testing.T) {
		body := mustJSON(t, map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Read"})
		resp := doHookPost(t, client, base, "NONEXISTENT", "post-tool-use", d.Token(), body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	// 10. Verify total recorded events for the session.
	t.Run("total_events", func(t *testing.T) {
		events := d.Server().ReceivedEvents(sessionID)
		// async_post_tool_use(1) + sync_pre_tool_use_read_only(1) + sync_pre_tool_use_write(1) + sync_permission_request(1) + async_session_start(1) = 5
		if len(events) != 5 {
			t.Fatalf("expected 5 total events, got %d", len(events))
		}
	})
}

// TestDaemonLockfileRoundTrip verifies lockfile survives daemon start and
// another process can read it to discover port + token.
func TestDaemonLockfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	d, err := daemon.Start(lockPath)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer d.Stop(context.Background())

	// Read lockfile as a second process would.
	info, err := daemon.ReadLockfile(lockPath)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}

	// Use lockfile info to hit the daemon.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/healthz", info.Port))
	if err != nil {
		t.Fatalf("health via lockfile port: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func doHookPost(t *testing.T, client *http.Client, base, sessionID, eventType, token string, body []byte) *http.Response {
	t.Helper()
	url := fmt.Sprintf("%s/hooks/%s/%s", base, sessionID, eventType)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
