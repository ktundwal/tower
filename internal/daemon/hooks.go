package daemon

import "encoding/json"

// HookEvent represents a parsed hook event body from Claude Code.
// Field inventory derived from live-capture fixture at test/fixtures/hooks/live-capture-2026-03-09.json.
type HookEvent struct {
	// Common fields (present in all or most event types)
	SessionID      string `json:"session_id,omitempty"`
	HookEventName  string `json:"hook_event_name"`
	CWD            string `json:"cwd,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`

	// Tool-related fields (PreToolUse, PostToolUse, PermissionRequest)
	ToolName  string         `json:"tool_name,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`

	// PostToolUse: response from the tool execution (structure varies by tool)
	ToolResponse map[string]any `json:"tool_response,omitempty"`

	// PermissionRequest: suggested permission changes
	PermissionSuggestions []map[string]any `json:"permission_suggestions,omitempty"`

	// Stop event fields
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`
	StopHookActive       *bool  `json:"stop_hook_active,omitempty"`

	// Notification event fields
	Message          string `json:"message,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`
}

// parseHookEvent decodes a JSON body into a HookEvent.
func parseHookEvent(data []byte) (HookEvent, error) {
	var event HookEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return HookEvent{}, err
	}
	return event, nil
}
