package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// ManagedProcess wraps an os/exec.Cmd with Tower-specific metadata.
// In v1 passthrough mode this uses pipes. ConPTY/PTY can be layered in
// when terminal resize and ANSI passthrough are needed.
type ManagedProcess struct {
	cmd       *exec.Cmd
	stdout    bytes.Buffer
	stderr    bytes.Buffer
	pid       int
	startedAt time.Time
}

// SpawnProcess launches a child process per the spec and returns immediately.
// The caller must call Wait() to collect output and exit status.
func SpawnProcess(ctx context.Context, spec SpawnSpec) (*ManagedProcess, error) {
	executable, err := exec.LookPath(spec.Executable)
	if err != nil {
		return nil, fmt.Errorf("resolve executable %q: %w", spec.Executable, err)
	}

	cmd := exec.CommandContext(ctx, executable, spec.Args...)

	if spec.WorkingDir != "" {
		cmd.Dir = spec.WorkingDir
	}

	// Start with the current process environment, then overlay spec environment.
	cmd.Env = buildEnv(spec.Environment)

	proc := &ManagedProcess{cmd: cmd}
	cmd.Stdout = &proc.stdout
	cmd.Stderr = &proc.stderr

	proc.startedAt = time.Now().UTC()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}
	proc.pid = cmd.Process.Pid

	return proc, nil
}

// PID returns the child process ID.
func (p *ManagedProcess) PID() int { return p.pid }

// StartedAt returns when the process was spawned.
func (p *ManagedProcess) StartedAt() time.Time { return p.startedAt }

// Wait blocks until the process exits and returns combined stdout+stderr.
func (p *ManagedProcess) Wait() (string, error) {
	err := p.cmd.Wait()
	output := p.stdout.String() + p.stderr.String()
	return output, err
}

// BuildTowerEnv creates the Tower-injected environment variables per design doc section 5.3.
func BuildTowerEnv(sessionID, runtimeID string, daemonPort int, hookToken string) map[string]string {
	return map[string]string{
		"TOWER_MANAGED":    "1",
		"TOWER_SESSION_ID": sessionID,
		"TOWER_RUNTIME_ID": runtimeID,
		"TOWER_DAEMON_PORT": strconv.Itoa(daemonPort),
		"TOWER_HOOK_TOKEN": hookToken,
	}
}

// buildEnv merges the current process environment with the overlay map.
func buildEnv(overlay map[string]string) []string {
	env := os.Environ()
	for k, v := range overlay {
		env = append(env, k+"="+v)
	}
	return env
}
