# Tower Roadmap

This file captures the phased execution roadmap for Tower.

This is the repo-visible execution plan. The session `plan.md` is a working scratchpad for Copilot; if the two ever drift, this file should be kept in sync and treated as the repository source of truth.

## Guiding sequence

1. lock the foundations
2. design the Claude managed adapter
3. ship one truthful end-to-end v1 slice
4. broaden observed visibility without weakening the product line
5. launch publicly once there is a real workflow to show
6. add manager features after real usage validates the next priorities

## V1: trustworthy supervision

Goal: make 3 to 10 concurrent agent sessions feel manageable, safe, and worth using every day.

### Current status

- `foundation-spec` is complete.
- `claude-managed-adapter-design` is complete.
- `repo-bootstrap` is complete enough for the first implementation pass: contracts, schema stub, bootstrap CLI entrypoints, and demo fixture exist.
- The current weakness is not architecture clarity; it is the lack of a proved managed runtime path.

### Workstreams

- foundation specs and repository bootstrap
- Claude managed adapter design
- managed Claude runtime
- core event and state engine
- cockpit UX
- conflict detection, park/resume, and summaries
- observed adapters for Copilot CLI, VS Code, and WSL
- launch assets and alpha hardening

### Immediate next slice

The next execution slice is runtime-first, not scaffold-first. Hooks are the primary integration surface; PTY is for terminal bridging.

1. **Tower daemon HTTP endpoint + hook config injection**
   - minimal Go HTTP server that receives hook events and logs them
   - hook config template that `tower run` injects into Claude Code sessions
   - run a real Claude Code session with hooks installed, capture real event payloads
   - validate: can a hook return value approve/deny a permission request?
2. **Managed Claude PTY/ConPTY spike**
   - start with Windows first
   - launch Claude through a Tower-owned helper with hooks auto-injected
   - tee output to the terminal plus a capture/log path
   - prove resize, interruption, and child cleanup behavior
3. **One truthful remote action path**
   - hook receives approval request, POSTs to daemon, daemon decides, hook returns decision
   - single managed-session approval path, no batch yet
   - freshness/staleness safety before convenience
4. **Fixture-driven Bubble Tea cockpit**
   - wire the six-session fixture into a real TUI
   - validate operator workflow and make the product visually real in parallel
5. **SQLite after runtime proof**
   - make persistence blocking before restart/reconnect, audit durability, or park/resume claims

### Execution constraints

- Do not add more empty placeholder directories.
- Do not deepen the engine/store abstraction until the hook pipeline is real.
- Hooks are the primary approval channel. PTY output parsing is a fallback, not the main path.
- Keep the launch wedge honest: one managed Claude path that actually works beats wider but softer scaffolding.

### Demo milestones

- launch `tower run claude` and see the session appear in the cockpit
- approve one managed action remotely with audit history
- run a six-session mixed demo and batch-clear read-only approvals
- surface one same-file conflict and resolve it from the cockpit
- park a managed session, restart Tower, and resume with context intact

## V1.5: manager acceleration

Goal: move from basic supervision to efficient swarm management.

### Candidate additions

- attention-ranked inbox
- durable decisions memory
- learned recommendations and suggested policies
- resource governor for heavy work
- richer park/resume bundles
- daily swarm outcomes view

## V2: agent-team operating layer

Goal: make Tower feel like the operating layer for a personal team of coding agents.

### Candidate additions

- compare competing agent outputs side by side
- ask one agent to review or summarize another
- reassign and rebalance work between sessions
- reusable supervision playbooks and policies
- primary-session model per repo
- optional shared or team-facing views
