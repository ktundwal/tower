package daemon

import (
	"testing"

	"tower/internal/contracts"
)

func TestClassifyReadOnlyTools(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		input    map[string]any
		expected contracts.RiskClass
	}{
		{"Read any", "Read", map[string]any{"file_path": "/tmp/foo"}, contracts.RiskClassReadOnly},
		{"Glob any", "Glob", map[string]any{"pattern": "**/*.go"}, contracts.RiskClassReadOnly},
		{"Grep any", "Grep", map[string]any{"pattern": "TODO"}, contracts.RiskClassReadOnly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRisk(tt.tool, tt.input)
			if got != tt.expected {
				t.Fatalf("ClassifyRisk(%q) = %q, want %q", tt.tool, got, tt.expected)
			}
		})
	}
}

func TestClassifyBashReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"git status", "git status"},
		{"git log", "git log --oneline"},
		{"git diff", "git diff HEAD"},
		{"ls", "ls -la"},
		{"ls path", "ls /tmp/foo"},
		{"which", "which go"},
		{"git status with path", "git status ."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRisk("Bash", map[string]any{"command": tt.command})
			if got != contracts.RiskClassReadOnly {
				t.Fatalf("ClassifyRisk(Bash %q) = %q, want read_only", tt.command, got)
			}
		})
	}
}

func TestClassifyBashGitMutation(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"git add", "git add ."},
		{"git commit", "git commit -m 'fix'"},
		{"git push", "git push origin main"},
		{"git push force", "git push --force"},
		{"git reset", "git reset --hard HEAD~1"},
		{"git checkout branch", "git checkout -b feature"},
		{"git merge", "git merge main"},
		{"git rebase", "git rebase main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRisk("Bash", map[string]any{"command": tt.command})
			if got != contracts.RiskClassGitMutation {
				t.Fatalf("ClassifyRisk(Bash %q) = %q, want git_mutation", tt.command, got)
			}
		})
	}
}

func TestClassifyBashPackageInstall(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"npm install", "npm install express"},
		{"npm i", "npm i -D typescript"},
		{"pip install", "pip install requests"},
		{"pip3 install", "pip3 install flask"},
		{"yarn add", "yarn add react"},
		{"pnpm add", "pnpm add vite"},
		{"go install", "go install golang.org/x/tools/...@latest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRisk("Bash", map[string]any{"command": tt.command})
			if got != contracts.RiskClassPackageInstall {
				t.Fatalf("ClassifyRisk(Bash %q) = %q, want package_install", tt.command, got)
			}
		})
	}
}

func TestClassifyWorkspaceWrite(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input map[string]any
	}{
		{"Edit", "Edit", map[string]any{"file_path": "/tmp/foo", "old_string": "a", "new_string": "b"}},
		{"Write", "Write", map[string]any{"file_path": "/tmp/foo", "content": "hello"}},
		{"Bash rm", "Bash", map[string]any{"command": "rm -rf node_modules"}},
		{"Bash mv", "Bash", map[string]any{"command": "mv foo.go bar.go"}},
		{"Bash mkdir", "Bash", map[string]any{"command": "mkdir -p /tmp/new"}},
		{"Bash cp", "Bash", map[string]any{"command": "cp file1 file2"}},
		{"Bash chmod", "Bash", map[string]any{"command": "chmod +x script.sh"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRisk(tt.tool, tt.input)
			if got != contracts.RiskClassWorkspaceWrite {
				t.Fatalf("ClassifyRisk(%q) = %q, want workspace_write", tt.name, got)
			}
		})
	}
}

func TestClassifyNetworkRead(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input map[string]any
	}{
		{"WebFetch", "WebFetch", map[string]any{"url": "https://example.com"}},
		{"WebSearch", "WebSearch", map[string]any{"query": "golang"}},
		{"Bash curl GET", "Bash", map[string]any{"command": "curl https://api.example.com"}},
		{"Bash wget", "Bash", map[string]any{"command": "wget https://example.com/file.tar.gz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRisk(tt.tool, tt.input)
			if got != contracts.RiskClassNetworkRead {
				t.Fatalf("ClassifyRisk(%q) = %q, want network_read", tt.name, got)
			}
		})
	}
}

func TestClassifyBashNetworkWrite(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"curl POST", "curl -X POST https://api.example.com/data"},
		{"curl -d", "curl -d '{\"key\":\"val\"}' https://api.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRisk("Bash", map[string]any{"command": tt.command})
			if got != contracts.RiskClassNetworkWrite {
				t.Fatalf("ClassifyRisk(Bash %q) = %q, want network_write", tt.command, got)
			}
		})
	}
}

func TestClassifyUnknownTool(t *testing.T) {
	got := ClassifyRisk("SomeNewTool", map[string]any{"foo": "bar"})
	if got != contracts.RiskClassUnknown {
		t.Fatalf("expected unknown, got %q", got)
	}
}

func TestClassifyBashUnknownCommand(t *testing.T) {
	got := ClassifyRisk("Bash", map[string]any{"command": "some-custom-script --flag"})
	if got != contracts.RiskClassUnknown {
		t.Fatalf("expected unknown for unrecognized bash command, got %q", got)
	}
}

func TestClassifyBashNoCommand(t *testing.T) {
	got := ClassifyRisk("Bash", map[string]any{})
	if got != contracts.RiskClassUnknown {
		t.Fatalf("expected unknown when no command, got %q", got)
	}
}

func TestClassifyBashPipedCommands(t *testing.T) {
	// Piped read-only commands are still read-only.
	got := ClassifyRisk("Bash", map[string]any{"command": "git log | head -5"})
	if got != contracts.RiskClassReadOnly {
		t.Fatalf("expected read_only for piped git log, got %q", got)
	}

	// Pipe into a write command is workspace_write.
	got = ClassifyRisk("Bash", map[string]any{"command": "echo hello | tee file.txt"})
	if got != contracts.RiskClassUnknown && got != contracts.RiskClassWorkspaceWrite {
		t.Fatalf("expected unknown or workspace_write for tee, got %q", got)
	}
}

func TestClassifyBashChainedCommands(t *testing.T) {
	// && with a mutation escalates.
	got := ClassifyRisk("Bash", map[string]any{"command": "git add . && git commit -m 'fix'"})
	if got != contracts.RiskClassGitMutation {
		t.Fatalf("expected git_mutation for chained git commands, got %q", got)
	}
}
