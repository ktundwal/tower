package contracts

import "time"

type Capability string

const (
	CapabilityObserve               Capability = "observe"
	CapabilityJumpToTerminal        Capability = "jump_terminal"
	CapabilityJumpToIDE             Capability = "jump_ide"
	CapabilityApprove               Capability = "approve"
	CapabilityDeny                  Capability = "deny"
	CapabilityRespond               Capability = "respond"
	CapabilityInjectCommand         Capability = "inject_command"
	CapabilityPark                  Capability = "park"
	CapabilityResume                Capability = "resume"
	CapabilityBatchReadOnlyApproval Capability = "batch_read_only_approval"
)

type CapabilitySet struct {
	Supported []Capability `json:"supported"`
}

func NewCapabilitySet(values ...Capability) CapabilitySet {
	seen := make(map[Capability]struct{}, len(values))
	supported := make([]Capability, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		supported = append(supported, value)
	}
	return CapabilitySet{Supported: supported}
}

func (set CapabilitySet) Has(capability Capability) bool {
	for _, supported := range set.Supported {
		if supported == capability {
			return true
		}
	}
	return false
}

type RiskClass string

const (
	RiskClassReadOnly       RiskClass = "read_only"
	RiskClassWorkspaceWrite RiskClass = "workspace_write"
	RiskClassGitRead        RiskClass = "git_read"
	RiskClassGitMutation    RiskClass = "git_mutation"
	RiskClassPackageInstall RiskClass = "package_install"
	RiskClassNetworkRead    RiskClass = "network_read"
	RiskClassNetworkWrite   RiskClass = "network_write"
	RiskClassProcessExec    RiskClass = "process_exec"
	RiskClassSecretAccess   RiskClass = "secret_access"
	RiskClassUnknown        RiskClass = "unknown"
)

type ApprovalKind string

const ApprovalKindToolCall ApprovalKind = "tool_call"

type ApprovalResolution string

const (
	ApprovalResolutionApproved    ApprovalResolution = "approved"
	ApprovalResolutionDenied      ApprovalResolution = "denied"
	ApprovalResolutionMessageSent ApprovalResolution = "message_sent"
	ApprovalResolutionSuperseded  ApprovalResolution = "superseded"
	ApprovalResolutionExpired     ApprovalResolution = "expired"
	ApprovalResolutionCancelled   ApprovalResolution = "cancelled"
)

type ApprovalRequest struct {
	ApprovalID         ApprovalID    `json:"action_id"`
	ApprovalKind       ApprovalKind  `json:"approval_kind"`
	ToolName           string        `json:"tool_name"`
	RiskClass          RiskClass     `json:"risk_class"`
	PromptExcerpt      string        `json:"prompt_excerpt"`
	PromptFingerprint  string        `json:"prompt_fingerprint"`
	Detector           string        `json:"detector"`
	InputEpochAtPrompt uint64        `json:"input_epoch_at_prompt"`
	ConfirmedWaitingAt time.Time     `json:"confirmed_waiting_at"`
	FreshUntil         time.Time     `json:"fresh_until"`
	DecisionOptions    []string      `json:"decision_options"`
	NormalizedKey      string        `json:"normalized_key,omitempty"`
	ToolArgs           map[string]any `json:"tool_args,omitempty"`
	CWD                string        `json:"cwd,omitempty"`
	RepoRoot           string        `json:"repo_root,omitempty"`
	HookEventID        string        `json:"hook_event_id,omitempty"`
	DisplayTitle       string        `json:"display_title,omitempty"`
	DisplaySubtitle    string        `json:"display_subtitle,omitempty"`
}

type GitOperationClass string

const (
	GitOperationClassRead   GitOperationClass = "read"
	GitOperationClassWrite  GitOperationClass = "write"
	GitOperationClassMerge  GitOperationClass = "merge"
	GitOperationClassRebase GitOperationClass = "rebase"
	GitOperationClassOther  GitOperationClass = "other"
)

type ConflictRecord struct {
	RepoRoot          string            `json:"repo_root,omitempty"`
	BranchName        string            `json:"branch_name,omitempty"`
	TouchedPaths      []string          `json:"touched_paths,omitempty"`
	GitOperationClass GitOperationClass `json:"git_operation_class,omitempty"`
	TaskExcerpt       string            `json:"task_excerpt,omitempty"`
}

type ActionKind string

const (
	ActionApprove       ActionKind = "approve"
	ActionDeny          ActionKind = "deny"
	ActionRespond       ActionKind = "respond"
	ActionInjectCommand ActionKind = "inject_command"
	ActionJumpTerminal  ActionKind = "jump_terminal"
	ActionJumpIDE       ActionKind = "jump_ide"
	ActionParkSession   ActionKind = "park_session"
	ActionResumeSession ActionKind = "resume_session"
)

type Action struct {
	ID          ActionID           `json:"id"`
	Kind        ActionKind         `json:"kind"`
	ApprovalID  ApprovalID         `json:"approval_id,omitempty"`
	Body        string             `json:"body,omitempty"`
	RequestedBy string             `json:"requested_by,omitempty"`
	RequestedAt time.Time          `json:"requested_at,omitempty"`
	Metadata    map[string]string  `json:"metadata,omitempty"`
}

type ActionStatus string

const (
	ActionStatusAccepted ActionStatus = "accepted"
	ActionStatusApplied  ActionStatus = "applied"
	ActionStatusNoop     ActionStatus = "noop"
	ActionStatusRejected ActionStatus = "rejected"
	ActionStatusFailed   ActionStatus = "failed"
)

type ActionResult struct {
	ActionID    ActionID     `json:"action_id"`
	Status      ActionStatus `json:"status"`
	Message     string       `json:"message,omitempty"`
	EventIDs    []EventID    `json:"event_ids,omitempty"`
	CompletedAt time.Time    `json:"completed_at,omitempty"`
}

type AuditEntry struct {
	AuditID         string         `json:"audit_id"`
	SessionID       SessionID      `json:"session_id"`
	ActionID        ActionID       `json:"action_id"`
	Operator        string         `json:"operator,omitempty"`
	RequestSnapshot map[string]any `json:"request_snapshot,omitempty"`
	ContextSnapshot map[string]any `json:"context_snapshot,omitempty"`
	Decision        string         `json:"decision,omitempty"`
	DecisionAt      time.Time      `json:"decision_at,omitempty"`
	Result          ActionStatus   `json:"result"`
	ResultEventIDs  []EventID      `json:"result_event_ids,omitempty"`
}
