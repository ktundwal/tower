package daemon

import (
	"strings"

	"tower/internal/contracts"
)

// ClassifyRisk determines the risk class of a tool call from its name and input.
// Classification follows the rules in design doc section 6.3.
func ClassifyRisk(toolName string, toolInput map[string]any) contracts.RiskClass {
	switch toolName {
	case "Read", "Glob", "Grep":
		return contracts.RiskClassReadOnly
	case "Edit", "Write":
		return contracts.RiskClassWorkspaceWrite
	case "WebFetch", "WebSearch":
		return contracts.RiskClassNetworkRead
	case "Bash":
		return classifyBash(toolInput)
	default:
		return contracts.RiskClassUnknown
	}
}

func classifyBash(toolInput map[string]any) contracts.RiskClass {
	cmd, ok := toolInput["command"].(string)
	if !ok || cmd == "" {
		return contracts.RiskClassUnknown
	}

	// For chained commands (&&, ;), classify each segment and return the highest risk.
	if strings.Contains(cmd, "&&") || strings.Contains(cmd, ";") {
		return classifyChainedBash(cmd)
	}

	// For piped commands, classify the first command (the one that determines intent).
	// But also check all segments for mutations.
	if strings.Contains(cmd, "|") {
		return classifyPipedBash(cmd)
	}

	return classifySingleBash(cmd)
}

func classifySingleBash(cmd string) contracts.RiskClass {
	first := firstWord(strings.TrimSpace(cmd))

	// Git commands need sub-command analysis.
	if first == "git" {
		return classifyGit(cmd)
	}

	// Read-only commands.
	readOnly := []string{"ls", "which", "cat", "head", "tail", "wc", "find", "echo", "pwd", "env", "printenv", "whoami", "hostname", "uname", "date", "df", "du", "file", "stat", "type", "test"}
	for _, ro := range readOnly {
		if first == ro {
			return contracts.RiskClassReadOnly
		}
	}

	// Package install.
	if isPackageInstall(first, cmd) {
		return contracts.RiskClassPackageInstall
	}

	// Network commands.
	if first == "curl" || first == "wget" {
		return classifyNetwork(cmd)
	}

	// Workspace write commands.
	writeCommands := []string{"rm", "mv", "cp", "mkdir", "chmod", "chown", "touch", "sed", "awk", "tee", "truncate", "dd"}
	for _, w := range writeCommands {
		if first == w {
			return contracts.RiskClassWorkspaceWrite
		}
	}

	return contracts.RiskClassUnknown
}

func classifyGit(cmd string) contracts.RiskClass {
	sub := gitSubcommand(cmd)

	readOnlyGit := []string{"status", "log", "diff", "show", "branch", "tag", "remote", "describe", "rev-parse", "ls-files", "ls-tree", "shortlog", "stash list", "blame", "reflog"}
	for _, ro := range readOnlyGit {
		if sub == ro {
			return contracts.RiskClassReadOnly
		}
	}

	// Everything else is a mutation.
	return contracts.RiskClassGitMutation
}

func classifyNetwork(cmd string) contracts.RiskClass {
	lower := strings.ToLower(cmd)
	// curl with -X POST/PUT/PATCH/DELETE or -d/--data is a write.
	writeIndicators := []string{"-x post", "-x put", "-x patch", "-x delete", " -d ", " --data", " --data-", " -f ", " --form"}
	for _, indicator := range writeIndicators {
		if strings.Contains(lower, indicator) {
			return contracts.RiskClassNetworkWrite
		}
	}
	return contracts.RiskClassNetworkRead
}

func isPackageInstall(first, cmd string) bool {
	switch first {
	case "npm":
		sub := nthWord(cmd, 1)
		return sub == "install" || sub == "i" || sub == "ci"
	case "yarn":
		sub := nthWord(cmd, 1)
		return sub == "add" || sub == "install"
	case "pnpm":
		return nthWord(cmd, 1) == "add" || nthWord(cmd, 1) == "install"
	case "pip", "pip3":
		return nthWord(cmd, 1) == "install"
	case "go":
		return nthWord(cmd, 1) == "install"
	}
	return false
}

// classifyChainedBash splits on && and ; then returns the highest-risk class.
func classifyChainedBash(cmd string) contracts.RiskClass {
	// Normalize separators.
	normalized := strings.ReplaceAll(cmd, "&&", "\x00")
	normalized = strings.ReplaceAll(normalized, ";", "\x00")
	segments := strings.Split(normalized, "\x00")

	highest := contracts.RiskClassReadOnly
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		risk := classifySingleBash(seg)
		if riskLevel(risk) > riskLevel(highest) {
			highest = risk
		}
	}
	return highest
}

// classifyPipedBash classifies piped commands. The first segment determines
// the primary intent, but any mutating segment escalates.
func classifyPipedBash(cmd string) contracts.RiskClass {
	segments := strings.Split(cmd, "|")
	highest := contracts.RiskClassReadOnly
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		risk := classifySingleBash(seg)
		if riskLevel(risk) > riskLevel(highest) {
			highest = risk
		}
	}
	return highest
}

// riskLevel returns a numeric severity for ordering.
func riskLevel(rc contracts.RiskClass) int {
	switch rc {
	case contracts.RiskClassReadOnly:
		return 0
	case contracts.RiskClassGitRead:
		return 1
	case contracts.RiskClassNetworkRead:
		return 2
	case contracts.RiskClassWorkspaceWrite:
		return 3
	case contracts.RiskClassPackageInstall:
		return 4
	case contracts.RiskClassGitMutation:
		return 5
	case contracts.RiskClassNetworkWrite:
		return 6
	case contracts.RiskClassProcessExec:
		return 7
	case contracts.RiskClassSecretAccess:
		return 8
	case contracts.RiskClassUnknown:
		return 9
	default:
		return 9
	}
}

func firstWord(s string) string {
	return nthWord(s, 0)
}

func nthWord(s string, n int) string {
	fields := strings.Fields(s)
	if n >= len(fields) {
		return ""
	}
	return fields[n]
}

func gitSubcommand(cmd string) string {
	fields := strings.Fields(cmd)
	for i, f := range fields {
		if f == "git" && i+1 < len(fields) {
			// Skip flags between git and subcommand (e.g., git -C /path status).
			for j := i + 1; j < len(fields); j++ {
				if !strings.HasPrefix(fields[j], "-") {
					return fields[j]
				}
				// Some flags consume the next arg (e.g., -C <path>).
				if fields[j] == "-C" || fields[j] == "-c" || fields[j] == "--git-dir" || fields[j] == "--work-tree" {
					j++ // skip the value
				}
			}
		}
	}
	return ""
}
