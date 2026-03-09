# /tower-scope — V1 Scope Boundaries

Load this before starting any new slice. If work isn't listed in "In Scope", don't do it.

## V1 User Promise

Tower v1 lets a developer supervise multiple coding-agent sessions from one cockpit without losing trust, context, or flow.

## In Scope

- Managed Claude Code sessions on native Windows and native macOS
- Observed visibility for Copilot CLI, VS Code, and WSL sessions
- Cockpit showing session state, confidence, task, and time in state
- Needs-you-now queue
- Read-only batch approvals for managed sessions only
- Jump-to-terminal and jump-to-IDE flows
- Cross-session conflict warnings (same repo, branch, file, competing git activity)
- Park/resume basics for managed sessions
- Session summaries and durable local audit history

## Support Matrix

| Surface | V1 Mode |
|---------|---------|
| Claude Code (native Windows) | Managed + observed |
| Claude Code (native macOS) | Managed + observed |
| Copilot CLI | Observed-first |
| VS Code native | Observed-first |
| VS Code remote / dev-container | Observe-only |
| WSL | Observe-only |

## Explicit Non-Goals (Do NOT Build)

- Universal managed support across all tools
- Control of arbitrary already-running sessions Tower didn't launch
- Cloud sync or cross-machine orchestration
- Team-shared multi-user control
- Aggressive automation beyond current policy model
- Operator text injection (sending messages to Claude from cockpit)
- PTY output parsing for approval detection
- Lease/pause/resume interception model
- Detach/reattach (if tower run exits, session dies)
- Daemon restart recovery with PTY reconnection
- External adapter SDK
- Infinite transcript retention
- Magic side-channel input injection into unmanaged consoles

## Execution Slices (Current Plan)

1. Tower daemon HTTP endpoint + hook config injection
2. Managed Claude PTY/ConPTY spike (Windows first)
3. One truthful remote action path (hook → daemon → hook)
4. Fixture-driven Bubble Tea cockpit
5. SQLite after runtime proof

## Execution Constraints

- Do not add empty placeholder directories
- Do not deepen engine/store abstraction until hook pipeline is real
- Hooks are the primary approval channel; PTY output parsing is a fallback, not the main path
- One managed Claude path that actually works beats wider but softer scaffolding

## If You're Unsure

If the work isn't clearly in "In Scope" above, stop and ask. Don't build it speculatively.

Full scope doc: `docs/requirements/v1-scope.md`
Full roadmap: `docs/requirements/roadmap.md`
