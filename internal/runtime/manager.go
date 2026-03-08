package runtime

import (
	"context"
	"errors"
	"time"

	"tower/internal/contracts"
)

type LaunchRequest struct {
	SessionID   contracts.SessionID
	RuntimeID   contracts.RuntimeID
	Tool        string
	Args        []string
	WorkingDir  string
	Environment map[string]string
	Terminal    contracts.TerminalMetadata
}

type LaunchHandle struct {
	Descriptor   contracts.SessionDescriptor
	Capabilities contracts.CapabilitySet
}

type Manager interface {
	LaunchManaged(ctx context.Context, request LaunchRequest) (LaunchHandle, error)
	Reconnect(ctx context.Context) ([]contracts.SessionDescriptor, error)
}

type RuntimeSpec = LaunchRequest

type RuntimeHost interface {
	Start(ctx context.Context, spec RuntimeSpec) (RuntimeHandle, error)
	Reconnect(ctx context.Context, descriptor contracts.SessionDescriptor) (RuntimeHandle, error)
}

type RuntimeHandle interface {
	SessionID() contracts.SessionID
	RuntimeID() contracts.RuntimeID
	Events() <-chan contracts.Event
	Snapshot(ctx context.Context) (contracts.SessionSnapshot, error)
	AcquireLease(ctx context.Context, request LeaseRequest) (LeaseResult, error)
	InjectApproval(ctx context.Context, request ApprovalResponse) error
	InjectText(ctx context.Context, request TextInjection) error
	AttachTerminal(ctx context.Context, request TerminalAttachRequest) error
	DetachTerminal(ctx context.Context, reason string) error
	Terminate(ctx context.Context, force bool) error
}

type PTYBackend interface {
	Start(ctx context.Context, spec SpawnSpec) (ChildProcess, error)
	WriteInput(ctx context.Context, data []byte) error
	ReadOutput(ctx context.Context) ([]byte, error)
	Resize(columns uint16, rows uint16) error
	Close() error
}

type TerminalBridge interface {
	PauseInput(ctx context.Context) error
	ResumeInput(ctx context.Context) error
	LastInputEpoch() uint64
}

type ControlLease struct {
	SessionID         contracts.SessionID
	RuntimeID         contracts.RuntimeID
	InputEpoch        uint64
	PromptFingerprint string
	GrantedAt         time.Time
	ExpiresAt         time.Time
}

type LeaseRequest struct {
	ActionID          contracts.ActionID
	ApprovalID        contracts.ApprovalID
	PromptFingerprint string
	InputEpoch        uint64
	RequestedAt       time.Time
	ExpiresAt         time.Time
}

type LeaseResult struct {
	Granted bool
	Lease   ControlLease
	Reason  string
}

type ApprovalResponse struct {
	ActionID   contracts.ActionID
	ApprovalID contracts.ApprovalID
	Resolution contracts.ApprovalResolution
	Body       string
}

type TextInjection struct {
	ActionID contracts.ActionID
	Body     string
}

type TerminalAttachRequest struct {
	Terminal contracts.TerminalMetadata
}

type SpawnSpec struct {
	Executable  string
	Args        []string
	WorkingDir  string
	Environment map[string]string
}

type ChildProcess interface {
	PID() int
	StartedAt() time.Time
}

type BootstrapManager struct{}

func NewBootstrapManager() *BootstrapManager {
	return &BootstrapManager{}
}

func (manager *BootstrapManager) LaunchManaged(_ context.Context, request LaunchRequest) (LaunchHandle, error) {
	if request.Tool != "claude" {
		return LaunchHandle{}, errors.New("bootstrap managed runtime is only wired for tower run claude")
	}

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
			contracts.CapabilityRespond,
			contracts.CapabilityInjectCommand,
			contracts.CapabilityPark,
			contracts.CapabilityResume,
			contracts.CapabilityBatchReadOnlyApproval,
		),
	}, nil
}

func (manager *BootstrapManager) Reconnect(context.Context) ([]contracts.SessionDescriptor, error) {
	return nil, nil
}
