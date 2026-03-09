package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestDaemonStartAndHealth(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	d, err := Start(lockPath)
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer d.Stop(context.Background())

	// Health check via real HTTP.
	url := fmt.Sprintf("http://localhost:%d/healthz", d.Port())
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %q", body["status"])
	}
}

func TestDaemonWritesLockfile(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	d, err := Start(lockPath)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer d.Stop(context.Background())

	info, err := ReadLockfile(lockPath)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}
	if info.Port != d.Port() {
		t.Fatalf("lockfile port %d != daemon port %d", info.Port, d.Port())
	}
	if info.Token == "" {
		t.Fatal("lockfile token is empty")
	}
}

func TestDaemonStopRemovesLockfile(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	d, err := Start(lockPath)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := d.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Lockfile should be gone.
	_, err = ReadLockfile(lockPath)
	if err == nil {
		t.Fatal("expected lockfile to be removed after stop")
	}
}

func TestDaemonStopRefusesNewConnections(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "daemon.lock")

	d, err := Start(lockPath)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	port := d.Port()

	if err := d.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Give the listener a moment to close.
	time.Sleep(50 * time.Millisecond)

	// Connection should fail.
	_, err = http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	if err == nil {
		t.Fatal("expected connection refused after stop")
	}
}

func TestDaemonTokenIsRandom(t *testing.T) {
	dir := t.TempDir()

	d1, err := Start(filepath.Join(dir, "d1.lock"))
	if err != nil {
		t.Fatalf("start d1: %v", err)
	}
	defer d1.Stop(context.Background())

	d2, err := Start(filepath.Join(dir, "d2.lock"))
	if err != nil {
		t.Fatalf("start d2: %v", err)
	}
	defer d2.Stop(context.Background())

	if d1.Token() == d2.Token() {
		t.Fatal("two daemons got the same token — should be random")
	}
}
