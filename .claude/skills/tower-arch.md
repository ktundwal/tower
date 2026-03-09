# /tower-arch — Architecture Briefing

Load this before any design or implementation work. This is a curated summary of locked decisions. For full detail, read the referenced docs.

## Topology

```
user terminal
    |
    v
tower run claude (foreground bridge, owns PTY)
    | attach + stdin/stdout + resize
    v
Tower daemon <--- HTTP hooks (PreToolUse, PermissionRequest, etc.) --- Claude Code child
    |                                                                        ^
    +---- spawn + PTY/ConPTY ownership -------------------------------------|
```

Approvals flow through **HTTP hooks**, not PTY stdin injection. Claude posts `PermissionRequest` to Tower's HTTP API and blocks. Tower responds with allow/deny JSON.

## Two Session Classes

- **Managed** (`tower run claude`): Tower owns the process, hooks, PTY, approvals. Deterministic control.
- **Observed**: Visibility-only. No approval control. Copilot CLI, VS Code, WSL.
- The line is hard. Observed never unlocks managed capabilities.

## Session Identity (4 layers)

1. `session_id` — Tower logical ULID
2. `runtime_id` — Current process incarnation ULID
3. `adapter_ref` — Tool-native reference (opaque)
4. `process_fingerprint` — Executable, PID, start time, workspace, terminal ID

## Event Model

Normalized envelope: `docs/engineering/foundation-spec.md` lines 87–106, schema at `schemas/event-envelope-v1.schema.json`.

Event families: `session.{discovered,started,reconnected,ended}`, `state.changed`, `activity.excerpt.updated`, `approval.{requested,resolved}`, `command.{sent,applied}`, `conflict.{detected,resolved}`, `summary.updated`, `session.{parked,resumed}`, `error.reported`

## Materialized State

Each session snapshot carries: `control_mode`, `lifecycle` (discovered|launching|active|parked|completed|failed|detached), `activity` (running|waiting_human|waiting_tool|waiting_external|idle|unknown), `confidence` (certain|high|medium|low), `attention` (none|info|needs_user|urgent).

## Hook Integration (Primary Approval Channel)

Tower injects `.claude/settings.local.json` with hooks pointing at `http://localhost:<port>/hooks/<session_id>/<event>`.

Synchronous (Claude blocks): `PreToolUse`, `PermissionRequest`
Async (observational): `PostToolUse`, `PostToolUseFailure`, `SessionStart`, `SessionEnd`, `Stop`, `Notification`, `SubagentStart`, `SubagentStop`

Approval response format:
```json
{"hookSpecificOutput": {"hookEventName": "PermissionRequest", "decision": {"behavior": "allow"}}}
```

Batch auto-approve via PreToolUse:
```json
{"hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "allow", "permissionDecisionReason": "..."}}
```

## Risk Classes

`read_only`, `workspace_write`, `git_read`, `git_mutation`, `package_install`, `network_read`, `network_write`, `process_exec`, `secret_access`, `unknown`

## Daemon HTTP API

```
POST /hooks/<session_id>/pre-tool-use
POST /hooks/<session_id>/permission-request
POST /hooks/<session_id>/post-tool-use
POST /hooks/<session_id>/post-tool-use-failure
POST /hooks/<session_id>/session-start
POST /hooks/<session_id>/session-end
POST /hooks/<session_id>/stop
POST /hooks/<session_id>/notification
POST /hooks/<session_id>/subagent-start
POST /hooks/<session_id>/subagent-stop
```

Auth: `Bearer $TOWER_HOOK_TOKEN` header (per-session random secret).

## Key Contracts (source of truth: `internal/contracts/`)

- `SessionAdapter` interface: `Discover`, `Subscribe`, `Snapshot`, `Capabilities`, `Perform`
- `SessionSnapshot`: full materialized state
- `Event`: normalized envelope
- `ApprovalRequest`: tool, risk, prompt, fingerprint, freshness, normalized_key
- `AuditEntry`: operator, request/context snapshots, decision, result

## PTY Model

- Passthrough only in v1. Tower doesn't write to PTY stdin for approvals.
- Windows: ConPTY + Job Object cleanup
- macOS: PTY + process group cleanup
- If `tower run` exits, Claude dies. No detach/reattach in v1.

## Full Design Docs

- `docs/engineering/architecture-decisions.md` — locked decisions
- `docs/engineering/foundation-spec.md` — technical foundation
- `docs/engineering/claude-managed-adapter-design.md` — managed adapter implementation design
- `docs/requirements/v1-scope.md` — scope boundaries
- `docs/requirements/roadmap.md` — execution plan
