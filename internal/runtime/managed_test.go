package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"

	"tower/internal/contracts"
	"tower/internal/daemon"
)

func TestManagedManagerLaunch(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	d, err := daemon.Start(lockPath)
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer d.Stop(context.Background())

	mgr := NewManagedManager(d)

	projectDir := filepath.Join(dir, "project")

	// Use a simple command that exits quickly instead of Claude.
	var request LaunchRequest
	if goruntime.GOOS == "windows" {
		request = LaunchRequest{
			SessionID:   "MGR-SID-1",
			RuntimeID:   "MGR-RID-1",
			Tool:        "claude",
			Args:        []string{},
			WorkingDir:  projectDir,
			Environment: map[string]string{},
			Terminal:    contracts.TerminalMetadata{Columns: 120, Rows: 40},
		}
	} else {
		request = LaunchRequest{
			SessionID:   "MGR-SID-1",
			RuntimeID:   "MGR-RID-1",
			Tool:        "claude",
			Args:        []string{},
			WorkingDir:  projectDir,
			Environment: map[string]string{},
			Terminal:    contracts.TerminalMetadata{Columns: 120, Rows: 40},
		}
	}

	handle, err := mgr.LaunchManaged(context.Background(), request)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}

	// Verify descriptor.
	if handle.Descriptor.SessionID != "MGR-SID-1" {
		t.Fatalf("session id: %q", handle.Descriptor.SessionID)
	}
	if handle.Descriptor.ControlMode != contracts.ControlModeManaged {
		t.Fatalf("control mode: %q", handle.Descriptor.ControlMode)
	}
	if !handle.Capabilities.Has(contracts.CapabilityApprove) {
		t.Fatal("expected approve capability")
	}
	if !handle.Capabilities.Has(contracts.CapabilityBatchReadOnlyApproval) {
		t.Fatal("expected batch read-only capability")
	}
}

func TestManagedManagerRegistersSession(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	d, err := daemon.Start(lockPath)
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer d.Stop(context.Background())

	mgr := NewManagedManager(d)

	request := LaunchRequest{
		SessionID:  "REG-SID-1",
		RuntimeID:  "REG-RID-1",
		Tool:       "claude",
		WorkingDir: filepath.Join(dir, "project"),
		Terminal:   contracts.TerminalMetadata{},
	}

	mgr.LaunchManaged(context.Background(), request)

	// Post a hook event and verify it's recorded (not silently dropped as unknown session).
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://localhost:%d/hooks/REG-SID-1/post-tool-use", d.Port())
	body := []byte(`{"hook_event_name":"PostToolUse","tool_name":"Read"}`)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+d.Token())
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST hook: %v", err)
	}
	resp.Body.Close()

	events := d.Server().ReceivedEvents("REG-SID-1")
	if len(events) != 1 {
		t.Fatalf("expected 1 recorded event (session is registered), got %d", len(events))
	}
}

func TestManagedManagerWritesHookConfig(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")
	projectDir := filepath.Join(dir, "project")

	d, err := daemon.Start(lockPath)
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer d.Stop(context.Background())

	mgr := NewManagedManager(d)

	request := LaunchRequest{
		SessionID:  "HC-SID-1",
		RuntimeID:  "HC-RID-1",
		Tool:       "claude",
		WorkingDir: projectDir,
		Terminal:   contracts.TerminalMetadata{},
	}

	mgr.LaunchManaged(context.Background(), request)

	// Verify hook config file was written.
	hookPath := filepath.Join(projectDir, ".claude", "settings.local.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook config: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse hook config: %v", err)
	}

	hooks, ok := cfg["hooks"].(map[string]any)
	if !ok {
		t.Fatal("expected hooks key")
	}
	if len(hooks) != 10 {
		t.Fatalf("expected 10 hook events, got %d", len(hooks))
	}
}

func TestManagedManagerRejectsNonClaude(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	d, err := daemon.Start(lockPath)
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer d.Stop(context.Background())

	mgr := NewManagedManager(d)

	request := LaunchRequest{
		SessionID: "X-SID-1",
		RuntimeID: "X-RID-1",
		Tool:      "copilot",
	}

	_, err = mgr.LaunchManaged(context.Background(), request)
	if err == nil {
		t.Fatal("expected error for non-claude tool")
	}
}
