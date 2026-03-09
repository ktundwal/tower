package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// hookEventDef maps a Claude hook event name to its URL slug and whether it's async.
type hookEventDef struct {
	Name    string // Claude event name (PascalCase)
	Slug    string // URL path segment (kebab-case)
	Async   bool
	Timeout int // seconds, 0 means use default (30)
}

var hookEvents = []hookEventDef{
	{Name: "PreToolUse", Slug: "pre-tool-use", Async: false, Timeout: 30},
	{Name: "PermissionRequest", Slug: "permission-request", Async: false, Timeout: 600},
	{Name: "PostToolUse", Slug: "post-tool-use", Async: true, Timeout: 30},
	{Name: "PostToolUseFailure", Slug: "post-tool-use-failure", Async: true, Timeout: 30},
	{Name: "SessionStart", Slug: "session-start", Async: true, Timeout: 30},
	{Name: "SessionEnd", Slug: "session-end", Async: true, Timeout: 30},
	{Name: "Stop", Slug: "stop", Async: true, Timeout: 30},
	{Name: "Notification", Slug: "notification", Async: true, Timeout: 30},
	{Name: "SubagentStart", Slug: "subagent-start", Async: true, Timeout: 30},
	{Name: "SubagentStop", Slug: "subagent-stop", Async: true, Timeout: 30},
}

// GenerateHookConfig builds the settings.local.json structure for a managed session.
func GenerateHookConfig(sessionID string, port int) map[string]any {
	hooks := make(map[string]any, len(hookEvents))

	for _, def := range hookEvents {
		url := fmt.Sprintf("http://localhost:%d/hooks/%s/%s", port, sessionID, def.Slug)

		hook := map[string]any{
			"type":    "http",
			"url":     url,
			"timeout": def.Timeout,
			"headers": map[string]any{
				"Authorization": "Bearer $TOWER_HOOK_TOKEN",
			},
			"allowedEnvVars": []any{"TOWER_HOOK_TOKEN"},
		}
		if def.Async {
			hook["async"] = true
		}

		hooks[def.Name] = []any{
			map[string]any{
				"hooks": []any{hook},
			},
		}
	}

	return map[string]any{
		"hooks": hooks,
	}
}

// WriteHookConfig writes the hook config to .claude/settings.local.json in the project dir.
// Returns the full path of the written file.
func WriteHookConfig(projectDir string, config map[string]any) (string, error) {
	dir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create .claude dir: %w", err)
	}

	path := filepath.Join(dir, "settings.local.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal hook config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("write hook config: %w", err)
	}

	return path, nil
}
