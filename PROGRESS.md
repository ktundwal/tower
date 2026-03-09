# Tower Progress

## Current Slice

Slice 2: Risk classification, batch auto-approve, process spawning

## Status

Slices 1–2 complete. Hook integration live-validated against real Claude Code. 73 unit/e2e tests + 4 fixture regression tests + 1 smoke test (Haiku), vet clean.

## What's Done

- Architecture decisions locked (`docs/engineering/`)
- Contract types and adapter interface (`internal/contracts/`)
- Bootstrap CLI with `tower run claude` and `tower-demo` entrypoints
- Demo fixture harness (`test/fixtures/demo/six-session-mixed.json`)
- Event envelope schema (`schemas/event-envelope-v1.schema.json`)
- Memory-backed repository (SQLite deferred)
- Project CLAUDE.md, skills (`/tower-arch`, `/tower-scope`, `/review`), settings
- Session commands (`/start`, `/wrap`) — note: may need CLI restart to register
- Go 1.26.1 installed
- 5-slice implementation plan reviewed and approved
- Fixed `.claude/settings.json` — replaced invalid `PreCommit` hook with `PreToolUse` + `Bash(git commit*)` matcher
- **Daemon HTTP server** (`internal/daemon/server.go`) — health endpoint, auth middleware, hook routing for all 10 event types, sync/async dispatch, body parsing, event recording
- **Daemon lifecycle** (`internal/daemon/daemon.go`) — Start/Stop with real listener on ephemeral port, random token generation, graceful shutdown
- **Lockfile** (`internal/daemon/lockfile.go`) — write/read/remove daemon.lock with port + token + PID
- **Hook config injection** (`internal/adapters/claude/hooks.go`) — GenerateHookConfig for all 10 hook events with correct URLs, timeouts, auth headers, sync/async flags; WriteHookConfig to `.claude/settings.local.json`
- **Bootstrap wiring** (`internal/app/bootstrap.go`) — `tower run claude` now starts daemon, creates session, registers with daemon, writes hook config
- **E2e tests** (`test/e2e/`) — full launch flow and daemon hook round-trip over real HTTP
- **Risk classification** (`internal/daemon/classify.go`) — deterministic risk classification per design doc section 6.3: Read/Glob/Grep → read_only, Bash git/ls/rm/curl/npm analysis, pipe/chain handling, Edit/Write → workspace_write, WebFetch/WebSearch → network_read
- **Batch auto-approve** — PreToolUse handler auto-approves read_only ops with `permissionDecision: "allow"`, non-read-only passes through to PermissionRequest
- **Process spawner** (`internal/runtime/process.go`) — cross-platform subprocess spawning via os/exec, env injection, working dir, PID/startedAt tracking, Tower env vars (TOWER_MANAGED, TOWER_SESSION_ID, etc.)
- **ManagedManager** (`internal/runtime/managed.go`) — orchestrates daemon registration + hook config injection + session descriptor construction; replaces BootstrapManager for managed sessions
- **Bootstrap upgraded** — `tower run claude` starts daemon → upgrades engine to ManagedManager → session creation handles registration + hooks automatically
- **Live hook fixtures** (`test/fixtures/hooks/live-capture-2026-03-09.json`) — 7 real Claude hook payloads captured from live session, used as ground truth
- **Fixture regression tests** (`internal/daemon/regression_test.go`) — replay real payloads through parse → classify → server response pipeline; catches struct drift, classification changes, response format changes; runs in normal `go test ./...`
- **Automated smoke test** (`test/smoke/smoke_test.go`) — spawns real Claude (Haiku) against live daemon, validates full hook round-trip; build-tagged `smoke`, run with `go test -tags smoke`
- **Smoke test harness** (`cmd/tower-smoke/`) — manual debugging tool for watching hook events in real time
- **HookEvent struct enriched** — captures all fields Claude actually sends: permission_mode, tool_use_id, tool_response, permission_suggestions, stop_hook_active, notification_type, etc.

## What's Next

1. Cockpit UI (Slice 3) — Bubble Tea TUI showing sessions + pending approvals
2. Conflict detection (Slice 4) — same repo/branch/file overlap across sessions
3. SQLite persistence (Slice 5) — replace MemoryRepository
4. ConPTY/PTY terminal bridging — layer real pseudoterminal on top of process spawner for resize/ANSI

## Blockers

None.

## Decisions

- Go module version in go.mod says 1.22 but Go 1.26.1 is installed. May need to update go.mod.
- Skills vs commands: skills are agent-loaded context, commands are user-invoked slash commands.
- Daemon uses ephemeral port (OS-assigned) — no hardcoded port. Lockfile records the actual port.
- PermissionRequest stub auto-allows everything. Real policy + cockpit approval is Slice 3.
- Hook config uses `$TOWER_HOOK_TOKEN` env var reference per design doc — token injected at spawn time.
- Process spawner uses os/exec with pipes (not full PTY). ConPTY/PTY layered in later when resize/ANSI needed.
- ManagedManager replaces BootstrapManager at daemon start time via Engine.SetRuntime().
- Claude HTTP hooks confirmed working: `type: "http"` in settings.local.json, sync blocking, env var expansion via `allowedEnvVars`.
- Smoke tests use `--model haiku` to minimize cost.
- Must strip `CLAUDECODE` env var when spawning Claude as subprocess.
