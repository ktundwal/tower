package core

import (
	"context"
	"errors"
	"time"

	"tower/internal/contracts"
	towerruntime "tower/internal/runtime"
	"tower/internal/store"
)

var ErrUnsupportedManagedTool = errors.New("unsupported managed tool")

type Engine struct {
	repository store.Repository
	runtime    towerruntime.Manager
	adapters   map[string]contracts.SessionAdapter
	clock      func() time.Time
}

func NewEngine(repository store.Repository, runtime towerruntime.Manager) *Engine {
	return &Engine{
		repository: repository,
		runtime:    runtime,
		adapters:   make(map[string]contracts.SessionAdapter),
		clock:      func() time.Time { return time.Now().UTC() },
	}
}

func (engine *Engine) RegisterAdapter(name string, adapter contracts.SessionAdapter) {
	engine.adapters[name] = adapter
}

func (engine *Engine) LaunchManagedSession(
	ctx context.Context,
	tool string,
	args []string,
	workingDir string,
	environment map[string]string,
	terminal contracts.TerminalMetadata,
) (contracts.SessionSnapshot, error) {
	if tool != "claude" {
		return contracts.SessionSnapshot{}, ErrUnsupportedManagedTool
	}

	now := engine.clock()
	request := towerruntime.LaunchRequest{
		SessionID:   NewSessionID(now),
		RuntimeID:   NewRuntimeID(now),
		Tool:        tool,
		Args:        append([]string(nil), args...),
		WorkingDir:  workingDir,
		Environment: cloneMap(environment),
		Terminal:    terminal,
	}

	handle, err := engine.runtime.LaunchManaged(ctx, request)
	if err != nil {
		return contracts.SessionSnapshot{}, err
	}

	snapshot := contracts.SessionSnapshot{
		SessionDescriptor: handle.Descriptor,
		Lifecycle:         contracts.LifecycleLaunching,
		Activity:          contracts.ActivityIdle,
		Attention:         contracts.AttentionInfo,
		TaskExcerpt:       "Managed Claude launch path registered.",
		LastActivityAt:    now,
		SummaryExcerpt:    "PTY/ConPTY runtime helper and bridge are scaffolded but not implemented yet.",
		Capabilities:      handle.Capabilities,
	}

	startEvent := contracts.Event{
		SchemaVersion: contracts.SchemaVersionV1,
		EventID:       NewEventID(now),
		Kind:          contracts.EventSessionStarted,
		SessionID:     snapshot.SessionID,
		RuntimeID:     snapshot.RuntimeID,
		ControlMode:   contracts.ControlModeManaged,
		Source: contracts.EventSource{
			Adapter: snapshot.Adapter,
			Tool:    snapshot.Tool,
			Host:    "local",
		},
		OccurredAt:    now,
		IngestedAt:    now,
		Confidence:    contracts.ConfidenceCertain,
		CorrelationID: string(snapshot.SessionID),
		Payload: map[string]any{
			"argv":        append([]string{tool}, args...),
			"working_dir": workingDir,
		},
	}

	stateEvent := contracts.Event{
		SchemaVersion: contracts.SchemaVersionV1,
		EventID:       NewEventID(now.Add(time.Millisecond)),
		Kind:          contracts.EventStateChanged,
		SessionID:     snapshot.SessionID,
		RuntimeID:     snapshot.RuntimeID,
		ControlMode:   contracts.ControlModeManaged,
		Source:        startEvent.Source,
		OccurredAt:    now,
		IngestedAt:    now,
		Confidence:    contracts.ConfidenceCertain,
		CorrelationID: string(snapshot.SessionID),
		CausationID:   string(startEvent.EventID),
		Payload: map[string]any{
			"lifecycle": snapshot.Lifecycle,
			"activity":  snapshot.Activity,
			"attention": snapshot.Attention,
		},
	}

	if err := engine.repository.AppendEvent(ctx, startEvent); err != nil {
		return contracts.SessionSnapshot{}, err
	}
	if err := engine.repository.AppendEvent(ctx, stateEvent); err != nil {
		return contracts.SessionSnapshot{}, err
	}
	if err := engine.repository.SaveSnapshot(ctx, snapshot); err != nil {
		return contracts.SessionSnapshot{}, err
	}

	return snapshot, nil
}

func (engine *Engine) RecordEvent(ctx context.Context, event contracts.Event) error {
	now := engine.clock()
	if event.SchemaVersion == "" {
		event.SchemaVersion = contracts.SchemaVersionV1
	}
	if event.EventID == "" {
		event.EventID = NewEventID(now)
	}
	if event.IngestedAt.IsZero() {
		event.IngestedAt = now
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	if event.Payload == nil {
		event.Payload = make(map[string]any)
	}
	return engine.repository.AppendEvent(ctx, event)
}

func (engine *Engine) Snapshot(ctx context.Context, sessionID contracts.SessionID) (contracts.SessionSnapshot, error) {
	return engine.repository.Snapshot(ctx, sessionID)
}

func (engine *Engine) ListSessions(ctx context.Context) ([]contracts.SessionSnapshot, error) {
	return engine.repository.ListSnapshots(ctx)
}

func (engine *Engine) Perform(ctx context.Context, sessionID contracts.SessionID, action contracts.Action) (contracts.ActionResult, error) {
	snapshot, err := engine.repository.Snapshot(ctx, sessionID)
	if err != nil {
		return contracts.ActionResult{
			ActionID:    action.ID,
			Status:      contracts.ActionStatusFailed,
			Message:     err.Error(),
			CompletedAt: engine.clock(),
		}, err
	}

	result := contracts.ActionResult{
		ActionID:    action.ID,
		Status:      contracts.ActionStatusNoop,
		Message:     "bootstrap accepted the action boundary; runtime injection is still pending",
		CompletedAt: engine.clock(),
	}

	if capability := capabilityForAction(action.Kind); capability != "" && !snapshot.Capabilities.Has(capability) {
		result.Status = contracts.ActionStatusRejected
		result.Message = "capability not supported by this session"
	}
	if err := ctx.Err(); err != nil {
		result.Status = contracts.ActionStatusFailed
		result.Message = err.Error()
		return result, err
	}

	audit := contracts.AuditEntry{
		AuditID:   string(NewEventID(engine.clock())),
		SessionID: sessionID,
		ActionID:  action.ID,
		Operator:  action.RequestedBy,
		RequestSnapshot: map[string]any{
			"kind": action.Kind,
			"body": action.Body,
		},
		ContextSnapshot: map[string]any{
			"control_mode": snapshot.ControlMode,
			"lifecycle":    snapshot.Lifecycle,
			"activity":     snapshot.Activity,
		},
		Decision:   string(action.Kind),
		DecisionAt: engine.clock(),
		Result:     result.Status,
	}
	if err := engine.repository.RecordAudit(ctx, audit); err != nil {
		return contracts.ActionResult{}, err
	}

	return result, nil
}

func capabilityForAction(kind contracts.ActionKind) contracts.Capability {
	switch kind {
	case contracts.ActionApprove:
		return contracts.CapabilityApprove
	case contracts.ActionDeny:
		return contracts.CapabilityDeny
	case contracts.ActionRespond:
		return contracts.CapabilityRespond
	case contracts.ActionInjectCommand:
		return contracts.CapabilityInjectCommand
	case contracts.ActionJumpTerminal:
		return contracts.CapabilityJumpToTerminal
	case contracts.ActionJumpIDE:
		return contracts.CapabilityJumpToIDE
	case contracts.ActionParkSession:
		return contracts.CapabilityPark
	case contracts.ActionResumeSession:
		return contracts.CapabilityResume
	default:
		return ""
	}
}

func cloneMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
