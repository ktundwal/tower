# Tower Foundation Spec

This file is the concrete technical foundation for the first implementation of Tower.

## Locked stack

- **Tower core:** Go
- **Terminal UI:** Bubble Tea + Bubbles + Lip Gloss
- **Local storage:** SQLite
- **Marketing site:** Next.js on Vercel
- **Distribution:** single `tower` binary for Windows and macOS, with the landing site deployed separately

## Delivery model

- Tower v1 is a local-first desktop-terminal product
- Tower core runs as one main local process
- v1 adapters are in-process Go modules
- a future out-of-process adapter protocol is explicitly deferred until after the first truthful v1 slice

## Repo layout

```text
/cmd/tower/
/cmd/tower-demo/
/internal/app/
/internal/core/
/internal/store/
/internal/runtime/
/internal/platform/windows/
/internal/platform/darwin/
/internal/adapters/claude/
/internal/adapters/copilot/
/internal/adapters/vscode/
/internal/adapters/wsl/
/internal/ui/
/internal/policy/
/internal/conflicts/
/internal/summaries/
/internal/contracts/
/deployment/
/test/
/web/
```

## Local data layout

**Windows**

- `%LocalAppData%\Tower\tower.db`
- `%LocalAppData%\Tower\parked\`
- `%LocalAppData%\Tower\artifacts\`

**macOS**

- `~/Library/Application Support/Tower/tower.db`
- `~/Library/Application Support/Tower/parked/`
- `~/Library/Application Support/Tower/artifacts/`

Stored locally:

- append-only normalized events
- materialized session snapshots
- approval requests and resolutions
- conflict records
- parked-session bundles
- bounded activity excerpts
- audit entries

## Session identity model

Each session uses four identity layers:

1. `session_id` - Tower logical session ID, ULID string
2. `runtime_id` - current process incarnation or attachment, ULID string
3. `adapter_ref` - opaque tool-native reference
4. `process_fingerprint` - observed runtime fingerprint such as executable path, PID, start time, workspace, and terminal identity

Rules:

- managed sessions always get a `session_id` at launch time
- managed restarts or reconnects keep the same `session_id` and mint a new `runtime_id`
- observed continuity is correlation-only and must remain confidence-tagged
- observed correlation never unlocks managed capabilities

## Normalized event envelope

```json
{
  "schema_version": "v1",
  "event_id": "01J...",
  "kind": "approval.requested",
  "session_id": "01J...",
  "runtime_id": "01J...",
  "control_mode": "managed",
  "source": {
    "adapter": "claude",
    "tool": "claude-code",
    "host": "local"
  },
  "occurred_at": "2026-03-06T18:00:00Z",
  "ingested_at": "2026-03-06T18:00:00Z",
  "confidence": "certain",
  "correlation_id": "01J...",
  "causation_id": "01J...",
  "payload": {}
}
```

Initial event families:

- `session.discovered`
- `session.started`
- `session.reconnected`
- `session.ended`
- `state.changed`
- `activity.excerpt.updated`
- `approval.requested`
- `approval.resolved`
- `command.sent`
- `command.applied`
- `conflict.detected`
- `conflict.resolved`
- `summary.updated`
- `session.parked`
- `session.resumed`
- `error.reported`

## Materialized session state model

Each materialized session snapshot carries:

- `control_mode`: `managed | observed`
- `lifecycle`: `discovered | launching | active | parked | completed | failed | detached`
- `activity`: `running | waiting_human | waiting_tool | waiting_external | idle | unknown`
- `confidence`: `certain | high | medium | low`
- `attention`: `none | info | needs_user | urgent`
- `task_excerpt`
- `workspace_root`
- `repo_root`
- `branch_name`
- `pending_action_count`
- `conflict_count`
- `last_activity_at`
- `summary_excerpt`

## Approval policy foundation

Normalized risk classes:

- `read_only`
- `workspace_write`
- `git_read`
- `git_mutation`
- `package_install`
- `network_read`
- `network_write`
- `process_exec`
- `secret_access`
- `unknown`

V1 batch approval rules:

- only managed sessions are eligible
- only read-only requests are eligible
- requests must share the same normalized key
- every request must still be unresolved and fresh
- Tower must be able to prove no newer local terminal input already answered the prompt
- the session must still be in `waiting_human` with high or certain confidence

## Audit model

Every control action gets a durable local audit entry with:

- `audit_id`
- `session_id`
- `action_id`
- `operator`
- `request_snapshot`
- `context_snapshot`
- `decision`
- `decision_at`
- `result`
- `result_event_ids`

Audit history is retained locally until the user deletes it.

## Minimal conflict model for v1

The core engine must reason about:

- `repo_root`
- `branch_name`
- `touched_paths`
- `git_operation_class`
- `task_excerpt`

V1 conflict detectors:

- same repo + same file overlap
- same repo + same branch + concurrent git mutation
- same repo + overlapping changed-file sets

## Internal adapter contract for v1

```text
Discover() -> []SessionDescriptor
Subscribe(ctx) -> <-chan Event
Snapshot(sessionID) -> SessionSnapshot
Capabilities(sessionID) -> CapabilitySet
Perform(sessionID, Action) -> ActionResult
```

Rules:

- `Perform` is a no-op for unsupported capabilities
- observed adapters can surface data without pretending they can control
- managed adapters must provide stable action IDs for approvals and commands

## Demo harness

Tower needs synthetic fixtures from day one:

- 6-session demo streams
- 4 read-only approval batch fixture
- same-file conflict fixture
- parked-session bundle fixture
- end-of-day summary fixture
- `tower-demo` command to replay fixtures into the cockpit

## Explicit deferrals

- external adapter SDK
- cross-machine orchestration
- cloud sync
- shared multi-user control
- infinite transcript retention
- universal managed support across all tools
