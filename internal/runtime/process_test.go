package runtime

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSpawnAndWait(t *testing.T) {
	var spec SpawnSpec
	if runtime.GOOS == "windows" {
		spec = SpawnSpec{
			Executable: "cmd.exe",
			Args:       []string{"/C", "echo", "hello"},
		}
	} else {
		spec = SpawnSpec{
			Executable: "echo",
			Args:       []string{"hello"},
		}
	}

	ctx := context.Background()
	proc, err := SpawnProcess(ctx, spec)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	if proc.PID() <= 0 {
		t.Fatalf("expected positive PID, got %d", proc.PID())
	}
	if proc.StartedAt().IsZero() {
		t.Fatal("started_at should not be zero")
	}

	output, err := proc.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !strings.Contains(output, "hello") {
		t.Fatalf("expected output containing 'hello', got %q", output)
	}
}

func TestSpawnInjectsEnvironment(t *testing.T) {
	var spec SpawnSpec
	if runtime.GOOS == "windows" {
		spec = SpawnSpec{
			Executable: "cmd.exe",
			Args:       []string{"/C", "echo", "%TOWER_SESSION_ID%"},
			Environment: map[string]string{
				"TOWER_SESSION_ID": "TEST-SESSION-42",
			},
		}
	} else {
		spec = SpawnSpec{
			Executable: "sh",
			Args:       []string{"-c", "echo $TOWER_SESSION_ID"},
			Environment: map[string]string{
				"TOWER_SESSION_ID": "TEST-SESSION-42",
			},
		}
	}

	ctx := context.Background()
	proc, err := SpawnProcess(ctx, spec)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	output, err := proc.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !strings.Contains(output, "TEST-SESSION-42") {
		t.Fatalf("expected env var in output, got %q", output)
	}
}

func TestSpawnSetsWorkingDir(t *testing.T) {
	dir := t.TempDir()

	var spec SpawnSpec
	if runtime.GOOS == "windows" {
		spec = SpawnSpec{
			Executable: "cmd.exe",
			Args:       []string{"/C", "cd"},
			WorkingDir: dir,
		}
	} else {
		spec = SpawnSpec{
			Executable: "pwd",
			WorkingDir: dir,
		}
	}

	ctx := context.Background()
	proc, err := SpawnProcess(ctx, spec)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	output, err := proc.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	// Normalize for comparison.
	output = strings.TrimSpace(strings.ReplaceAll(output, "\\", "/"))
	dirNorm := strings.ReplaceAll(dir, "\\", "/")
	if !strings.Contains(strings.ToLower(output), strings.ToLower(dirNorm)) {
		t.Fatalf("expected working dir %q in output, got %q", dirNorm, output)
	}
}

func TestSpawnBadExecutableFails(t *testing.T) {
	spec := SpawnSpec{
		Executable: "nonexistent-binary-that-doesnt-exist",
	}

	ctx := context.Background()
	_, err := SpawnProcess(ctx, spec)
	if err == nil {
		t.Fatal("expected error for nonexistent executable")
	}
}

func TestSpawnExitCode(t *testing.T) {
	var spec SpawnSpec
	if runtime.GOOS == "windows" {
		spec = SpawnSpec{
			Executable: "cmd.exe",
			Args:       []string{"/C", "exit", "1"},
		}
	} else {
		spec = SpawnSpec{
			Executable: "sh",
			Args:       []string{"-c", "exit 1"},
		}
	}

	ctx := context.Background()
	proc, err := SpawnProcess(ctx, spec)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	_, err = proc.Wait()
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestSpawnInjectsTowerEnvVars(t *testing.T) {
	env := BuildTowerEnv("SID-1", "RID-1", 7832, "secret-tok")

	if env["TOWER_MANAGED"] != "1" {
		t.Fatalf("expected TOWER_MANAGED=1, got %q", env["TOWER_MANAGED"])
	}
	if env["TOWER_SESSION_ID"] != "SID-1" {
		t.Fatalf("expected session id, got %q", env["TOWER_SESSION_ID"])
	}
	if env["TOWER_RUNTIME_ID"] != "RID-1" {
		t.Fatalf("expected runtime id, got %q", env["TOWER_RUNTIME_ID"])
	}
	if env["TOWER_DAEMON_PORT"] != "7832" {
		t.Fatalf("expected port, got %q", env["TOWER_DAEMON_PORT"])
	}
	if env["TOWER_HOOK_TOKEN"] != "secret-tok" {
		t.Fatalf("expected token, got %q", env["TOWER_HOOK_TOKEN"])
	}
}

func TestSpawnStartedAtIsRecent(t *testing.T) {
	before := time.Now()

	var spec SpawnSpec
	if runtime.GOOS == "windows" {
		spec = SpawnSpec{Executable: "cmd.exe", Args: []string{"/C", "echo", "ok"}}
	} else {
		spec = SpawnSpec{Executable: "echo", Args: []string{"ok"}}
	}

	proc, err := SpawnProcess(context.Background(), spec)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	proc.Wait()

	if proc.StartedAt().Before(before) {
		t.Fatalf("started_at %v is before test start %v", proc.StartedAt(), before)
	}
}
