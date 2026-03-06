# Tower

**Mission control for AI coding agents.**
*A cockpit for supervising multiple coding-agent sessions on one machine.*

Tower is a local, inspectable control plane for developers running multiple AI coding agents on one machine. It is built for the moment when agent productivity creates a new bottleneck: supervision tax.

Without Tower, 5 active sessions becomes window hunting, approval spam, stale context, and surprise collisions. With Tower, you get one cockpit to see what every session is doing, clear safe decisions fast, catch overlap early, and stay in flow while your terminal remains first-class.

## Why now

As developers move from using one agent to supervising many, the hard part stops being generation and starts being coordination.

Tower helps you:

- know which session needs you now
- clear low-risk approvals in one move
- catch repo, branch, and file collisions before they hurt
- jump into the right session with enough context to act
- park work and resume later without replaying the whole transcript

## What Tower does

Tower provides a unified cockpit for active AI agent sessions:

1. **Bird's-eye status** - See every session, its state, confidence, task, and time in state
2. **Action inbox** - Review pending approvals across sessions and batch low-risk ones
3. **Zoom in / jump out** - Inspect activity in the cockpit or jump straight to the terminal or IDE
4. **Conflict radar** - Catch likely collisions across repos, branches, files, and git operations
5. **Remote nudges** - Send commands to managed sessions from the cockpit
6. **Park and resume** - Save sessions as work bundles, not just raw transcripts
7. **Outcome summaries** - See what each session finished, what is blocked, and what still needs you

## Why trust it

Tower is designed to be useful without being slippery:

- **Local-first** - session state, decisions, and audit history stay on your machine
- **Inspectable** - actions and outcomes are reviewable, not hidden behind magic
- **Explicit about control** - Tower distinguishes observed sessions from managed sessions
- **Terminal-first** - your direct terminal or IDE workflow stays first-class

## Quick proof points

- supervise 5 agents without juggling 5 terminals
- clear clusters of low-risk approvals in one move
- catch same-file or same-repo collisions before merge pain
- resume yesterday's work in seconds instead of replaying the whole log

## What people should see first

If the first screenshot or GIF is good, the value should be obvious in under 10 seconds:

1. a cockpit with 6 active sessions and one clear "needs you now" queue
2. four matching read-only approvals selected and cleared together
3. a same-file conflict warning between two sessions
4. a parked session card with repo, goal, blockers, and `Resume`

## Core product model

Tower has two public session classes.

### Observed sessions

Tower discovers these from the side and provides read-mostly visibility:

- identity, liveness, repo or workspace, and best-effort state
- recent activity and summaries when available
- jump-to-terminal or jump-to-IDE
- low-confidence continuity when rediscovery is inferred rather than proven

Observed sessions are **not** guaranteed remotely controllable.

### Managed sessions

Managed sessions are launched under Tower control, for example:

```text
tower run claude
```

Managed sessions are the controllable tier:

- deterministic approvals
- remote command injection
- parking and resume
- durable session identity
- stronger audit and provenance guarantees

If Tower did not launch the session, Tower treats it as observe-only unless the tool exposes a stable external control API.

## Architecture direction

Tower is built around a few hard decisions:

- **Observed vs managed** instead of pretending every discovered session is equally controllable
- **Tower-owned PTY or ConPTY for managed sessions** instead of fragile OS input injection
- **Event-first core** where adapters emit normalized events and Tower materializes state
- **Policy-based approvals** with risk classes, staleness checks, and explicit batch rules
- **Richer state model** with substates and confidence, not just working, blocked, idle, or done

## Adapter model

Tower core is tool-agnostic. Adapters discover sessions, emit events, expose capabilities, and perform actions when supported.

```text
discover_sessions() -> session descriptors
subscribe_events() -> event stream
get_snapshot(session_id) -> current materialized state
list_capabilities(session_id) -> supported actions
perform(session_id, action) -> result
```

Examples of normalized events:

- `session.discovered`
- `session.updated`
- `state.changed`
- `approval.requested`
- `approval.resolved`
- `summary.updated`
- `error.reported`

## V1 focus

Tower v1 goes deep on one truthful managed experience first:

| Adapter | V1 mode | Notes |
|---------|---------|-------|
| `claude-code` | Managed + observed | Managed sessions launched via `tower run claude`; passive discovery also supported |
| `copilot-cli` | Observed-first | Stable hooks and control surfaces still need exploration |
| `vscode` | Observed-first | Native VS Code can be observed; remote and dev-container sessions are observe-only in v1 |
| `wsl` | Observed-first | Observe-only in v1 |

V1 is honest about its wedge: deep managed support for Claude Code, observed-first support elsewhere. It is optimized for trustworthy supervision, not broad but fragile control claims.

## Product roadmap

### V1: trustworthy cockpit

- managed Claude Code sessions on native Windows and native macOS
- observed sessions across other supported tools
- bird's-eye session list with state and confidence
- low-risk batch approvals
- conflict warnings
- jump-to-session flows
- park and resume basics
- durable local audit log

### Next: manager workflows

- attention-ranked inbox
- decisions memory and learned recommendations
- resource governor for heavy tasks
- richer park and resume bundles
- compare, review, and reassign flows between agents
- daily portfolio summary of what the swarm got done

## Future adapters

Any CLI or IDE agent tool can be supported by implementing the adapter contract. Examples: Aider, Cursor, Kiro, Windsurf, Cline.

## Repository layout

- `docs\brainstorm-product\` - how we got here: session transcripts, research, model reviews, and prompt artifacts
- `docs\requirements\` - execution-facing product requirements, v1 scope, and roadmap
- `docs\engineering\` - durable technical design and implementation-facing decisions
- `cmd\` - Go entrypoints (`tower`, `tower-demo`)
- `internal\` - Tower runtime, adapters, UI, storage, and core packages
- `deployment\` - release, dev, and hosting configuration
- `test\` - fixtures, integration coverage, and end-to-end scenarios
- `web\` - landing page and public marketing site
- `scripts\` - repository helper scripts

## Key docs

- `docs\engineering\architecture-decisions.md` - canonical architecture decisions
- `docs\engineering\foundation-spec.md` - concrete technical foundation for the first implementation
- `docs\requirements\v1-scope.md` - execution-facing v1 scope and support matrix
- `docs\requirements\roadmap.md` - phased V1, V1.5, and V2 execution roadmap
- `docs\brainstorm-product\session-context.md` - full design session and decisions
- `docs\brainstorm-product\research.md` - prior art and integration notes
- `docs\brainstorm-product\review-gpt.md` - detailed architecture review
- `docs\brainstorm-product\review-gemini.md` - second-model review
- `docs\brainstorm-product\gpt-questions.md` - the architecture questions used to pressure-test the design
- `docs\brainstorm-product\claude-feedback-on-architecture-decisions.md` - follow-up feedback and cleanup notes

## Status

Pre-development. Architecture, product framing, and scope definition are underway.

## License

MIT
