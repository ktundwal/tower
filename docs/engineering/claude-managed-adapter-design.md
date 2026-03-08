# Claude Managed Adapter Design

Status: proposed v1 implementation design (needs update, see note below)
Audience: Tower engineers
Depends on:

- `docs\engineering\architecture-decisions.md`
- `docs\engineering\foundation-spec.md`
- `docs\requirements\v1-scope.md`
- `docs\requirements\roadmap.md`

This document defines the implementation-facing design for Tower's managed Claude Code adapter in v1. It is intentionally narrow: it covers `tower run claude`, the managed runtime boundary, approval detection, command injection, recovery, and Claude-specific event payloads. It does not restate the general Tower architecture, cockpit UX, or observed-adapter design except where the managed boundary requires a precise rule.

> **Update needed (2026-03-07):** This document was written before the hook-mediated approval channel was validated as the primary approach. Section 6 (Approval Detection Strategy) assumes PTY output parsing as the primary detection mechanism with hooks as enrichment. Per `architecture-decisions.md` sections 11-13, hooks are now the **primary** approval channel: hooks fire on `PreToolUse` and `PermissionRequest` events with structured JSON, POST to Tower's daemon HTTP API, and return approval decisions directly. PTY output parsing is a fallback for edge cases, not the main path. Section 6 should be revised to reflect this inversion. The PTY/ConPTY wrapper (section 5) remains correct for terminal bridging, session identity, and detach/reattach.

## 0. Inputs, proposed decisions, and unproven assumptions

The goal here is to be explicit about what is already locked, what this document proposes, and what still requires validation.

| Statement | Status | Notes |
|---|---|---|
| Tower has a hard public boundary between `observed` and `managed` sessions. | Locked input | `docs\engineering\architecture-decisions.md` |
| Deterministic approvals and command injection require `tower run <tool>`. | Locked input | `docs\engineering\architecture-decisions.md` |
| V1 supports managed Claude Code on native Windows and native macOS. | Locked input | `docs\engineering\architecture-decisions.md`, `docs\requirements\v1-scope.md` |
| Terminal input remains user-owned by default during focused work. | Locked input | `docs\engineering\architecture-decisions.md` |
| Claude Code reads approval responses from stdin (`y`, `n`, or a message). | Observed prior art | `docs\brainstorm-product\research.md`; validate exact prompt shapes in prototype |
| Claude Code supports hooks via `.claude/settings.json` for tool-call-related events. | Observed prior art | `docs\brainstorm-product\research.md`; event coverage and safe config injection path are still unproven |
| Tower should not promise remote control for arbitrary already-running Claude sessions. | Locked input | architecture decisions, v1 scope, prior review docs |
| Managed Claude sessions should run behind a Tower-owned PTY/ConPTY wrapper. | Proposed here | This is the recommended design for v1 |
| A per-session runtime helper process should own the PTY/ConPTY and Claude child. | Proposed here | Chosen to preserve durable identity and enable recovery/reconnect |
| Hook signals can provide structured approval metadata. | Unproven assumption | Good candidate for enrichment, but must not be the only authority for remote approval |
| Claude prompt text and screen layout are stable enough for a versioned detector. | Unproven assumption | Must be validated with captured fixtures on Windows and macOS |

## 1. Purpose and scope

### Purpose

The managed Claude adapter exists to make one Tower promise truthful in v1:

- if a Claude Code session was launched through `tower run claude`, Tower can supervise it as a `managed` session with deterministic identity, auditable approvals, and explicit command injection
- if Tower did not launch the session, Tower does not pretend it has that level of control

### In scope

- launching Claude Code through `tower run claude`
- owning the PTY/ConPTY and child process tree
- attaching the launch terminal as the default foreground input surface
- detecting approval prompts with enough confidence to support managed workflows
- remotely approving, denying, or sending a short text response from the cockpit
- emitting Claude-managed-specific events into the normalized event model
- recovering managed sessions across Tower restarts or temporary cockpit disconnects
- clearly separating managed discovery from observed discovery

## 2. Non-goals and hard boundaries

| Item | Boundary |
|---|---|
| Remote control of arbitrary already-running Claude sessions | Out of scope. Those remain observed-only unless Claude exposes a real external control API. |
| Magic side-channel input injection into unmanaged consoles | Explicitly out of scope. No `AttachConsole`, simulated keystrokes, foreground-window automation, or equivalent as a product promise. |
| Universal managed support for other tools in v1 | Out of scope. Claude is the deep managed integration; Copilot CLI, VS Code, and WSL stay observed-first / observe-only. |
| Long-lived remote terminal replacement from the cockpit | Out of scope for v1. The cockpit may perform bounded input actions; it does not become the default terminal owner. |
| Silent mutation of repo files to install hooks | Out of scope until a safe, explicit, reversible config path is validated. |
| Rewriting the generic event envelope or overall Tower state model | Out of scope. This doc only defines Claude-managed payloads and runtime behavior. |

## 3. Design summary

`tower run claude` is a managed launch path, not a discovery trick.

The v1 design uses three cooperating Tower pieces:

1. **Tower core process**  
   Owns the event log, materialized state, policy checks, cockpit IPC, and durable session registry.

2. **Per-session Claude runtime helper**  
   A hidden helper process, spawned by Tower core, that owns the PTY/ConPTY, the Claude child process, approval detection, and all write-path actions. In v1 this should be implemented as a hidden subcommand of the same single `tower` binary (for example `tower internal claude-runtime`), not as a second shipped executable.

3. **Foreground terminal bridge**  
   The `tower run claude` command the user runs in their terminal. It attaches the current terminal to the runtime helper, forwards stdin/stdout/resize events, and can temporarily pause terminal input when the cockpit performs an explicit action.

### Why split helper and bridge

- the **runtime helper** is the authoritative owner of the managed session
- the **terminal bridge** is just the current attachment
- this keeps the session identity stable if Tower core restarts or the terminal is reattached later
- it also preserves the principle that the terminal is the default input owner while still allowing explicit, auditable cockpit control

### Runtime topology

```text
user terminal
    │
    ▼
tower run claude (foreground bridge)
    │ attach + stdin/stdout + resize
    ▼
claude-runtime (hidden helper) <---- spawn/register/reconnect ---- Tower core
    │
    └---- control + events --------------------------------------> Tower core
    │
    ▼
Claude Code child
```

## 4. End-to-end lifecycle for `tower run claude`

### 4.1 Launch flow

1. User runs `tower run claude [args...]` inside a terminal, from a workspace.
2. The bridge ensures Tower core is reachable. If Tower core is not running, `tower run` starts it or fails with a clear launch error.
3. The bridge sends `LaunchManagedClaude` to Tower core with:
   - current working directory
   - raw `argv`
   - terminal dimensions
   - terminal metadata if detectable (`TERM_PROGRAM`, `WT_SESSION`, tty name, etc.)
   - a sanitized environment snapshot for launch only
4. Tower core allocates:
   - `session_id` immediately, before process spawn
   - `runtime_id` for this process incarnation
5. Tower core spawns the per-session Claude runtime helper and persists the managed session registry entry.
6. The runtime helper:
   - resolves the Claude executable
   - creates the PTY/ConPTY backend
   - creates a local control endpoint
   - optionally enables Tower hook enrichment if configured and safe
   - spawns Claude attached to the PTY/ConPTY
7. The helper emits `session.started` and an initial `state.changed` to `launching` / `active`.
8. The bridge attaches its terminal streams to the helper and enters passthrough mode.
9. The helper mirrors PTY output to:
   - the attached terminal bridge
   - the approval detector / activity excerpt pipeline
   - Tower core event emission

### 4.2 Normal focused work

- terminal input is forwarded directly to the helper, then into the Claude PTY
- PTY output is shown in the terminal with no cockpit mediation
- Tower only observes, updates materialized state, and surfaces approvals/conflicts in the cockpit

### 4.3 Approval from the cockpit

1. The helper detects a live approval prompt and emits `approval.requested`.
2. The cockpit selects approve / deny / respond.
3. Tower core validates:
   - session is `managed`
   - action is unresolved
   - prompt is still fresh
   - no newer terminal input has already answered it
   - batch policy, if applicable
4. Tower core writes the audit intent before injection.
5. The helper acquires a short control lease, pauses terminal forwarding, injects the response, observes the prompt resolution, then releases the lease.
6. Tower core emits `approval.resolved` and updates state back to `running` or another post-action state.

### 4.4 Process exit

- when Claude exits normally, the helper emits `session.ended`
- when Claude crashes, the helper emits `error.reported` and then `session.ended`
- the helper cleans up PTY resources, control endpoints, and temporary session artifacts
- the session remains in Tower history with durable audit records

### 4.5 Recovery / reconnect

- if Tower core restarts while the helper and Claude are still alive, the restarted core reconnects to registered managed runtimes and emits `session.reconnected`
- if the terminal bridge disconnects while the helper and Claude are still alive, the session enters `detached` lifecycle and stays managed
- reattachment is a managed-only operation against the existing helper; it does not convert an observed session into a managed one

### 4.6 Park / resume boundary

Park and resume remain managed-only features. This document does not redefine the full park bundle format, but it does define the Claude-managed event payloads for `session.parked` and `session.resumed` so the runtime and store can be implemented consistently with the managed adapter boundary.

### Sequence diagram: launch and attach

```text
User terminal      tower run bridge      Tower core        Claude runtime        Claude
     |                    |                  |                   |                 |
1. run command ---------->|                  |                   |                 |
     |                    | launch request ->|                   |                 |
     |                    |                  | spawn helper ---->| create PTY      |
     |                    |                  |                   | exec ---------->|
     |                    |<-----------------| session metadata  |                 |
     | attach streams ==========================================>|================>|
     |<============================== PTY output mirror =========|<================|
```

## 5. PTY / ConPTY wrapper model

The managed adapter is a PTY-owner design. The Claude child never owns the user terminal directly; the runtime helper owns the PTY/ConPTY and the terminal attaches through the bridge.

### 5.1 Platform backend split

| Concern | Windows | macOS | Design rule |
|---|---|---|---|
| Pseudoterminal primitive | ConPTY | PTY (`openpty`/equivalent) | Keep a common `PTYBackend` interface with platform-specific implementations. |
| Child attachment | `CreatePseudoConsole` + `STARTUPINFOEX` pseudoconsole attribute | Spawn child on PTY slave as session leader | Child stdio always terminates at the PTY backend, never at the cockpit directly. |
| Local IPC | Named pipes | Unix domain sockets | The control protocol should be transport-agnostic. |
| Resize | `ResizePseudoConsole` | `TIOCSWINSZ` | Resize is driven by the terminal bridge. |
| Process cleanup | Job Object for child tree | Process group for child tree | Cleanup semantics belong to the helper, not the bridge. |
| Encoding / ANSI handling | ConPTY VT stream; handle Windows console mode edge cases | Native PTY stream | Treat the PTY output as byte/VT stream and normalize in one parser. |
| Interrupt semantics | Validate `Ctrl+C` / console event behavior in prototype | Validate PTY signal behavior in prototype | Do not hardcode cross-platform assumptions before prototype capture. |

### 5.2 Ownership model

| Resource | Owner | Notes |
|---|---|---|
| `session_id`, runtime registry, audit log | Tower core | Durable local state |
| PTY/ConPTY master + control handles | Claude runtime helper | Authoritative managed control surface |
| Claude stdin/stdout/stderr | Claude child via PTY slave / pseudoconsole | Never directly attached to Tower core |
| User terminal stdin/stdout while attached | Terminal bridge | Default owner of focused interaction |
| Cockpit-issued control actions | Tower core -> runtime helper over control IPC | Cockpit never writes to PTY directly |

### 5.3 Process spawning rules

- resolve the Claude executable before spawn
- avoid an interactive shell wrapper when direct execution is possible
- if Windows resolution lands on a batch wrapper such as `cc.cmd`, record both:
  - the transport wrapper used to launch it
  - the effective Claude tool path for identity/audit purposes
- always record:
  - resolved executable path
  - `argv`
  - working directory
  - platform backend
  - child PID and start time
- inject Tower-only environment variables for the managed runtime, but never persist the full environment blob in SQLite

Recommended Tower-only environment additions:

- `TOWER_MANAGED=1`
- `TOWER_SESSION_ID=<session id>`
- `TOWER_RUNTIME_ID=<runtime id>`
- `TOWER_CONTROL_ENDPOINT=<pipe or socket path>`
- `TOWER_HOOK_ENDPOINT=<pipe or socket path>` when hook enrichment is enabled

### 5.4 Passthrough mode

Passthrough mode is the default.

In passthrough mode:

- terminal input goes to the bridge
- the bridge forwards bytes and resize events to the helper
- the helper writes bytes to the Claude PTY
- PTY output is mirrored back to the bridge and into Tower's read path
- Tower may observe and summarize, but it does not take input ownership

### 5.5 Interception mode

Interception mode is entered only for an explicit cockpit action:

- approve
- deny
- send a short response message
- inject a bounded operator command/text

Interception mode is intentionally short-lived:

1. helper requests the bridge to pause terminal input forwarding
2. bridge stops reading further stdin and acknowledges the last forwarded input epoch
3. helper verifies the action is still fresh against that epoch and the live prompt fingerprint
4. helper injects bytes into the PTY
5. helper observes resolution or failure
6. helper tells the bridge to resume passthrough

The bridge should pause by **stopping reads**, not by draining and discarding user keystrokes. This preserves pending terminal input in the OS buffer rather than losing it.

## 6. Approval detection strategy

The adapter needs two things at once:

1. proof that Claude is **actually blocked right now** on a prompt that will read stdin
2. enough structure to classify the prompt, show context, and apply safe policy

### 6.1 Recommended primary strategy: hook-enriched, PTY-confirmed detection

Recommended steady-state v1 path, once checkpoint 4 in section 13.2 validates a safe hook path:

1. **Hook candidate**  
   If a Tower-compatible Claude hook is available, it emits structured metadata for a candidate tool action:
   - tool name
   - arguments / target paths
   - workspace/cwd
   - timestamp
   - optional Claude-native identifiers if available

2. **PTY confirmation**  
   The helper's PTY parser confirms that Claude rendered a live approval prompt and is waiting for stdin.

3. **Action materialization**  
   Only once PTY confirmation exists does Tower emit `approval.requested` as remotely actionable.

Why this is the recommended path:

- the hook gives better risk classification, normalized keys, and audit context
- the PTY confirmation proves the prompt is live and currently blocking
- the combination avoids treating "tool call happened" as equivalent to "approval is still pending"

Bring-up note: the first end-to-end implementation may ship PTY-only detection before hook enrichment is proven. That does not change the long-term recommended shape; it only changes the order of implementation.

### 6.2 Fallback strategy: PTY-only detection

If hook enrichment is unavailable, misconfigured, or loses connectivity:

- the helper still parses the PTY output stream
- Tower may still emit `approval.requested`
- missing fields fall back conservatively:
  - `risk_class = "unknown"` unless the parser can classify with high confidence
  - `normalized_key` omitted
  - batch approval disabled
  - `confidence` reduced if prompt detection is ambiguous

PTY-only detection is acceptable for managed single-action approvals, but not for optimistic batching.

### 6.3 `normalized_key` format

`normalized_key` exists only to support safe grouping for read-only batch approvals. It must be deterministic across platforms and independent of prompt wording.

Emit `normalized_key` only when the adapter has enough structured data to canonicalize safely. Otherwise omit it.

Format:

```text
<risk_class>:<tool_family>:<operation>:<target_hash>
```

Where:

- `risk_class` is one of the normalized foundation values, usually `read_only` for batch-eligible approvals
- `tool_family` is a lowercased stable family such as `bash`, `read`, `glob`, or `git`
- `operation` is a lowercased canonical action label such as `git-status`, `read-file`, or `grep`
- `target_hash` is the lowercase hex SHA-256 of canonical JSON for the relevant arguments

Canonical JSON rules:

- use absolute cleaned paths with platform-native separators
- sort arrays and object keys
- collapse redundant whitespace in shell commands
- lowercase tool family and operation labels
- omit timestamps, ANSI text, and other display-only fields

If any of those inputs are missing or ambiguous, omit `normalized_key` and disable batch eligibility for that request.

### 6.4 Detector design

The detector should be screen-state based, not line-grep based.

Required parser behavior:

- consume VT/ANSI output and reconstruct the logical visible prompt region
- normalize cursor movement, color sequences, and line wrapping
- produce a `prompt_fingerprint` hash over the normalized visible approval region
- keep the latest confirmed prompt state with timestamp and terminal input epoch
- version pattern definitions from captured fixtures instead of hardcoding ad hoc regexes in business logic

### 6.5 Freshness and race proof

Every forwarded terminal input chunk increments a monotonically increasing `terminal_input_epoch`.

For each `approval.requested`, the helper records:

- `input_epoch_at_prompt`
- `prompt_fingerprint`
- `confirmed_waiting_at`
- `fresh_until` (time guardrail; recommended initial default: 30 seconds)

A cockpit approval is allowed only if all are true:

- current terminal input epoch equals `input_epoch_at_prompt`
- current live prompt fingerprint still matches
- the request is unresolved
- current time is before `fresh_until`
- session is still `waiting_human`

If any check fails, Tower must not inject. The action becomes stale and falls back to individual terminal handling or explicit re-review.

### 6.6 Local-terminal-wins rule

If terminal input reaches the PTY before the helper acquires the control lease:

- local terminal input wins
- Tower marks the pending action resolved or superseded based on observed evidence
- any cockpit action waiting on that approval fails with a stale-action result

This rule is critical to preserving trust.

### 6.7 Detached-session rule

Cockpit approvals and command injections are allowed while a managed session is `detached`, but only if:

- there is no currently attached terminal bridge
- the helper still owns the live PTY/ConPTY
- the same freshness checks still pass

In detached mode, lease acquisition skips the bridge-pause step and validates directly against the helper's current `terminal_input_epoch` and live prompt fingerprint. If a new bridge attaches before injection completes, the cockpit action must fail stale.

### Sequence diagram: cockpit approval with lease

```text
Cockpit            Tower core         Claude runtime      Terminal bridge        Claude
   |                   |                    |                    |                 |
1. approve(action) --->|                    |                    |                 |
   |                   | validate + audit   |                    |                 |
   |                   |------------------->| pause request ---->|                 |
   |                   |                    |<-------------------| paused @ epoch 42
   |                   |                    | verify prompt+epoch |                 |
   |                   |                    | inject "y\n" ----------------------->|
   |                   |                    | observe prompt clear / tool start    |
   |<------------------| approval.resolved  | resume ----------->|                 |
```

## 7. Command injection and handoff semantics

In this adapter, "command injection" means **submitting explicit operator input into the managed Claude session**. It does not mean spawning an arbitrary shell command outside Claude.

### 7.1 Supported v1 input actions

- approve
- deny
- send a short response message
- send a bounded operator command/text block

### 7.2 Explicit handoff rules

| Rule | Behavior |
|---|---|
| Default owner | Terminal bridge owns input while attached. |
| Cockpit write path | Requires an explicit control action and a short control lease. |
| Lease duration | Short-lived, only long enough to pause terminal reads, inject, observe result, and release. |
| Mixed typing | If local terminal input appears before lease acquisition, local input wins and the cockpit action fails stale. |
| Detached managed session | Cockpit may act directly through the helper if no bridge is attached; a new bridge attach invalidates the in-flight lease. |
| Long interactive work | User should jump back to the terminal. The cockpit is not a full remote REPL in v1. |
| Visibility | Cockpit-injected text must still appear in the managed terminal stream. No invisible side channel. |

### 7.3 Input lease states

```text
terminal_attached_passthrough
    -> cockpit_lease_pending
    -> cockpit_injecting
    -> terminal_attached_passthrough

terminal_attached_passthrough
    -> detached
    -> terminal_attached_passthrough   (on reattach)
```

Recommended runtime semantics:

- only one active lease per session
- a lease is bound to one `action_id` or `command_id`
- lease acquisition requires the caller to present the expected `terminal_input_epoch`
- the helper is the only component allowed to inject into the PTY

## 8. Session discovery boundaries

The managed Claude adapter and the observed discovery system must not blur together.

### 8.1 Managed adapter responsibilities

The managed Claude adapter is responsible for:

- launching Claude via `tower run claude`
- minting and persisting `session_id` / `runtime_id`
- owning the PTY/ConPTY and child process tree
- reconnecting to Tower-launched managed helper processes after Tower restart
- attaching and detaching user terminals to existing managed runtimes
- emitting high-confidence events for managed sessions
- performing approvals and command injections for those managed sessions

### 8.2 Observed discovery responsibilities

Observed discovery is responsible for:

- scanning for arbitrary already-running Claude processes
- best-effort process correlation and session descriptors
- low-confidence continuity across rediscovery
- liveness, recent activity, deep-linking, and other observe-only surfaces

Observed discovery is **not** responsible for:

- claiming ownership of Tower-managed runtime helpers
- upgrading arbitrary Claude processes into managed sessions
- performing remote approvals or input injection

### 8.3 De-duplication rule

If observed discovery sees a Claude process that matches a registered managed runtime:

- the managed record remains authoritative
- observed evidence may enrich liveness or process fingerprint fields
- no duplicate observed session should be shown

### 8.4 Recovery boundary

On Tower startup:

1. managed adapter checks the managed runtime registry and reconnects to helper processes it previously launched
2. observed discovery separately scans the machine for arbitrary Claude sessions
3. only the first path restores managed control

## 9. Claude-managed event payloads

The normalized event envelope from `docs\engineering\foundation-spec.md` stays unchanged. This section defines the `payload` shape expectations for Claude-managed sessions.

### 9.1 Event kinds covered here

- `session.discovered`
- `session.started`
- `session.reconnected`
- `session.parked`
- `session.resumed`
- `session.ended`
- `state.changed`
- `approval.requested`
- `approval.resolved`
- `command.sent`
- `command.applied`
- `error.reported`

Other generic event families may still be emitted, but the above are the minimum payloads the managed Claude runtime must support.

### 9.2 Payload requirements by event

#### `session.discovered` (managed recovery only)

Used when Tower core rediscovers a helper process from its own managed runtime registry, not when it launches a new one.

Required fields:

- `discovery_source` (`managed_registry`)
- `session_id`
- `runtime_id`
- `helper_pid`
- `child_pid`
- `workspace_root`
- `platform_backend` (`windows_conpty` or `darwin_pty`)
- `terminal_attached` (bool)

Optional fields:

- `terminal_metadata`
- `hook_mode`
- `adapter_ref`

#### `session.started`

Required fields:

- `launch_kind` (`tower_run`)
- `workspace_root`
- `argv`
- `resolved_executable`
- `platform_backend`
- `helper_pid`
- `child_pid`
- `adapter_ref`
- `terminal_attached`

Optional fields:

- `transport_wrapper`
- `repo_root`
- `terminal_metadata`
- `hook_mode`

#### `session.reconnected`

Required fields:

- `previous_runtime_id`
- `current_runtime_id`
- `reason` (`tower_restart` \| `terminal_reattach` \| `helper_reconnect`)
- `helper_pid`
- `child_pid`
- `terminal_attached`

Optional fields:

- `reattached_terminal_metadata`
- `gap_detected` (bool)

#### `session.parked`

Required fields:

- `park_id`
- `reason`
- `artifact_path`
- `helper_pid`
- `child_pid`
- `runtime_id`
- `terminal_attached`

Optional fields:

- `workspace_root`
- `repo_root`
- `resume_hint`

#### `session.resumed`

Required fields:

- `park_id`
- `previous_runtime_id`
- `current_runtime_id`
- `reason` (`user_resume` \| `recovery_resume`)
- `workspace_root`
- `platform_backend`
- `helper_pid`
- `child_pid`

Optional fields:

- `terminal_attached`
- `terminal_metadata`
- `adapter_ref`

#### `session.ended`

Required fields:

- `reason` (`exit_0` \| `exit_nonzero` \| `interrupted` \| `helper_failure` \| `launch_failure`)
- `ended_at`
- `helper_pid`
- `child_pid`
- `final_lifecycle`

Optional fields:

- `exit_code`
- `signal`
- `duration_ms`
- `last_known_activity`

#### `state.changed`

Required fields:

- `previous_lifecycle`
- `lifecycle`
- `previous_activity`
- `activity`
- `confidence`
- `reason`

Optional fields:

- `attention`
- `action_id`
- `excerpt`
- `detector`

Recommended `reason` values for this adapter:

- `launch_complete`
- `approval_detected`
- `approval_cleared`
- `command_injected`
- `terminal_detached`
- `terminal_reattached`
- `hook_channel_lost`
- `parser_desynced`
- `process_exited`

#### `approval.requested`

Required fields:

- `action_id`
- `approval_kind` (`tool_call`; only allowed v1 value)
- `tool_name` (use `"unknown"` if not known)
- `risk_class` (use `"unknown"` if not known)
- `prompt_excerpt`
- `prompt_fingerprint`
- `detector` (`hook_confirmed_by_pty` \| `pty_only`)
- `input_epoch_at_prompt`
- `confirmed_waiting_at`
- `fresh_until`
- `decision_options` (array; v1 expected values are `approve`, `deny`, `message`)

Optional fields:

- `normalized_key`
- `tool_args`
- `cwd`
- `repo_root`
- `hook_event_id`
- `display_title`
- `display_subtitle`

#### `approval.resolved`

Required fields:

- `action_id`
- `resolution` (`approved` \| `denied` \| `message_sent` \| `superseded` \| `expired` \| `cancelled`)
- `resolved_via` (`terminal` \| `cockpit` \| `process_exit` \| `reconciliation`)
- `result` (`accepted` \| `stale` \| `failed` \| `unknown`)
- `resolved_at`
- `input_epoch_at_resolution`

Optional fields:

- `operator`
- `message_excerpt`
- `evidence`
- `latency_ms`
- `error_code`

#### `command.sent`

Required fields:

- `command_id`
- `input_kind` (`operator_text`)
- `source_surface` (`cockpit`)
- `text_excerpt`
- `text_sha256`
- `expected_input_epoch`

Optional fields:

- `action_context_id`
- `request_id`
- `interrupt_before_send`

#### `command.applied`

Required fields:

- `command_id`
- `result` (`applied` \| `stale` \| `rejected` \| `failed`)
- `applied_at`
- `input_epoch_at_apply`

Optional fields:

- `echo_excerpt`
- `error_code`
- `state_after`

#### `error.reported`

Required fields:

- `code`
- `component`
- `operation`
- `message`
- `recoverable` (bool)

Optional fields:

- `session_id`
- `runtime_id`
- `action_id`
- `os_error`
- `details`

### 9.3 Example payload: `approval.requested`

```json
{
  "schema_version": "v1",
  "event_id": "01JEXAMPLE",
  "kind": "approval.requested",
  "session_id": "01JSESSION",
  "runtime_id": "01JRUNTIME",
  "control_mode": "managed",
  "source": {
    "adapter": "claude",
    "tool": "claude-code",
    "host": "local"
  },
  "occurred_at": "2026-03-06T18:00:00Z",
  "ingested_at": "2026-03-06T18:00:00Z",
  "confidence": "certain",
  "payload": {
    "action_id": "01JACTION",
    "approval_kind": "tool_call",
    "tool_name": "Bash",
    "risk_class": "read_only",
    "prompt_excerpt": "Claude wants to run `git status`",
    "prompt_fingerprint": "sha256:2f4d...",
    "detector": "hook_confirmed_by_pty",
    "input_epoch_at_prompt": 42,
    "confirmed_waiting_at": "2026-03-06T18:00:00Z",
    "fresh_until": "2026-03-06T18:00:30Z",
    "decision_options": ["approve", "deny", "message"],
    "normalized_key": "read_only:bash:git-status:91ab3e8d4c7f...",
    "tool_args": {
      "command": "git status",
      "cwd": "C:\\github\\tower"
    }
  }
}
```

### 9.4 Example payload: `command.sent`

```json
{
  "schema_version": "v1",
  "event_id": "01JCOMMAND",
  "kind": "command.sent",
  "session_id": "01JSESSION",
  "runtime_id": "01JRUNTIME",
  "control_mode": "managed",
  "source": {
    "adapter": "claude",
    "tool": "claude-code",
    "host": "local"
  },
  "occurred_at": "2026-03-06T18:05:00Z",
  "ingested_at": "2026-03-06T18:05:00Z",
  "confidence": "certain",
  "payload": {
    "command_id": "01JCMDID",
    "input_kind": "operator_text",
    "source_surface": "cockpit",
    "text_excerpt": "Please summarize the failing tests before editing.",
    "text_sha256": "sha256:ab31...",
    "expected_input_epoch": 42
  }
}
```

## 10. Failure modes and recovery behavior

| Failure mode | Expected behavior |
|---|---|
| Claude executable cannot be resolved | Emit `error.reported(code=claude_exec_missing, recoverable=false)` and fail launch before `session.started`. |
| PTY/ConPTY creation fails | Emit `error.reported(code=pty_create_failed, recoverable=false)` and fail launch. |
| Control endpoint creation fails | Fail launch; managed session cannot exist without Tower-owned control IPC. |
| Hook enrichment unavailable | Launch continues in PTY-only mode, emit `error.reported(code=hook_unavailable, recoverable=true)`, disable batch eligibility for unclassified approvals. |
| PTY parser loses confidence / desyncs | Emit `error.reported(code=approval_parser_desynced, recoverable=true)`, degrade session activity/confidence, disable remote approval until prompt state is re-established. |
| Terminal bridge disconnects unexpectedly | Helper keeps session alive, emit `state.changed(reason=terminal_detached, lifecycle=detached)`, allow managed reattach later. |
| Tower core restarts | Helper stays authoritative; restarted core reconnects from registry and emits `session.reconnected`. |
| Cockpit action races with local terminal input | Reject cockpit action as stale; emit `approval.resolved(result=stale, resolved_via=reconciliation)` or `command.applied(result=stale)`. |
| Claude exits while approval is pending | Emit `approval.resolved(resolution=cancelled, resolved_via=process_exit)` followed by `session.ended`. |
| Helper dies but Claude child remains | Treat as managed runtime failure; do not attempt side-channel salvage. Mark session `unknown` or `failed` and require relaunch or explicit recovery tooling later. |

### Recovery principle

Recovery should only restore **truth the adapter can prove**:

- if the helper still owns the PTY and can report state, reconnect
- if the helper or proof path is gone, degrade to `unknown` / `failed`
- do not guess a strong managed state from process scans alone

## 11. Security and trust constraints

- Tower only injects input into PTYs/ConPTYs that it owns for managed sessions.
- Tower does not inject into arbitrary consoles, windows, or unrelated processes.
- Control endpoints must be local-user-only:
  - Windows: named pipe ACL restricted to the current user
  - macOS: Unix socket with user-only permissions
- Every cockpit control action is audited before injection and closed out after result observation.
- Batch approval is allowed only when the foundation policy is satisfied; otherwise fall back to individual review.
- Hook data is advisory until correlated with the live managed runtime state.
- Cockpit-injected text must be visible in the terminal stream; no hidden side-channel control.
- The adapter must not silently rewrite tracked repo files or persistent user Claude config to install hooks. Any hook configuration path must be explicit, reversible, and validated first.
- Persist only bounded excerpts, hashes, and structured metadata needed for state and audit; do not store full inherited environment snapshots.

## 12. Go-oriented runtime boundary

The foundation spec already defines the generic adapter contract. The managed Claude adapter needs a sharper runtime boundary underneath that contract.

### 12.1 Core-facing adapter surface

```go
package claude

import "context"

type Adapter interface {
    DiscoverManaged(ctx context.Context) ([]ManagedDescriptor, error)
    Subscribe(ctx context.Context) (<-chan Event, error)
    Snapshot(ctx context.Context, sessionID string) (ManagedSnapshot, error)
    Capabilities(ctx context.Context, sessionID string) (CapabilitySet, error)
    Perform(ctx context.Context, sessionID string, action ManagedAction) (ActionResult, error)
    Launch(ctx context.Context, req LaunchRequest) (LaunchResult, error)
}
```

### 12.2 Runtime host boundary

```go
type RuntimeHost interface {
    Start(ctx context.Context, spec RuntimeSpec) (RuntimeHandle, error)
    Reconnect(ctx context.Context, reg ManagedDescriptor) (RuntimeHandle, error)
}

type RuntimeHandle interface {
    SessionID() string
    RuntimeID() string
    Events() <-chan Event
    Snapshot(ctx context.Context) (ManagedSnapshot, error)
    AcquireLease(ctx context.Context, req LeaseRequest) (LeaseResult, error)
    InjectApproval(ctx context.Context, req ApprovalResponse) error
    InjectText(ctx context.Context, req TextInjection) error
    AttachTerminal(ctx context.Context, req TerminalAttachRequest) error
    DetachTerminal(ctx context.Context, reason string) error
    Terminate(ctx context.Context, force bool) error
}
```

### 12.3 PTY backend boundary

```go
type PTYBackend interface {
    Start(ctx context.Context, spec SpawnSpec) (ChildProcess, error)
    WriteInput(ctx context.Context, data []byte) error
    ReadOutput(ctx context.Context) ([]byte, error)
    Resize(cols, rows uint16) error
    Close() error
}
```

### 12.4 Key data types

```go
type LaunchRequest struct {
    WorkspaceRoot string
    Args          []string
    Env           map[string]string // launch only; do not persist wholesale
    Terminal      TerminalMetadata
}

type TerminalMetadata struct {
    Columns     uint16
    Rows        uint16
    Program     string
    SessionHint string
    TTYName     string
}

type ManagedDescriptor struct {
    SessionID       string
    RuntimeID       string
    AdapterRef      string
    WorkspaceRoot   string
    HelperPID       int
    ChildPID        int
    PlatformBackend string
    TerminalAttached bool
}

type LeaseRequest struct {
    SessionID          string
    Owner              string // "cockpit"
    ExpectedInputEpoch uint64
    ActionID           string
}

type ApprovalResponse struct {
    ActionID           string
    Decision           string // approve | deny | message
    Message            string
    ExpectedInputEpoch uint64
}

type TextInjection struct {
    CommandID          string
    Text               string
    ExpectedInputEpoch uint64
}

type ManagedAction struct {
    Kind string
    Approval *ApprovalResponse
    Text     *TextInjection
}
```

### 12.5 Implementation note

The helper is the only process allowed to:

- own the PTY backend
- interpret freshness against the live prompt
- inject bytes into Claude stdin

Tower core decides **whether** an action is allowed; the helper decides **whether it is still safe right now** against the live runtime state.

## 13. Open risks and validation plan

### 13.1 Open risks

| Risk | Why it matters |
|---|---|
| Claude hook coverage may not expose the exact approval lifecycle needed. | Without structured metadata, batching and rich audit context get weaker. |
| Prompt layouts may differ across Claude versions or platforms. | PTY parsing is the most fragile part of the adapter. |
| Windows launch resolution may involve wrappers like `cc.cmd`. | This affects identity, process cleanup, and exact child-tree semantics. |
| Terminal pause/resume behavior may differ across Windows Terminal, PowerShell, and macOS terminals. | Lease acquisition must not drop user keystrokes or deadlock the session. |
| Helper crash recovery may be harder than core reconnect recovery. | Managed trust depends on the helper being the authoritative runtime owner. |

### 13.2 Prototype checkpoints

1. **Windows ConPTY launch proof**
   - launch Claude through a helper-owned ConPTY
   - preserve direct terminal use
   - verify resize, output fidelity, and child-tree cleanup

2. **macOS PTY launch proof**
   - same as above on native macOS
   - verify parity of prompt detection and process-group cleanup

3. **Approval fixture capture**
   - capture real Claude approval prompts across read-only, write, git, and network examples
   - produce versioned fixtures for parser tests
   - confirm what can be classified safely from prompt alone

4. **Hook viability check**
   - validate whether a safe, reversible Tower hook path exists without mutating tracked repo files
   - document the exact event coverage Claude exposes
   - decide whether hook enrichment ships in the first managed slice or lands immediately after PTY-only bring-up

5. **Race and freshness test matrix**
   - terminal approves before cockpit
   - cockpit approves before terminal
   - stale prompt after timeout
   - prompt fingerprint changes before injection
   - terminal detach / reattach during a pending approval

6. **Recovery proof**
   - restart Tower core while managed helper + Claude stay alive
   - reconnect without minting a new `session_id`
   - degrade to `unknown` when proof is missing

## 14. Immediate implementation order

Recommended build order:

1. helper process + PTY/ConPTY abstraction
2. terminal bridge attach/pause/resume loop
3. managed runtime registry + reconnect path
4. PTY output parser with captured fixtures
5. approval freshness / lease logic
6. cockpit `Perform` wiring for approve/deny/text
7. optional hook enrichment once validated

This order keeps the hard boundary intact: first make `tower run claude` truly managed, then add richer classification on top.
