package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tower/internal/adapters/claude"
	"tower/internal/contracts"
	"tower/internal/core"
	"tower/internal/daemon"
	towerruntime "tower/internal/runtime"
	"tower/internal/store"
)

// TestManagedLaunchFlow exercises the full launch sequence that tower run claude performs:
//  1. Start daemon
//  2. Engine creates session
//  3. Register session with daemon
//  4. Generate + write hook config
//  5. Simulate Claude posting hooks to the daemon
//  6. Verify events arrive and session state is coherent
func TestManagedLaunchFlow(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")
	projectDir := filepath.Join(dir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// 1. Start daemon.
	d, err := daemon.Start(lockPath)
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer d.Stop(context.Background())

	// 2. Create session via engine (same path as bootstrap.runManaged).
	layout := store.Layout{
		DatabasePath:  filepath.Join(dir, "tower.db"),
		ParkedDir:     filepath.Join(dir, "parked"),
		ArtifactsDir:  filepath.Join(dir, "artifacts"),
	}
	repo := store.NewMemoryRepository(layout)
	engine := core.NewEngine(repo, towerruntime.NewBootstrapManager())

	ctx := context.Background()
	snapshot, err := engine.LaunchManagedSession(
		ctx,
		"claude",
		[]string{"--model", "opus"},
		projectDir,
		map[string]string{"TERM": "xterm-256color"},
		contracts.TerminalMetadata{Program: "wt", Columns: 120, Rows: 40},
	)
	if err != nil {
		t.Fatalf("launch session: %v", err)
	}

	sessionID := string(snapshot.SessionID)
	if sessionID == "" {
		t.Fatal("session ID is empty")
	}

	// 3. Register session with daemon.
	d.Server().RegisterSession(sessionID)

	// 4. Generate and write hook config.
	cfg := claude.GenerateHookConfig(sessionID, d.Port())
	hookPath, err := claude.WriteHookConfig(projectDir, cfg)
	if err != nil {
		t.Fatalf("write hook config: %v", err)
	}

	// Verify the file exists and is valid JSON.
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse hook config: %v", err)
	}
	hooks := parsed["hooks"].(map[string]any)
	if len(hooks) != 10 {
		t.Fatalf("expected 10 hook events, got %d", len(hooks))
	}

	// 5. Simulate Claude posting hooks.
	client := &http.Client{Timeout: 5 * time.Second}
	base := fmt.Sprintf("http://localhost:%d", d.Port())

	// SessionStart
	postHook(t, client, base, sessionID, "session-start", d.Token(), map[string]any{
		"hook_event_name": "SessionStart",
		"session_id":      "claude-abc",
	})

	// PreToolUse for a read-only op
	resp := postHookResp(t, client, base, sessionID, "pre-tool-use", d.Token(), map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Read",
		"tool_input":      map[string]any{"file_path": "/tmp/main.go"},
	})
	var preResult map[string]any
	json.NewDecoder(resp.Body).Decode(&preResult)
	resp.Body.Close()
	if preResult["hookSpecificOutput"] == nil {
		t.Fatal("PreToolUse should return hookSpecificOutput")
	}

	// PostToolUse
	postHook(t, client, base, sessionID, "post-tool-use", d.Token(), map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "Read",
		"tool_input":      map[string]any{"file_path": "/tmp/main.go"},
	})

	// PermissionRequest for a dangerous op
	resp = postHookResp(t, client, base, sessionID, "permission-request", d.Token(), map[string]any{
		"hook_event_name": "PermissionRequest",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "rm -rf /"},
	})
	var permResult map[string]any
	json.NewDecoder(resp.Body).Decode(&permResult)
	resp.Body.Close()
	decision := permResult["hookSpecificOutput"].(map[string]any)["decision"].(map[string]any)
	if decision["behavior"] != "allow" {
		t.Fatalf("stub should allow, got %v", decision["behavior"])
	}

	// 6. Verify daemon recorded all events.
	events := d.Server().ReceivedEvents(sessionID)
	if len(events) != 4 {
		t.Fatalf("expected 4 recorded events, got %d", len(events))
	}

	// Verify engine snapshot is in launching state.
	snap, err := engine.Snapshot(ctx, snapshot.SessionID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.Lifecycle != contracts.LifecycleLaunching {
		t.Fatalf("expected launching, got %s", snap.Lifecycle)
	}
	if snap.ControlMode != contracts.ControlModeManaged {
		t.Fatalf("expected managed, got %s", snap.ControlMode)
	}
}

func postHook(t *testing.T, client *http.Client, base, sessionID, eventType, token string, payload map[string]any) {
	t.Helper()
	resp := postHookResp(t, client, base, sessionID, eventType, token, payload)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST %s: expected 200, got %d", eventType, resp.StatusCode)
	}
}

func postHookResp(t *testing.T, client *http.Client, base, sessionID, eventType, token string, payload map[string]any) *http.Response {
	t.Helper()
	body, _ := json.Marshal(payload)
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
