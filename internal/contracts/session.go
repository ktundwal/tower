package contracts

import "time"

type SessionID string
type RuntimeID string
type EventID string
type ActionID string
type ApprovalID string
type AdapterRef string
type SchemaVersion string

const SchemaVersionV1 SchemaVersion = "v1"

type ControlMode string

const (
	ControlModeManaged  ControlMode = "managed"
	ControlModeObserved ControlMode = "observed"
)

type Lifecycle string

const (
	LifecycleDiscovered Lifecycle = "discovered"
	LifecycleLaunching  Lifecycle = "launching"
	LifecycleActive     Lifecycle = "active"
	LifecycleParked     Lifecycle = "parked"
	LifecycleCompleted  Lifecycle = "completed"
	LifecycleFailed     Lifecycle = "failed"
	LifecycleDetached   Lifecycle = "detached"
)

type Activity string

const (
	ActivityRunning         Activity = "running"
	ActivityWaitingHuman    Activity = "waiting_human"
	ActivityWaitingTool     Activity = "waiting_tool"
	ActivityWaitingExternal Activity = "waiting_external"
	ActivityIdle            Activity = "idle"
	ActivityUnknown         Activity = "unknown"
)

type Confidence string

const (
	ConfidenceCertain Confidence = "certain"
	ConfidenceHigh    Confidence = "high"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceLow     Confidence = "low"
)

type Attention string

const (
	AttentionNone      Attention = "none"
	AttentionInfo      Attention = "info"
	AttentionNeedsUser Attention = "needs_user"
	AttentionUrgent    Attention = "urgent"
)

type ProcessFingerprint struct {
	ExecutablePath string    `json:"executable_path,omitempty"`
	PID            int       `json:"pid,omitempty"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	WorkspaceRoot  string    `json:"workspace_root,omitempty"`
	TerminalID     string    `json:"terminal_id,omitempty"`
}

type TerminalMetadata struct {
	Program       string `json:"program,omitempty"`
	WindowSession string `json:"window_session,omitempty"`
	DeviceName    string `json:"device_name,omitempty"`
	Columns       int    `json:"columns,omitempty"`
	Rows          int    `json:"rows,omitempty"`
}

type SessionDescriptor struct {
	SessionID     SessionID          `json:"session_id,omitempty"`
	RuntimeID     RuntimeID          `json:"runtime_id,omitempty"`
	Adapter       string             `json:"adapter"`
	Tool          string             `json:"tool"`
	AdapterRef    AdapterRef         `json:"adapter_ref,omitempty"`
	ControlMode   ControlMode        `json:"control_mode"`
	WorkspaceRoot string             `json:"workspace_root,omitempty"`
	RepoRoot      string             `json:"repo_root,omitempty"`
	BranchName    string             `json:"branch_name,omitempty"`
	Confidence    Confidence         `json:"confidence,omitempty"`
	Process       ProcessFingerprint `json:"process"`
	Terminal      TerminalMetadata   `json:"terminal"`
}

type SessionSnapshot struct {
	SessionDescriptor
	Lifecycle          Lifecycle     `json:"lifecycle"`
	Activity           Activity      `json:"activity"`
	Attention          Attention     `json:"attention"`
	TaskExcerpt        string        `json:"task_excerpt,omitempty"`
	PendingActionCount int           `json:"pending_action_count,omitempty"`
	ConflictCount      int           `json:"conflict_count,omitempty"`
	LastActivityAt     time.Time     `json:"last_activity_at,omitempty"`
	SummaryExcerpt     string        `json:"summary_excerpt,omitempty"`
	Capabilities       CapabilitySet `json:"capabilities"`
}
