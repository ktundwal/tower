# Tower Roadmap

This file captures the phased execution roadmap for Tower.

## Guiding sequence

1. lock the foundations
2. design the Claude managed adapter
3. ship one truthful end-to-end v1 slice
4. broaden observed visibility without weakening the product line
5. launch publicly once there is a real workflow to show
6. add manager features after real usage validates the next priorities

## V1: trustworthy supervision

Goal: make 3 to 10 concurrent agent sessions feel manageable, safe, and worth using every day.

### Workstreams

- foundation specs and repository bootstrap
- Claude managed adapter design
- managed Claude runtime
- core event and state engine
- cockpit UX
- conflict detection, park/resume, and summaries
- observed adapters for Copilot CLI, VS Code, and WSL
- launch assets and alpha hardening

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
