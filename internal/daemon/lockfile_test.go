package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadLockfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.lock")

	info := LockInfo{
		Port:  7832,
		Token: "secret-token-123",
		PID:   12345,
	}

	if err := WriteLockfile(path, info); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	got, err := ReadLockfile(path)
	if err != nil {
		t.Fatalf("read lockfile: %v", err)
	}

	if got.Port != info.Port {
		t.Fatalf("port: expected %d, got %d", info.Port, got.Port)
	}
	if got.Token != info.Token {
		t.Fatalf("token: expected %q, got %q", info.Token, got.Token)
	}
	if got.PID != info.PID {
		t.Fatalf("pid: expected %d, got %d", info.PID, got.PID)
	}
}

func TestReadLockfileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.lock")

	_, err := ReadLockfile(path)
	if err == nil {
		t.Fatal("expected error for missing lockfile")
	}
}

func TestWriteLockfileCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "daemon.lock")

	info := LockInfo{Port: 9000, Token: "tok", PID: 1}

	if err := WriteLockfile(path, info); err != nil {
		t.Fatalf("write lockfile with missing parent: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lockfile not created: %v", err)
	}
}

func TestRemoveLockfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.lock")

	info := LockInfo{Port: 7832, Token: "tok", PID: 1}
	if err := WriteLockfile(path, info); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := RemoveLockfile(path); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("lockfile still exists after remove")
	}
}

func TestRemoveLockfileMissingIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.lock")

	if err := RemoveLockfile(path); err != nil {
		t.Fatalf("remove nonexistent should be noop, got: %v", err)
	}
}
