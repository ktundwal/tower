package runtime

import (
	"context"
	"errors"
	"time"

	"tower/internal/adapters/claude"
	"tower/internal/contracts"
	"tower/internal/daemon"
)

// ManagedManager orchestrates daemon registration, hook config injection,
// and process spawning for managed Claude sessions.
type ManagedManager struct {
	daemon *daemon.Daemon
}

// NewManagedManager creates a manager backed by a running daemon.
func NewManagedManager(d *daemon.Daemon) *ManagedManager {
	return &ManagedManager{daemon: d}
}

func (m *ManagedManager) LaunchManaged(ctx context.Context, request LaunchRequest) (LaunchHandle, error) {
	if request.Tool != "claude" {
		return LaunchHandle{}, errors.New("managed launch only supports claude")
	}

	sessionID := string(request.SessionID)

	// 1. Register session with daemon so hook POSTs are accepted.
	m.daemon.Server().RegisterSession(sessionID)

	// 2. Generate and write hook config.
	cfg := claude.GenerateHookConfig(sessionID, m.daemon.Port())
	if _, err := claude.WriteHookConfig(request.WorkingDir, cfg); err != nil {
		return LaunchHandle{}, err
	}

	// 3. Build descriptor.
	descriptor := contracts.SessionDescriptor{
		SessionID:     request.SessionID,
		RuntimeID:     request.RuntimeID,
		Adapter:       "claude",
		Tool:          "claude-code",
		AdapterRef:    contracts.AdapterRef("managed:claude"),
		ControlMode:   contracts.ControlModeManaged,
		WorkspaceRoot: request.WorkingDir,
		RepoRoot:      request.WorkingDir,
		Confidence:    contracts.ConfidenceCertain,
		Process: contracts.ProcessFingerprint{
			ExecutablePath: "claude-code",
			WorkspaceRoot:  request.WorkingDir,
			StartedAt:      time.Now().UTC(),
			TerminalID:     request.Terminal.WindowSession,
		},
		Terminal: request.Terminal,
	}

	return LaunchHandle{
		Descriptor: descriptor,
		Capabilities: contracts.NewCapabilitySet(
			contracts.CapabilityObserve,
			contracts.CapabilityJumpToTerminal,
			contracts.CapabilityApprove,
			contracts.CapabilityDeny,
			contracts.CapabilityBatchReadOnlyApproval,
		),
	}, nil
}

func (m *ManagedManager) Reconnect(context.Context) ([]contracts.SessionDescriptor, error) {
	return nil, nil
}
