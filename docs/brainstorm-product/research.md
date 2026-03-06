# Tower Research Notes

## Process Landscape (observed on dev box, 2026-03-06)

Snapshot of a typical working session:

- **7 Claude Code CLI sessions** running across terminal tabs (cons0-cons7)
- Each Claude session spawns ~3 supporting processes (agency node, workiq)
- **14 VS Code processes** (window hosts + extensions)
- **1 Copilot desktop app**
- Total: ~70 node/agent-related processes

Claude Code CLI is discoverable via `ps` as `/c/Users/{user}/bin/cc.cmd` or `claude.exe`.
Agency wraps Claude Code (`agency claude`) and Copilot (`agency copilot`) but is not required.

## Entire.io Analysis

Source: `github.com/entireio/cli` (Go, MIT, ~3.4k stars)

### What they built
- Git hook integration that captures AI agent sessions as checkpoints on a separate branch (`entire/checkpoints/v1`)
- Sessions: `YYYY-MM-DD-<UUID>` format, contain prompts, responses, file diffs
- Checkpoints: snapshots within sessions (12-char hex), created on commits
- Commands: `entire status`, `entire rewind`, `entire resume`, `entire explain`
- Multi-agent: hooks for Claude Code, Gemini, Cursor, OpenCode via their settings files
- Auto-generate AI summaries at commit time (intent, outcomes, learnings, friction)

### What's relevant to tower
- **Sessions as first-class objects**: tower needs this, with live state tracking
- **Hook-based integration**: Entire proves Claude Code hooks (`settings.json`) work as an integration surface
- **Rewind/resume**: maps to tower's "park and resume" feature
- **`entire explain`**: post-hoc code-to-session tracing, different from tower's real-time focus

### What Entire does NOT cover
- No live dashboard or cockpit
- No pending-approval aggregation or batch approve
- No remote command injection
- No "blocked on me" detection
- No cross-session batch operations

## Agency CLI Analysis (Microsoft internal, not a dependency)

Source: internal Microsoft tool (`aka.ms/agency`)

### What it does
- Wrapper that launches Claude Code (`agency claude`) and Copilot CLI (`agency copilot`)
- Adds agent loading from personal/repo/org/company sources
- MCP server orchestration (20+ MCP servers: ADO, Kusto, ICM, Teams, Mail, etc.)
- Structured session logs at `~/.agency/logs/session_{timestamp}_{pid}/`
- Eval/batch mode for running agents programmatically

### Architectural insight
- Agency sits *above* Claude Code and Copilot CLI, tower sits *beside* them
- Agency's session logs could be an additional data source but tower should not depend on agency
- Both Claude Code and Copilot CLI are stdin/stdout terminal processes underneath, confirming the proxy approach for approvals

## Claude Code Hook System

Claude Code supports hooks in `.claude/settings.json` that fire on events:
- Tool calls (before/after bash, file edits, etc.)
- This is the primary integration surface for the read path (monitoring)
- Hooks can emit events that tower's adapter listens for

For the write path (approvals), Claude Code reads from stdin:
- `y` = approve, `n` = deny, or type a message to chat
- A proxy layer would need to intercept stdin when tower is handling approvals
- When user is focused in the terminal, proxy is transparent (passthrough)

## Notification Pain Point

Current state: toast notifications with sound when Claude needs attention.
Problems:
- Notification says "claude needs attention" with no context about which session or what it needs
- Overlays current work, breaking flow
- After notification: must scan windows, find the right one, scroll to rebuild context
- Identical low-risk approvals (grep, read) pile up across sessions

Desired state: peripheral awareness (glanceable cockpit), not interruptions.

## Key Design Tensions

1. **Side-channel vs. wrapper**: tower must not degrade the direct terminal experience
2. **Push vs. pull**: adapters push events to tower (event bus) vs. tower polls adapter state
3. **Cross-platform**: Windows (named pipes, ConPTY) vs. Unix (domain sockets, PTY), dev box is Windows
4. **Approval proxy**: how to intercept stdin for approval without wrapping the terminal session
5. **Session identity**: who names sessions? User at launch time? Auto-generated from first prompt? Both?
