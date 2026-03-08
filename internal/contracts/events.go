package contracts

import "time"

type EventKind string

const (
	EventSessionDiscovered      EventKind = "session.discovered"
	EventSessionStarted         EventKind = "session.started"
	EventSessionReconnected     EventKind = "session.reconnected"
	EventSessionEnded           EventKind = "session.ended"
	EventStateChanged           EventKind = "state.changed"
	EventActivityExcerptUpdated EventKind = "activity.excerpt.updated"
	EventApprovalRequested      EventKind = "approval.requested"
	EventApprovalResolved       EventKind = "approval.resolved"
	EventCommandSent            EventKind = "command.sent"
	EventCommandApplied         EventKind = "command.applied"
	EventConflictDetected       EventKind = "conflict.detected"
	EventConflictResolved       EventKind = "conflict.resolved"
	EventSummaryUpdated         EventKind = "summary.updated"
	EventSessionParked          EventKind = "session.parked"
	EventSessionResumed         EventKind = "session.resumed"
	EventErrorReported          EventKind = "error.reported"
)

type EventSource struct {
	Adapter string `json:"adapter"`
	Tool    string `json:"tool"`
	Host    string `json:"host"`
}

type Event struct {
	SchemaVersion SchemaVersion   `json:"schema_version"`
	EventID       EventID         `json:"event_id"`
	Kind          EventKind       `json:"kind"`
	SessionID     SessionID       `json:"session_id"`
	RuntimeID     RuntimeID       `json:"runtime_id,omitempty"`
	ControlMode   ControlMode     `json:"control_mode"`
	Source        EventSource     `json:"source"`
	OccurredAt    time.Time       `json:"occurred_at"`
	IngestedAt    time.Time       `json:"ingested_at"`
	Confidence    Confidence      `json:"confidence"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
	Payload       map[string]any  `json:"payload"`
}
