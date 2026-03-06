# Tower V1 Scope

This file captures the execution-facing v1 scope for Tower.

## Product line

- `observed` sessions are visibility-only or read-mostly
- `managed` sessions are the controllable tier
- Tower only promises deterministic approvals, command injection, parking, and resume for managed sessions

## V1 user promise

Tower v1 should let a developer supervise multiple coding-agent sessions from one cockpit without losing trust, context, or flow.

## In scope

- managed Claude Code sessions on native Windows and native macOS
- observed visibility for Copilot CLI, VS Code, and WSL sessions
- a cockpit showing session state, confidence, task, and time in state
- a needs-you-now queue
- read-only batch approvals for managed sessions only
- jump-to-terminal and jump-to-IDE flows
- cross-session conflict warnings for same repo, branch, file, and competing git activity
- park/resume basics for managed sessions
- session summaries and durable local audit history

## Explicit non-goals

- universal managed support across all tools
- control of arbitrary already-running sessions Tower did not launch
- cloud sync or cross-machine orchestration
- team-shared multi-user control
- aggressive automation beyond the current policy model

## Support matrix

| Surface | V1 mode |
|---------|---------|
| Claude Code (native Windows) | Managed + observed |
| Claude Code (native macOS) | Managed + observed |
| Copilot CLI | Observed-first |
| VS Code native | Observed-first |
| VS Code remote / dev-container | Observe-only |
| WSL | Observe-only |

## Success criteria

- the observed-versus-managed line is obvious in the product
- a user can supervise 5 to 10 concurrent sessions without feeling lost
- low-risk approvals are significantly faster than terminal-by-terminal handling
- Tower degrades to unknown/low confidence after gaps instead of guessing
- early users report lower supervision tax in real work
