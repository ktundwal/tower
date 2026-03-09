package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateHookConfig(t *testing.T) {
	cfg := GenerateHookConfig("SESSION1", 7832)

	// Should have all 10 hook event types.
	hooks, ok := cfg["hooks"].(map[string]any)
	if !ok {
		t.Fatal("expected hooks key in config")
	}

	expectedEvents := []string{
		"PreToolUse",
		"PermissionRequest",
		"PostToolUse",
		"PostToolUseFailure",
		"SessionStart",
		"SessionEnd",
		"Stop",
		"Notification",
		"SubagentStart",
		"SubagentStop",
	}

	for _, event := range expectedEvents {
		if _, ok := hooks[event]; !ok {
			t.Errorf("missing hook event %q", event)
		}
	}
}

func TestHookURLFormat(t *testing.T) {
	cfg := GenerateHookConfig("S1", 9000)
	hooks := cfg["hooks"].(map[string]any)

	// Check PreToolUse URL format.
	entries := hooks["PreToolUse"].([]any)
	entry := entries[0].(map[string]any)
	hookList := entry["hooks"].([]any)
	hook := hookList[0].(map[string]any)

	url, ok := hook["url"].(string)
	if !ok {
		t.Fatal("expected url string")
	}
	expected := "http://localhost:9000/hooks/S1/pre-tool-use"
	if url != expected {
		t.Fatalf("expected %q, got %q", expected, url)
	}
}

func TestSyncHooksAreNotAsync(t *testing.T) {
	cfg := GenerateHookConfig("S1", 7832)
	hooks := cfg["hooks"].(map[string]any)

	syncEvents := []string{"PreToolUse", "PermissionRequest"}
	for _, event := range syncEvents {
		entries := hooks[event].([]any)
		entry := entries[0].(map[string]any)
		hookList := entry["hooks"].([]any)
		hook := hookList[0].(map[string]any)

		if async, ok := hook["async"]; ok && async.(bool) {
			t.Errorf("%s should be sync but has async=true", event)
		}
	}
}

func TestAsyncHooksHaveAsyncFlag(t *testing.T) {
	cfg := GenerateHookConfig("S1", 7832)
	hooks := cfg["hooks"].(map[string]any)

	asyncEvents := []string{
		"PostToolUse", "PostToolUseFailure", "SessionStart",
		"SessionEnd", "Stop", "Notification", "SubagentStart", "SubagentStop",
	}
	for _, event := range asyncEvents {
		entries := hooks[event].([]any)
		entry := entries[0].(map[string]any)
		hookList := entry["hooks"].([]any)
		hook := hookList[0].(map[string]any)

		async, ok := hook["async"]
		if !ok || !async.(bool) {
			t.Errorf("%s should have async=true", event)
		}
	}
}

func TestPermissionRequestHasLongTimeout(t *testing.T) {
	cfg := GenerateHookConfig("S1", 7832)
	hooks := cfg["hooks"].(map[string]any)

	entries := hooks["PermissionRequest"].([]any)
	entry := entries[0].(map[string]any)
	hookList := entry["hooks"].([]any)
	hook := hookList[0].(map[string]any)

	timeout, ok := hook["timeout"].(int)
	if !ok {
		t.Fatal("expected timeout number")
	}
	if timeout < 600 {
		t.Fatalf("PermissionRequest timeout should be >= 600, got %d", timeout)
	}
}

func TestHooksHaveAuthHeader(t *testing.T) {
	cfg := GenerateHookConfig("S1", 7832)
	hooks := cfg["hooks"].(map[string]any)

	entries := hooks["PreToolUse"].([]any)
	entry := entries[0].(map[string]any)
	hookList := entry["hooks"].([]any)
	hook := hookList[0].(map[string]any)

	headers, ok := hook["headers"].(map[string]any)
	if !ok {
		t.Fatal("expected headers map")
	}
	auth, ok := headers["Authorization"].(string)
	if !ok || auth != "Bearer $TOWER_HOOK_TOKEN" {
		t.Fatalf("expected Bearer $TOWER_HOOK_TOKEN, got %q", auth)
	}

	envVars, ok := hook["allowedEnvVars"].([]any)
	if !ok || len(envVars) == 0 {
		t.Fatal("expected allowedEnvVars with TOWER_HOOK_TOKEN")
	}
	if envVars[0].(string) != "TOWER_HOOK_TOKEN" {
		t.Fatalf("expected TOWER_HOOK_TOKEN, got %q", envVars[0])
	}
}

func TestWriteHookConfig(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cfg := GenerateHookConfig("S1", 7832)
	path, err := WriteHookConfig(projectDir, cfg)
	if err != nil {
		t.Fatalf("write hook config: %v", err)
	}

	expected := filepath.Join(projectDir, ".claude", "settings.local.json")
	if path != expected {
		t.Fatalf("expected path %q, got %q", expected, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse written config: %v", err)
	}
	if _, ok := parsed["hooks"]; !ok {
		t.Fatal("written config missing hooks key")
	}
}
