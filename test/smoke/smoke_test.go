//go:build smoke

package smoke

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tower/internal/adapters/claude"
	"tower/internal/daemon"
)

// TestSmokeClaudeHookIntegration runs a real Claude Code session against a live
// Tower daemon, verifying that hook events flow end-to-end. It requires:
//   - Claude CLI installed and on PATH
//   - Valid API credentials in the environment
//   - Explicit invocation: go test -run TestSmoke -count=1 ./test/smoke/
//
// Skipped automatically under go test -short or when claude binary is missing.
func TestSmokeClaudeHookIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode: requires real Claude binary and API access")
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude binary not found on PATH: ", err)
	}
	t.Logf("using claude at: %s", claudePath)

	// --- 1. Start daemon on ephemeral port ---
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "daemon.lock")

	d, err := daemon.Start(lockPath)
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer d.Stop(context.Background())
	t.Logf("daemon listening on port %d", d.Port())

	// --- 2. Create project dir and generate hook config ---
	sessionID := fmt.Sprintf("smoke-%d", time.Now().UnixNano())
	d.Server().RegisterSession(sessionID)

	projectDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	cfg := claude.GenerateHookConfig(sessionID, d.Port())
	hookPath, err := claude.WriteHookConfig(projectDir, cfg)
	if err != nil {
		t.Fatalf("write hook config: %v", err)
	}
	t.Logf("hook config written to: %s", hookPath)

	// --- 3. Spawn Claude with a deterministic prompt ---
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	prompt := "Read the current directory with ls, then create a file called tower-test.txt with the text 'hello from tower smoke test', then read it back"

	cmd := exec.CommandContext(ctx, claudePath,
		"-p", prompt,
		"--model", "haiku",
		"--allowedTools", "Bash,Write,Read",
	)
	cmd.Dir = projectDir

	// Build env: inherit system env + set TOWER_HOOK_TOKEN.
	// Strip CLAUDECODE to avoid "nested session" rejection when running inside Claude Code.
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	env = append(env, "TOWER_HOOK_TOKEN="+d.Token())
	cmd.Env = env

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Log("starting claude session...")
	startTime := time.Now()

	if err := cmd.Run(); err != nil {
		t.Logf("claude stdout:\n%s", stdout.String())
		t.Logf("claude stderr:\n%s", stderr.String())
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatal("claude timed out after 120s")
		}
		t.Fatalf("claude exited with error: %v", err)
	}

	elapsed := time.Since(startTime)
	t.Logf("claude completed in %s", elapsed.Round(time.Millisecond))
	t.Logf("claude stdout:\n%s", stdout.String())

	// --- 4. Verify Claude created the expected file ---
	testFile := filepath.Join(projectDir, "tower-test.txt")
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("expected tower-test.txt to exist: %v", err)
	}
	if !strings.Contains(string(content), "hello from tower smoke test") {
		t.Fatalf("tower-test.txt content mismatch: %q", string(content))
	}
	t.Log("tower-test.txt verified")

	// --- 5. Verify daemon received hook events ---
	events := d.Server().ReceivedEvents(sessionID)
	t.Logf("received %d hook events", len(events))

	if len(events) == 0 {
		t.Fatal("daemon received zero events — hooks did not fire")
	}

	// Build a set of event types we saw.
	eventTypes := make(map[string]int)
	for _, e := range events {
		eventTypes[e.HookEventName]++
	}
	t.Logf("event type counts: %v", eventTypes)

	// We expect at minimum: PreToolUse and PostToolUse for the tool calls Claude made.
	// The exact set depends on Claude's behavior, but these are the hard requirements.
	requiredEvents := []string{"PreToolUse", "PostToolUse"}
	for _, req := range requiredEvents {
		if eventTypes[req] == 0 {
			t.Errorf("missing required event type: %s", req)
		}
	}

	// --- 6. Verify risk classification on captured events ---
	for _, e := range events {
		if e.HookEventName != "PreToolUse" {
			continue
		}
		risk := daemon.ClassifyRisk(e.ToolName, e.ToolInput)
		t.Logf("  PreToolUse tool=%s risk=%s", e.ToolName, risk)

		// Read/Glob/Grep should always be read_only.
		switch e.ToolName {
		case "Read", "Glob", "Grep":
			if risk != "read_only" {
				t.Errorf("expected read_only for %s, got %s", e.ToolName, risk)
			}
		case "Write", "Edit":
			if risk != "workspace_write" {
				t.Errorf("expected workspace_write for %s, got %s", e.ToolName, risk)
			}
		}
	}

	// --- 7. Save captured events to test/fixtures/hooks/ ---
	// Save relative to the repo root (two levels up from test/smoke/).
	fixtureDir := filepath.Join("..", "..", "test", "fixtures", "hooks")
	if err := os.MkdirAll(fixtureDir, 0755); err != nil {
		t.Logf("warning: could not create fixture dir: %v", err)
	} else {
		capture := smokeCapture{
			CapturedAt: time.Now().UTC().Format(time.RFC3339),
			SessionID:  sessionID,
			EventCount: len(events),
			EventTypes: eventTypes,
			Events:     events,
			ClaudeArgs: []string{"-p", prompt, "--allowedTools", "Bash,Write,Read"},
			ElapsedMS:  elapsed.Milliseconds(),
		}
		data, err := json.MarshalIndent(capture, "", "  ")
		if err != nil {
			t.Logf("warning: could not marshal capture: %v", err)
		} else {
			fixturePath := filepath.Join(fixtureDir, fmt.Sprintf("smoke-%s.json",
				time.Now().UTC().Format("2006-01-02T150405")))
			if err := os.WriteFile(fixturePath, data, 0644); err != nil {
				t.Logf("warning: could not write fixture: %v", err)
			} else {
				t.Logf("fixture saved to: %s", fixturePath)
			}
		}
	}

	t.Logf("smoke test passed: %d events across %d types in %s",
		len(events), len(eventTypes), elapsed.Round(time.Millisecond))
}

// smokeCapture is the JSON shape written to test/fixtures/hooks/.
type smokeCapture struct {
	CapturedAt string            `json:"captured_at"`
	SessionID  string            `json:"session_id"`
	EventCount int               `json:"event_count"`
	EventTypes map[string]int    `json:"event_types"`
	Events     []daemon.HookEvent `json:"events"`
	ClaudeArgs []string          `json:"claude_args"`
	ElapsedMS  int64             `json:"elapsed_ms"`
}
