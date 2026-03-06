# Tower - Full Session Context for Cross-Model Review

## Session Goal
Design a supervisor cockpit ("tower") for managing multiple concurrent AI coding agents on a single dev box.

## How We Got Here

### Starting Point
User (Kapil, engineering manager at Microsoft) runs 5-10+ concurrent AI coding agent sessions on a single dev box. These include Claude Code CLI, GitHub Copilot CLI, VS Code Copilot, and Agency CLI (Microsoft internal wrapper). He loses track of what each is doing, which are blocked on him, and what they accomplished.

### Problem Refinement (Q&A)

**Q: Same machine or multiple?**
A: Same box first. Even cloud agents (VS Code remote) would still be controlled via the dev box.

**Q: How many concurrent sessions?**
A: Process scan revealed 7 Claude Code CLI sessions, 14 VS Code processes, 1 Copilot desktop app, plus ~3 supporting processes per Claude session (agency node, workiq). About 70 agent-related processes total.

**Q: How do you find out when blocked?**
A: Toast notifications with sound exist but are too aggressive. They overlay current work and break flow. Notification just says "claude needs attention" with no context about which session or what it needs. Then must scan windows, scroll to rebuild context.

**Q: What would good enough look like?**
A: A cockpit/dashboard (option A). Bird's-eye view with all controls, all status, ability to zoom in/out. Not smarter notifications.

**Q: Preferred surface?**
A: Optimize for efficiency, not fastest to build. Key additional requirements emerged:
- **Batch approve**: If 4 of 10 sessions want to run grep, approve all at once. If one wants to commit, zoom in.
- **Summary view**: Idle/done sessions show what they accomplished.
- **Remote commands**: Send "run session learnings skill and exit" from cockpit.
- **Session memory**: Park a session, resume next day.

**Q: How do you interact with approvals today?**
A: Press y/n in the terminal. Claude Code doesn't expose an external API.

**Q: Wrapper or side-channel?**
A: Side-channel proxy for approvals only. Not a wrapper. When doing focus work, interact directly with the terminal. The cockpit only intercepts approval requests and can inject responses.

**Q: Scope of tools?**
A: Must be extensible for future agentic CLI tools (Kiro, Aider, Cursor, etc.). Ship v1 with Claude Code CLI, GitHub Copilot CLI, VS Code. Don't take dependency on Agency (Microsoft internal), but learn from it.

### Research Conducted

**Entire.io** (github.com/entireio/cli, Go, MIT, ~3.4k stars):
- Session capture and replay, not live supervision
- Hooks into git to record agent sessions as checkpoints on separate branch
- Sessions as first-class objects with identity, checkpoints, transcripts
- Uses Claude Code's hook system (settings.json) as integration surface
- Rewind/resume for sessions
- Does NOT have: live dashboard, approval aggregation, batch approve, remote commands, blocked detection

**Agency CLI** (Microsoft internal, aka.ms/agency):
- Wrapper that launches Claude Code and Copilot CLI underneath
- Adds agent loading, MCP orchestration, structured session logs
- Session logs at ~/.agency/logs/session_{timestamp}_{pid}/
- Confirms both Claude Code and Copilot CLI are stdin/stdout terminal processes
- Tower should work alongside Agency, not depend on it

### Architecture Decisions Made

1. **Side-channel architecture**: Tower monitors from the side, doesn't wrap terminal I/O
2. **Adapter interface**: Each tool supported via adapter implementing standard interface (discover_sessions, get_state, get_pending_actions, respond_to_action, send_command, etc.)
3. **v1 adapters**: Claude Code CLI, GitHub Copilot CLI, VS Code
4. **Future adapters**: Community-contributed for Aider, Cursor, Kiro, Windsurf, Cline, etc.

### Open Questions
- Language choice: Python (textual TUI) vs Go (bubbletea) vs TypeScript (ink)
- IPC mechanism: Unix domain sockets, named pipes (Windows), or file-based
- Event architecture: push (adapters emit events) vs pull (tower polls)
- Approval proxy: how to intercept stdin without wrapping the terminal
- Session identity: user-named at launch vs auto-generated from first prompt

## Adapter Interface (proposed)

```
Adapter Interface:
  - discover_sessions() -> list of active sessions
  - get_session_state(id) -> working | blocked | idle | done
  - get_pending_actions(id) -> list of approval requests
  - get_activity_log(id) -> recent activity summary
  - respond_to_action(id, action_id, response) -> approve/deny/chat
  - send_command(id, command) -> inject a command into the session
  - get_session_summary(id) -> what the session accomplished
```

## What We Need Feedback On

1. **Problem definition**: Is it complete? What are we missing?
2. **Architecture approach**: Side-channel + adapter pattern. Risks? Alternatives?
3. **Approval proxy**: This is the hardest unsolved problem. How to intercept stdin for approvals from a side-channel without wrapping the terminal?
4. **Extensibility**: Is the adapter interface right? Too broad? Too narrow?
5. **Prior art we missed**: Other tools or projects solving similar problems?
6. **Platform concerns**: Windows-first (dev box is Windows/MINGW64). Cross-platform considerations?
7. **What would YOU add to the problem space?** Blind spots in our thinking?
