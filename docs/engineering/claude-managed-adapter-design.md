# Claude Managed Adapter Design

Status: v1 implementation design
Audience: Tower engineers
Last updated: 2026-03-08
Depends on:

- `docs\engineering\architecture-decisions.md`
- `docs\engineering\foundation-spec.md`
- `docs\requirements\v1-scope.md`
- `docs\requirements\roadmap.md`

This document defines the implementation-facing design for Tower's managed Claude Code adapter in v1. It covers `tower run claude`, the managed runtime boundary, hook-based approval handling, recovery, and Claude-specific event payloads. It does not restate the general Tower architecture, cockpit UX, or observed-adapter design except where the managed boundary requires a precise rule.

## 0. Inputs, proposed decisions, and unproven assumptions

The goal here is to be explicit about what is already locked, what this document proposes, and what still requires validation.

| Statement | Status | Notes |
|---|---|---|
| Tower has a hard public boundary between `observed` and `managed` sessions. | Locked input | `docs\engineering\architecture-decisions.md` |
| Deterministic approvals require `tower run <tool>`. | Locked input | `docs\engineering\architecture-decisions.md` |
| V1 supports managed Claude Code on native Windows and native macOS. | Locked input | `docs\engineering\architecture-decisions.md`, `docs\requirements\v1-scope.md` |
| Terminal input remains user-owned by default during focused work. | Locked input | `docs\engineering\architecture-decisions.md` |
| Tower should not promise remote control for arbitrary already-running Claude sessions. | Locked input | architecture decisions, v1 scope, prior review docs |
| Claude Code supports HTTP hooks that POST structured JSON and accept JSON responses for approve/deny decisions. | Must validate | [Claude Code hooks reference](https://code.claude.com/docs/en/hooks) documents the API. `PermissionRequest` hook fires when a permission dialog is about to show; `PreToolUse` fires before every tool call. Prototype must confirm exact behavior for: allow/deny responses, timeout fallback, and hook merge semantics when multiple hooks return decisions. |
| Hooks block Claude's execution until the hook handler responds. | Must validate | Claude Code docs say hooks are synchronous by default. Non-2xx and connection failures are documented as "non-blocking errors" where "execution continues." Exact fallback behavior (does Claude show the terminal prompt? skip the tool? something else?) must be prototyped and documented. |
| Hook configuration can be injected via `.claude/settings.local.json` at launch time. | Must validate | `tower run` writes hook config to `.claude/settings.local.json` in the project directory. Must confirm this file is gitignored by default. Env-based injection is not documented in the hooks reference and is not part of the v1 plan. |
| Hook config is snapshotted at startup and used for the session lifetime. | Confirmed | Claude Code docs: "Claude Code captures a snapshot of hooks at startup." Consequence: daemon must restart on the same port with the same auth token. If it can't, existing sessions are orphaned. |
| `tower run` owns the PTY/ConPTY. If `tower run` exits, the Claude session dies. | Proposed here | No separate "runtime helper" process. No detach/reattach in v1. PTY is for terminal bridging and process lifecycle only. |
| V1 cockpit actions are limited to approve/deny. No operator text injection. | Proposed here | Approvals are handled entirely via hook responses. PTY stdin writes are not needed in v1. |
| Claude's permission mode must be compatible with Tower approval control. | Proposed here | If Claude runs in `dontAsk` or `bypassPermissions`, `PermissionRequest` never fires. Tower must detect this and downgrade to `managed_visibility_only`. |
| Hook merge semantics must be safe for Tower's approval claims. | Must validate | Claude runs matching hooks in parallel. If another hook can return `allow` before Tower responds, Tower's approval control is bypassed. This is a **release blocker**: if safe merge can't be proven, managed mode must require exclusive ownership of `PermissionRequest` hooks or downgrade to `managed_visibility_only`. |

## 1. Purpose and scope

### Purpose

The managed Claude adapter exists to make one Tower promise truthful in v1:

- if a Claude Code session was launched through `tower run claude`, Tower can supervise it as a `managed` session with deterministic identity and auditable approvals
- if Tower did not launch the session, Tower does not pretend it has that level of control

### In scope

- launching Claude Code through `tower run claude`
- owning the PTY/ConPTY and child process tree
- injecting hook configuration at launch so Claude posts events to Tower's HTTP API
- approving or denying tool calls from the cockpit via hook responses
- emitting Claude-managed-specific events into the normalized event model
- clearly separating managed discovery from observed discovery

### Not in scope for v1

- operator text injection (sending messages to Claude from the cockpit)
- PTY output parsing for approval detection
- lease/pause/resume interception model
- detach/reattach (if `tower run` exits, the session dies)
- daemon restart recovery with PTY reconnection

These are deferred because hook-based approvals handle the v1 cockpit use case (approve/deny from one place) without PTY stdin writes or complex process management.

## 2. Non-goals and hard boundaries

| Item | Boundary |
|---|---|
| Remote control of arbitrary already-running Claude sessions | Out of scope. Those remain observed-only unless Claude exposes a real external control API. |
| Magic side-channel input injection into unmanaged consoles | Explicitly out of scope. No `AttachConsole`, simulated keystrokes, foreground-window automation, or equivalent as a product promise. |
| Universal managed support for other tools in v1 | Out of scope. Claude is the deep managed integration; Copilot CLI, VS Code, and WSL stay observed-first / observe-only. |
| Operator text injection from cockpit | Deferred past v1. If you need to talk to Claude, switch to that terminal. The cockpit is for supervision, not a remote REPL. |
| Silent mutation of repo files to install hooks | Out of scope. Hook injection uses `.claude/settings.local.json` (gitignored), never tracked repo files. |
| Rewriting the generic event envelope or overall Tower state model | Out of scope. This doc only defines Claude-managed payloads and runtime behavior. |

## 3. Design summary

`tower run claude` is a managed launch path, not a discovery trick.

The v1 design has two key mechanisms:

1. **Hook-based approval channel** (primary)
   Tower daemon runs an HTTP server on localhost. At launch, `tower run` injects hook configuration so Claude Code POSTs `PreToolUse`, `PermissionRequest`, `PostToolUse`, `SessionStart`, `Stop`, and other events to Tower's endpoint. Tower responds synchronously with approve/deny decisions. Claude blocks until the response arrives.

2. **PTY/ConPTY wrapper** (terminal bridging)
   `tower run` owns the pseudoterminal for session identity and process lifecycle. The PTY is passthrough only. If `tower run` exits, the Claude session dies. No detach/reattach in v1.

### Runtime topology

```text
user terminal
    |
    v
tower run claude (foreground bridge)
    | attach + stdin/stdout + resize
    v
Tower daemon <------ HTTP hooks (PreToolUse, PermissionRequest, etc.) ------ Claude Code child
    |                                                                              ^
    +---- spawn + PTY/ConPTY ownership -------------------------------------------|
```

The critical difference from the previous design: approvals flow through HTTP hooks, not through PTY stdin injection. Claude posts a `PermissionRequest` event to Tower's HTTP API and blocks. Tower decides (via cockpit or policy), responds with allow/deny JSON, and Claude acts on it. No lease, no pause, no PTY write.

## 4. End-to-end lifecycle for `tower run claude`

### 4.1 Launch flow

1. User runs `tower run claude [args...]` inside a terminal, from a workspace.
2. `tower run` ensures Tower daemon is reachable. If the daemon is not running, `tower run` starts it.
3. `tower run` registers a new managed session with the daemon:
   - current working directory
   - raw `argv`
   - terminal dimensions
   - terminal metadata if detectable (`TERM_PROGRAM`, `WT_SESSION`, tty name, etc.)
4. Tower daemon allocates:
   - `session_id` immediately, before process spawn
   - `runtime_id` for this process incarnation
5. `tower run` creates the PTY/ConPTY backend and spawns Claude Code with:
   - Tower-injected hook configuration pointing at `http://localhost:<port>/hooks/<session_id>`
   - Tower-injected environment variables (`TOWER_MANAGED=1`, `TOWER_SESSION_ID`, etc.)
   - Claude's own args passed through
6. Tower daemon emits `session.started`.
7. The bridge attaches its terminal streams to the PTY and enters passthrough mode.

### 4.2 Hook injection

Tower injects hooks by writing `.claude/settings.local.json` in the project directory before spawning Claude. This file is project-local, gitignored by Claude Code's defaults, and does not interfere with user-level or project-level hooks in other settings files. The injected config registers HTTP hooks for:

| Hook event | Purpose |
|---|---|
| `PreToolUse` | Observe every tool call. Classify risk. Apply batch policy for read-only ops. |
| `PermissionRequest` | **The approval channel.** Tower responds allow/deny. This is where cockpit approvals happen. |
| `PostToolUse` | Audit trail. Record what tools ran and their results. |
| `PostToolUseFailure` | Audit trail for failed tool calls. |
| `SessionStart` | Confirm session is alive and register metadata. |
| `Stop` | Know when Claude finishes a response turn. |
| `SubagentStart` / `SubagentStop` | Track subagent activity for the cockpit. |
| `Notification` | Catch `permission_prompt` and `idle_prompt` for attention ranking. |
| `SessionEnd` | Clean lifecycle tracking. |

Example injected hook config (abbreviated, showing two representative hooks):

```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "hooks": [
          {
            "type": "http",
            "url": "http://localhost:7832/hooks/01JSESSION/permission-request",
            "timeout": 600,
            "headers": {
              "Authorization": "Bearer $TOWER_HOOK_TOKEN"
            },
            "allowedEnvVars": ["TOWER_HOOK_TOKEN"]
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "http",
            "url": "http://localhost:7832/hooks/01JSESSION/post-tool-use",
            "timeout": 30,
            "async": true,
            "headers": {
              "Authorization": "Bearer $TOWER_HOOK_TOKEN"
            },
            "allowedEnvVars": ["TOWER_HOOK_TOKEN"]
          }
        ]
      }
    ]
  }
}
```

All hook entries follow the same pattern. Full list of registered events: `PreToolUse`, `PermissionRequest`, `PostToolUse`, `PostToolUseFailure`, `SessionStart`, `SessionEnd`, `Stop`, `Notification`, `SubagentStart`, `SubagentStop`.

Design notes:
- **Authentication**: Every hook carries a `Bearer $TOWER_HOOK_TOKEN` header. The token is a per-session random secret generated at launch and injected via environment variable. The daemon validates the token on every request and rejects requests with missing or invalid tokens.
- `PermissionRequest` and `PreToolUse` are synchronous (no `async` flag) because Claude blocks on them and Tower needs to respond with a decision.
- `PostToolUse`, `SessionStart`, `SessionEnd`, `Stop`, and `Notification` are async because they're observational, Tower doesn't need to block Claude.
- The timeout on `PermissionRequest` is high (600s) because a human may need to review the action in the cockpit.
- The session ID is embedded in the URL path so Tower can route events to the right session without parsing the body first.

### 4.2.1 Hook merge behavior (release blocker)

Claude Code runs all matching hooks in parallel for a given event. If the user or project has existing `PermissionRequest` or `PreToolUse` hooks, those hooks run alongside Tower's hooks.

Claude's documented merge behavior must be validated in the prototype:
- Does Claude wait for ALL hooks to respond before acting?
- If one hook returns `allow` and another returns `deny`, which wins?
- Can an early `allow` from a non-Tower hook cause Claude to proceed before Tower responds?

**If Tower cannot prove safe merge behavior**, managed mode must either:
- require exclusive ownership of `PermissionRequest` hooks (Tower checks for conflicting hooks at launch and refuses managed mode if found), or
- downgrade the session to `managed_visibility_only` (hooks provide observation but Tower does not claim approval control)

This is a release-blocking validation, not a nice-to-have.

### 4.2.2 Launch preconditions

`tower run` must verify these conditions before spawning Claude. If any fail, launch aborts with a clear error.

1. **Daemon reachable**: Tower daemon HTTP server is listening and responds to a health check.
2. **Port and token stable**: Daemon port and auth token are persisted in a lockfile (`~/.tower/daemon.lock`). If the daemon restarts, it must reclaim the same port and token. If it can't (port conflict), existing managed sessions are marked `managed_degraded`.
3. **Hook config written**: `.claude/settings.local.json` was written successfully with Tower's hook entries.
4. **No conflicting hooks**: Check for existing `PermissionRequest` hooks in user/project settings. If found, warn and either require user confirmation or downgrade to `managed_visibility_only`.

After Claude starts, the `SessionStart` hook provides the `permission_mode` field. If the mode is `dontAsk` or `bypassPermissions`, Tower downgrades the session:
- `managed` -> `managed_visibility_only` (hooks provide observation, Tower does not claim approval control)
- The cockpit shows the session but the approval inbox is disabled for it

### 4.3 Normal focused work

- terminal input is forwarded directly through the PTY to Claude
- PTY output is shown in the terminal normally
- Tower receives async hook events (`PostToolUse`, `Stop`, etc.) and updates materialized state
- pre-approved tool calls (read-only batch policy) flow through `PreToolUse` without cockpit involvement

### 4.4 Approval from the cockpit

This is the core managed workflow. No PTY writes, no leases.

1. Claude decides to call a tool that requires permission (e.g., `Bash "rm -rf node_modules"`).
2. Claude Code fires `PermissionRequest` hook, POSTing to Tower:
   ```json
   {
     "session_id": "abc123",
     "hook_event_name": "PermissionRequest",
     "tool_name": "Bash",
     "tool_input": { "command": "rm -rf node_modules" },
     "cwd": "/home/user/project",
     "permission_mode": "default"
   }
   ```
3. Claude is now **blocked**, waiting for Tower's HTTP response.
4. Tower daemon:
   - records the pending approval with a unique `action_id`
   - classifies risk from `tool_name` and `tool_input`
   - checks batch policy: if read-only and policy allows, auto-approve immediately
   - otherwise, emits `approval.requested` to the cockpit and waits for human decision
5. Human selects approve or deny in the cockpit.
6. Tower responds to the HTTP request:
   ```json
   {
     "hookSpecificOutput": {
       "hookEventName": "PermissionRequest",
       "decision": {
         "behavior": "allow"
       }
     }
   }
   ```
   Or for deny:
   ```json
   {
     "hookSpecificOutput": {
       "hookEventName": "PermissionRequest",
       "decision": {
         "behavior": "deny",
         "message": "Destructive command blocked by Tower policy"
       }
     }
   }
   ```
7. Claude Code receives the response and proceeds (or shows the denial).
8. Tower emits `approval.resolved` and updates session state.

### Sequence diagram: hook-based approval

```text
Claude Code          Tower daemon HTTP        Tower cockpit
    |                      |                      |
1.  | POST /hooks/.../permission-request -------->|                      |
    | (blocked, waiting for response)             |                      |
    |                      | emit approval.requested ------------------>|
    |                      |                      | human reviews        |
    |                      |<--------------------- approve              |
    | <--- 200 OK {behavior: "allow"} ------------|                      |
    | (proceeds with tool call)                   |                      |
    |                      | emit approval.resolved                     |
```

No lease. No pause. No PTY injection. Claude's own hook protocol handles the blocking and response.

### 4.4.0 Pending approval concurrency

Claude is single-threaded per session: it blocks on `PermissionRequest` and doesn't fire another until the current one resolves. So at most **one pending approval per session** at a time.

However, Tower manages multiple sessions. The daemon maintains a per-session pending approval slot. Across sessions, multiple approvals can be pending simultaneously, one per session.

If the operator never answers: the `PermissionRequest` hook times out (600s default). Claude Code then shows the permission dialog in the terminal. Tower emits `approval.resolved(resolution=expired, resolved_via=hook_timeout)`.

If the cockpit disconnects but the daemon is still running: the daemon holds the pending HTTP request open. The approval stays pending until the cockpit reconnects or the hook times out.

### 4.4.1 V1 correlation boundary

In v1, `PermissionRequest` and `PreToolUse` are treated as **independent event streams** with different roles:

- **Manual approval** uses `PermissionRequest` only. Each `PermissionRequest` gets its own `action_id`. Tower does not attempt to correlate it with a prior `PreToolUse`.
- **Batch auto-approve** uses `PreToolUse` only. If `PreToolUse` returns `permissionDecision: "allow"`, Claude skips `PermissionRequest` entirely.

V1 does **not** claim a proven correlation model between the two events. Correlation (matching a `PermissionRequest` to the `PreToolUse` that preceded it) is a v2 optimization that can use temporal proximity and `tool_name` matching, but it is not required for the basic approve/deny path.

### 4.5 Batch approval via PreToolUse

For read-only operations, Tower can auto-approve without waiting for the human:

1. Claude fires `PreToolUse` for a tool call (fires before `PermissionRequest`).
2. Tower checks: is this `Read`, `Glob`, `Grep`, or `Bash "git status"`?
3. If risk class is `read_only` and batch policy allows, Tower responds:
   ```json
   {
     "hookSpecificOutput": {
       "hookEventName": "PreToolUse",
       "permissionDecision": "allow",
       "permissionDecisionReason": "Auto-approved by Tower batch policy (read-only)"
     }
   }
   ```
4. Claude skips the permission dialog entirely and runs the tool.

This means read-only operations never show up in the cockpit approval queue. They flow through silently and are logged in the audit trail via `PostToolUse`.

### 4.6 Process exit

- when Claude exits normally, the `SessionEnd` hook fires and Tower emits `session.ended`
- when Claude crashes, the PTY closes and Tower detects the child process exit
- the session remains in Tower history with durable audit records

### 4.7 Recovery / reconnect

**V1 model**: `tower run` owns the PTY. If `tower run` exits, Claude dies. There is no detach/reattach and no separate runtime helper process.

**Daemon restart**: If Tower daemon restarts while Claude is still alive:
1. In-flight hook requests fail (connection refused). Claude treats these as non-blocking errors and continues, showing permission dialogs in the terminal if needed.
2. The daemon must restart on the **same port** with the **same auth token** (both persisted in `~/.tower/daemon.lock`). Hook config is snapshotted at Claude startup and won't change.
3. When the daemon comes back on the same port, hooks from the running Claude session start arriving again.
4. During the gap, Tower marks the session as `managed_degraded`. Any approvals granted locally during the gap are unverified from Tower's perspective.
5. When hooks resume, Tower transitions back to `managed` but records the gap in the audit trail.

**If the daemon can't reclaim its port**: The session stays `managed_degraded` for its remaining lifetime. Tower does not claim strong managed semantics. The user must launch a new managed session for full approval control.

### 4.8 Park / resume boundary

Park and resume are v2 features. The event payload definitions are included in section 8 for forward compatibility but won't be implemented in the first slice.

### Sequence diagram: launch and attach

```text
User terminal      tower run         Tower daemon        Claude Code
     |                |                  |                    |
1. run command ------>|                  |                    |
     |                | register ------->|                    |
     |                | create PTY       |                    |
     |                | inject hooks     |                    |
     |                | exec claude -----|-------- spawn ---->|
     |                |                  |<-- SessionStart ---|
     | attach streams ==================>|                    |
     |<=============== PTY output ========================== |
```

## 5. PTY / ConPTY wrapper model

The managed adapter owns the PTY for terminal bridging, session identity, and process lifecycle. In v1, the PTY is **passthrough only**. Tower does not write to the PTY stdin for approvals, that's handled by hooks.

### 5.1 Platform backend split

| Concern | Windows | macOS | Design rule |
|---|---|---|---|
| Pseudoterminal primitive | ConPTY | PTY (`openpty`/equivalent) | Keep a common `PTYBackend` interface with platform-specific implementations. |
| Child attachment | `CreatePseudoConsole` + `STARTUPINFOEX` pseudoconsole attribute | Spawn child on PTY slave as session leader | Child stdio always terminates at the PTY backend. |
| Local IPC | Named pipes | Unix domain sockets | The control protocol should be transport-agnostic. |
| Resize | `ResizePseudoConsole` | `TIOCSWINSZ` | Resize is driven by the terminal bridge. |
| Process cleanup | Job Object for child tree | Process group for child tree | Cleanup semantics belong to the PTY owner, not the bridge. |
| Encoding / ANSI handling | ConPTY VT stream | Native PTY stream | Treat the PTY output as byte/VT stream. |

### 5.2 Ownership model

| Resource | Owner | Notes |
|---|---|---|
| `session_id`, runtime registry, audit log | Tower daemon | Durable local state |
| PTY/ConPTY master + control handles | `tower run` process | Authoritative for terminal bridging |
| Claude stdin/stdout/stderr | Claude child via PTY slave / pseudoconsole | Never directly attached to Tower daemon |
| User terminal stdin/stdout while attached | Terminal bridge | Default owner of focused interaction |
| Approval decisions | Tower daemon via HTTP hook responses | No PTY writes for approvals |

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

Tower-injected environment variables:

- `TOWER_MANAGED=1`
- `TOWER_SESSION_ID=<session id>`
- `TOWER_RUNTIME_ID=<runtime id>`
- `TOWER_DAEMON_PORT=<port>`
- `TOWER_HOOK_TOKEN=<per-session random secret>` (referenced in hook config headers via `allowedEnvVars`)

### 5.4 Passthrough mode

Passthrough mode is the only PTY mode in v1.

- terminal input goes to the bridge
- the bridge forwards bytes and resize events to the PTY
- PTY output is mirrored back to the bridge
- Tower observes via hook events, not by reading PTY output

## 6. Hook-based approval and observation

Hooks are the primary integration surface. Claude Code's hook system provides everything Tower needs for v1: structured tool call data, synchronous blocking for approval decisions, and async notifications for observability.

### 6.1 How hooks replace PTY parsing

The previous design assumed Tower would need to:
1. Parse PTY output to detect approval prompts
2. Confirm the prompt is "live" via screen state
3. Inject responses by writing to PTY stdin
4. Manage freshness via terminal input epochs and prompt fingerprints

Hooks eliminate all four:

| Old requirement | Hook solution |
|---|---|
| Detect approval prompt | `PermissionRequest` hook fires when Claude shows a permission dialog |
| Confirm prompt is live/blocking | Claude blocks on the HTTP response. If Tower got the POST, Claude is waiting. |
| Inject approval response | Tower's HTTP response IS the approval. `{behavior: "allow"}` or `{behavior: "deny"}`. |
| Freshness / race conditions | No race. Claude sent the request, Claude is blocked, Tower responds. One request, one response. |
| Risk classification from prompt text | `PreToolUse` provides `tool_name` and full `tool_input` as structured JSON. No text parsing. |
| Batch approval for read-only ops | `PreToolUse` response with `permissionDecision: "allow"` skips the permission dialog entirely. |

### 6.2 `normalized_key` format

`normalized_key` supports safe grouping for read-only batch approvals. It must be deterministic across platforms.

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

### 6.3 Risk classification

Tower classifies risk from structured hook data, not from prompt text:

| tool_name | tool_input pattern | Risk class |
|---|---|---|
| `Read` | any | `read_only` |
| `Glob` | any | `read_only` |
| `Grep` | any | `read_only` |
| `Bash` | `git status`, `git log`, `git diff`, `ls`, `which` | `read_only` |
| `Bash` | `git add`, `git commit`, `git push` | `git_mutation` |
| `Bash` | `npm install`, `pip install` | `package_install` |
| `Edit` | any | `workspace_write` |
| `Write` | any | `workspace_write` |
| `Bash` | `rm`, `mv`, other write commands | `workspace_write` |
| `WebFetch` / `WebSearch` | any | `network_read` |
| `Bash` | `curl`, `wget`, network commands | `network_read` or `network_write` |
| unknown | any | `unknown` |

This classification is deterministic from the `tool_name` and `tool_input.command` fields that hooks provide. No prompt parsing needed.

### 6.4 Tower daemon HTTP API

The daemon exposes these hook endpoints per session:

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

All endpoints accept the JSON body that Claude Code sends (documented in the hooks reference). Synchronous endpoints (`pre-tool-use`, `permission-request`) return a decision JSON body. Async endpoints return 200 with empty body.

If the daemon receives a POST for an unknown session ID, it returns 200 empty (non-blocking) rather than an error. This handles the case where Tower restarted and hasn't re-registered the session yet.

## 7. Session discovery boundaries

The managed Claude adapter and the observed discovery system must not blur together.

### 7.1 Managed adapter responsibilities

The managed Claude adapter is responsible for:

- launching Claude via `tower run claude`
- minting and persisting `session_id` / `runtime_id`
- owning the PTY/ConPTY and child process tree
- injecting hook configuration at launch
- receiving and processing hook events via HTTP
- approving/denying tool calls via hook responses
- validating launch preconditions and permission mode compatibility
- emitting high-confidence events for managed sessions

### 7.2 Observed discovery responsibilities

Observed discovery is responsible for:

- scanning for arbitrary already-running Claude processes
- best-effort process correlation and session descriptors
- low-confidence continuity across rediscovery
- liveness, recent activity, deep-linking, and other observe-only surfaces

Observed discovery is **not** responsible for:

- claiming ownership of Tower-managed runtime helpers
- upgrading arbitrary Claude processes into managed sessions
- performing remote approvals

### 7.3 De-duplication rule

If observed discovery sees a Claude process that matches a registered managed runtime:

- the managed record remains authoritative
- observed evidence may enrich liveness or process fingerprint fields
- no duplicate observed session should be shown

### 7.4 Recovery boundary

On Tower daemon startup:

1. managed adapter checks the registry for sessions that were launched before the restart
2. for each registered session where hooks resume (same port/token), transition from `managed_degraded` to `managed`
3. for registered sessions where hooks don't resume, mark as `managed_degraded` or `ended`
4. observed discovery separately scans the machine for arbitrary Claude sessions
5. only the managed path restores approval control

## 8. Claude-managed event payloads

The normalized event envelope from `docs\engineering\foundation-spec.md` stays unchanged. This section defines the `payload` shape expectations for Claude-managed sessions.

### 8.1 Event kinds covered here

- `session.discovered`
- `session.started`
- `session.reconnected`
- `session.parked`
- `session.resumed`
- `session.ended`
- `state.changed`
- `approval.requested`
- `approval.resolved`
- `error.reported`

Note: `command.sent` and `command.applied` from the previous design are removed. Those existed for operator text injection, which is deferred past v1.

### 8.2 Payload requirements by event

#### `session.discovered` (managed recovery only)

Used when Tower daemon rediscovers a process from its own managed runtime registry, not when it launches a new one.

Required fields:

- `discovery_source` (`managed_registry`)
- `session_id`
- `runtime_id`
- `pid`
- `workspace_root`
- `platform_backend` (`windows_conpty` or `darwin_pty`)
- `tower_run_pid` (int)

Optional fields:

- `adapter_ref`

#### `session.started`

Required fields:

- `launch_kind` (`tower_run`)
- `workspace_root`
- `argv`
- `resolved_executable`
- `platform_backend`
- `pid`
- `adapter_ref`
- `hook_endpoint` (the URL Claude is posting to)
- `tower_run_pid` (int)

Optional fields:

- `transport_wrapper`
- `repo_root`

#### `session.reconnected`

Used when hooks resume after a daemon restart. Not used for terminal reattach (no detach/reattach in v1).

Required fields:

- `previous_runtime_id`
- `current_runtime_id`
- `reason` (`hook_resume`)
- `pid`
- `gap_duration_ms` (time between last hook before outage and first hook after)

#### `session.parked`

Required fields:

- `park_id`
- `reason`
- `artifact_path`
- `pid`
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
- `pid`

Optional fields:

- `adapter_ref`

#### `session.ended`

Required fields:

- `reason` (`exit_0` \| `exit_nonzero` \| `interrupted` \| `launch_failure`)
- `ended_at`
- `pid`
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

Recommended `reason` values for this adapter:

- `launch_complete`
- `approval_detected`
- `approval_cleared`
- `hook_channel_lost`
- `hook_channel_resumed`
- `permission_mode_incompatible`
- `process_exited`

#### `approval.requested`

Required fields:

- `action_id`
- `approval_kind` (`tool_call`; only allowed v1 value)
- `tool_name`
- `risk_class`
- `decision_options` (array; v1 expected values are `approve`, `deny`)
- `tool_input_summary` (structured excerpt per data minimization rules in section 10.1: commands in full, file paths in full, file contents as SHA-256 hash)

Optional fields:

- `normalized_key`
- `cwd`
- `repo_root`
- `display_title`
- `display_subtitle`

#### `approval.resolved`

Required fields:

- `action_id`
- `resolution` (`approved` \| `denied` \| `auto_approved` \| `expired` \| `cancelled`)
- `resolved_via` (`cockpit` \| `batch_policy` \| `process_exit` \| `hook_timeout`)
- `resolved_at`

Optional fields:

- `operator`
- `latency_ms`
- `error_code`

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

### 8.3 Example payload: `approval.requested`

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
  "occurred_at": "2026-03-08T18:00:00Z",
  "ingested_at": "2026-03-08T18:00:00Z",
  "confidence": "certain",
  "payload": {
    "action_id": "01JACTION",
    "approval_kind": "tool_call",
    "tool_name": "Bash",
    "tool_input_summary": {
      "command": "git status",
      "cwd": "C:\\github\\tower"
    },
    "risk_class": "read_only",
    "decision_options": ["approve", "deny"],
    "normalized_key": "read_only:bash:git-status:91ab3e8d4c7f...",
    "display_title": "Bash: git status",
    "display_subtitle": "C:\\github\\tower"
  }
}
```

## 9. Failure modes and recovery behavior

| Failure mode | Expected behavior |
|---|---|
| Claude executable cannot be resolved | Emit `error.reported(code=claude_exec_missing, recoverable=false)` and fail launch before `session.started`. |
| PTY/ConPTY creation fails | Emit `error.reported(code=pty_create_failed, recoverable=false)` and fail launch. |
| Hook injection fails | Fail launch. Managed session cannot exist without hooks, that's the entire approval channel. |
| Tower daemon HTTP server unreachable | Claude treats hook failures as non-blocking errors (documented behavior). Exact fallback behavior must be validated in prototype, likely shows permission dialog in terminal. Session transitions to `managed_degraded`. |
| Tower daemon restarts mid-session | In-flight hook requests fail. Daemon must restart on same port/token (persisted in lockfile). When it does, hooks resume. During the gap, session is `managed_degraded` with an audit gap. |
| Daemon can't reclaim its port | Session stays `managed_degraded` for its remaining lifetime. No strong managed semantics. User must launch a new managed session. |
| Hook request times out (600s) | Claude falls back to showing permission dialog in terminal (must validate). Tower emits `approval.resolved(resolution=expired, resolved_via=hook_timeout)`. |
| `tower run` exits unexpectedly | Claude session dies (PTY closes). Tower emits `session.ended`. No detach/reattach in v1. |
| Claude exits while approval is pending | HTTP connection drops. Tower emits `approval.resolved(resolution=cancelled, resolved_via=process_exit)` followed by `session.ended`. |
| Unknown session ID in hook POST | Tower returns 200 empty (non-blocking) but logs `error.reported(code=unknown_session_hook)`. Repeated occurrences for the same ID escalate to a security warning in the cockpit. |
| Permission mode is `dontAsk` or `bypassPermissions` | `PermissionRequest` never fires. Tower downgrades to `managed_visibility_only` after detecting this in `SessionStart`. |
| Conflicting `PermissionRequest` hooks exist | Another hook may return a decision before Tower. Behavior depends on Claude's merge semantics (release blocker to validate). |

### Recovery principle

Recovery should only restore **truth the adapter can prove**:

- if hooks are posting and the PTY is alive, the session is `managed`
- if hooks stopped arriving, the session is `managed_degraded` (audit gap, not just visibility gap)
- if the PTY process is gone, the session is `ended`
- do not guess a strong managed state from process scans alone

### Managed session states

| State | Meaning | Approval control |
|---|---|---|
| `managed` | Hooks active, PTY alive, permission mode compatible | Full: cockpit approve/deny via hooks |
| `managed_degraded` | Hooks interrupted (daemon restart, network issue) | None: approvals during gap are unverified. Transitions back to `managed` when hooks resume. |
| `managed_visibility_only` | Hooks active but permission mode is `dontAsk`/`bypassPermissions`, or hook merge conflict detected | Observation only: Tower sees tool calls via `PreToolUse`/`PostToolUse` but cannot claim approval control |
| `ended` | Claude process exited or `tower run` exited | None |

### Graceful degradation

Claude Code treats hook failures as non-blocking errors (documented behavior). If Tower goes down:
- **read path**: Tower loses visibility (no hook events arriving)
- **write path**: Claude likely shows permission dialogs in the terminal (must validate in prototype)
- **audit**: approvals granted locally during the gap are not recorded by Tower. This is an **audit gap**, not just a temporary visibility gap. The gap is recorded in the audit trail when hooks resume.
- **when Tower comes back**: if daemon reclaims same port/token, hooks resume and session transitions from `managed_degraded` back to `managed`

## 10. Security and trust constraints

- Tower's HTTP hook endpoint binds to `127.0.0.1` only, never `0.0.0.0` or a network interface.
- **Per-session bearer token**: Every hook request carries an `Authorization: Bearer <token>` header. The token is a random secret generated per session, injected as `TOWER_HOOK_TOKEN` env var, and referenced in the hook config via `allowedEnvVars`. The daemon validates the token on every request and rejects missing/invalid tokens with 401.
- **Spoofing mitigation**: Localhost-only + bearer token means an attacker needs same-user access to read the token from the process environment or settings file. At that point the attacker already owns the user's session.
- **Browser-based attacks**: Browsers cannot set arbitrary `Authorization` headers in cross-origin requests, so malicious web pages targeting localhost are blocked.
- Every approval decision is audited before the HTTP response is sent.
- Batch approval is allowed only when the foundation policy is satisfied; otherwise fall back to individual review.
- The adapter must not silently rewrite tracked repo files to install hooks. Hook injection uses `.claude/settings.local.json` (gitignored) only.

### 10.1 Data minimization

Hook payloads may contain sensitive data (file contents, secrets, destructive commands). Tower applies these persistence rules:

| Data | Persistence rule |
|---|---|
| `tool_name`, `risk_class`, `cwd`, `action_id` | Stored in full |
| `tool_input.command` (Bash) | Stored in full (audit value: what was approved to run) |
| `tool_input.file_path` (Read, Write, Edit) | Stored in full (path, not content) |
| `tool_input.content` (Write) | SHA-256 hash only, content not persisted |
| `tool_input.old_string` / `new_string` (Edit) | SHA-256 hash only, diffs not persisted |
| `tool_input.pattern` (Grep, Glob) | Stored in full |
| Full hook JSON body | Held in memory for the pending approval. Discarded after resolution. Only structured audit fields persist to SQLite. |
| Environment snapshots | Never persisted |

## 11. Go-oriented runtime boundary

The foundation spec already defines the generic adapter contract. The managed Claude adapter needs a sharper runtime boundary underneath that contract.

### 11.1 Core-facing adapter surface

```go
package claude

import "context"

type Adapter interface {
    DiscoverManaged(ctx context.Context) ([]ManagedDescriptor, error)
    Subscribe(ctx context.Context) (<-chan Event, error)
    Snapshot(ctx context.Context, sessionID string) (ManagedSnapshot, error)
    Capabilities(ctx context.Context, sessionID string) (CapabilitySet, error)
    Approve(ctx context.Context, sessionID string, req ApprovalDecision) error
    Launch(ctx context.Context, req LaunchRequest) (LaunchResult, error)
}
```

### 11.2 Hook handler boundary

```go
// HookHandler processes incoming hook events from Claude Code sessions.
// Each method corresponds to a Claude Code hook event type.
type HookHandler interface {
    HandlePreToolUse(ctx context.Context, sessionID string, event PreToolUseEvent) (PreToolUseResponse, error)
    HandlePermissionRequest(ctx context.Context, sessionID string, event PermissionRequestEvent) (PermissionResponse, error)
    HandlePostToolUse(ctx context.Context, sessionID string, event PostToolUseEvent) error
    HandleSessionStart(ctx context.Context, sessionID string, event SessionStartEvent) error
    HandleSessionEnd(ctx context.Context, sessionID string, event SessionEndEvent) error
    HandleStop(ctx context.Context, sessionID string, event StopEvent) error
    HandleNotification(ctx context.Context, sessionID string, event NotificationEvent) error
}
```

### 11.3 PTY backend boundary

```go
type PTYBackend interface {
    Start(ctx context.Context, spec SpawnSpec) (ChildProcess, error)
    ReadOutput(ctx context.Context) ([]byte, error)
    Resize(cols, rows uint16) error
    Close() error
}
```

Note: `WriteInput` is removed from the interface. In v1, the only stdin path is from the terminal bridge. Tower doesn't write to the PTY.

### 11.4 Key data types

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
    PID             int
    PlatformBackend string
    HookEndpoint    string
    HookToken       string
}

type ApprovalDecision struct {
    ActionID string
    Decision string // "approve" | "deny"
    Message  string // reason shown to Claude on deny
}

type PreToolUseEvent struct {
    ToolName  string
    ToolInput map[string]any
    ToolUseID string
    CWD       string
    SessionID string
}

type PreToolUseResponse struct {
    PermissionDecision       string // "allow" | "deny" | "ask" | "" (pass through)
    PermissionDecisionReason string
}

type PermissionRequestEvent struct {
    ToolName  string
    ToolInput map[string]any
    CWD       string
    SessionID string
}

type PermissionResponse struct {
    Behavior string // "allow" | "deny"
    Message  string // for deny: shown to Claude
}
```

### 11.5 Implementation note

The daemon is the only process allowed to:

- decide whether an approval is granted
- apply batch policy against the risk classification
- respond to hook HTTP requests
- validate hook auth tokens

`tower run` is responsible for:

- process lifecycle (spawn, cleanup). If `tower run` exits, Claude dies.
- terminal bridging (stdin/stdout/resize forwarding)
- session registration with the daemon
- writing `.claude/settings.local.json` with hook config before spawning Claude

## 12. Open risks and validation plan

### 12.1 Open risks

| Risk | Why it matters | Severity |
|---|---|---|
| **Hook merge semantics unknown.** Claude runs matching hooks in parallel. If another hook can `allow` before Tower responds, Tower's approval control is bypassed. | Release blocker. Tower cannot claim managed approval control without proving safe merge behavior. | Critical |
| **`.claude/settings.local.json` gitignore status unknown.** If not gitignored by default, Tower's hook injection could leak into version control. | Launch precondition. Easy to validate but must be done. | High |
| **`PermissionRequest` timeout fallback behavior unknown.** When the hook times out, does Claude show a terminal prompt, skip the tool, or something else? | Affects degradation story and user experience. | High |
| **Hook injection merge with existing hooks unknown.** Does `.claude/settings.local.json` merge with user/project hooks or replace them? | Affects whether Tower breaks existing user workflows. | High |
| **Permission mode detection timing.** `SessionStart` provides `permission_mode`, but the mode might change mid-session. | Tower could claim managed approval control that was valid at start but not later. Likely low risk since mode changes are rare. | Medium |
| HTTP hook reliability on localhost. | Connection refused, port conflicts, firewall rules on Windows. | Medium |
| Windows launch resolution may involve wrappers like `cc.cmd`. | Affects identity, process cleanup, and exact child-tree semantics. | Medium |
| Multiple Tower instances on same machine. | Port conflicts. Strategy: fixed port with lockfile in `~/.tower/daemon.lock`. | Medium |

### 12.2 Prototype checkpoints

1. **Hook endpoint + injection proof** (combined, they depend on each other)
   - start a Go HTTP server on localhost with auth token validation
   - write `.claude/settings.local.json` with Tower hook config
   - spawn Claude Code
   - verify: PreToolUse, PermissionRequest, PostToolUse events arrive with correct JSON and valid auth token
   - verify: PermissionRequest response with `{behavior: "allow"}` causes Claude to proceed without showing the terminal prompt
   - verify: PermissionRequest response with `{behavior: "deny"}` causes Claude to show denial
   - verify: `.claude/settings.local.json` is gitignored by default
   - verify: existing user/project hooks still work alongside Tower's hooks
   - **Release blocker**: verify hook merge semantics. What happens when multiple hooks return decisions for the same event? Does deny-wins? Does Claude wait for ALL hooks?

2. **Permission mode proof**
   - verify `SessionStart` hook provides `permission_mode` field
   - verify `PermissionRequest` fires in `default` mode
   - verify `PermissionRequest` does NOT fire in `dontAsk` and `bypassPermissions` modes
   - verify Tower can detect the mode and downgrade accordingly

3. **Timeout and failure behavior proof**
   - verify: what does Claude show when a `PermissionRequest` hook times out?
   - verify: what does Claude show when the hook endpoint is unreachable (connection refused)?
   - verify: what does Claude show on non-2xx response?
   - document the exact observed behavior for each case

4. **Minimal `tower run` PTY wrapper**
   - spawn Claude through a PTY/ConPTY owned by `tower run`
   - passthrough stdin/stdout/resize
   - verify child-tree cleanup when `tower run` exits
   - Windows ConPTY and macOS PTY parity

5. **Single-session manual approve/deny path**
   - one real `PermissionRequest` from Claude
   - one cockpit decision (approve or deny)
   - one observed Claude outcome (tool runs or denial shown)
   - one durable audit entry in SQLite

6. **Daemon restart degradation proof**
   - kill Tower daemon while Claude is running
   - verify Claude continues (with terminal prompts during gap)
   - restart daemon on same port/token
   - verify hooks resume
   - verify `managed_degraded` -> `managed` transition
   - verify audit trail records the gap

7. **Batch auto-approve via PreToolUse** (only after steps 1-6 pass)
   - PreToolUse returns `permissionDecision: "allow"` for read-only ops
   - verify Claude skips the permission dialog entirely
   - verify non-read-only ops still go through PermissionRequest

## 13. Immediate implementation order

Recommended build order, informed by GPT 5.4 review:

1. **Hook endpoint + injection + auth + merge semantics proof**
   Prove the entire hook path works: daemon HTTP server, `.claude/settings.local.json` injection, auth token, and hook merge behavior. This is the release-blocker checkpoint. If merge semantics are unsafe, stop and redesign before building anything else.

2. **Permission mode detection + downgrade logic**
   Prove that Tower can detect `dontAsk`/`bypassPermissions` via `SessionStart` and downgrade to `managed_visibility_only`.

3. **Minimal `tower run` PTY wrapper**
   Spawn Claude through a Tower-owned PTY. Passthrough only. Verify child cleanup.

4. **Single-session manual approve/deny + SQLite audit**
   One `PermissionRequest`, one cockpit decision, one Claude outcome, one durable audit record. Audit persistence comes immediately after the first working approval loop, not later.

5. **Risk classification from PreToolUse data**
   Classify `tool_name` + `tool_input` into risk classes. No policy enforcement yet.

6. **Daemon restart degradation + `managed_degraded` state**
   Prove the daemon can restart on the same port/token. Prove `managed_degraded` -> `managed` transition. Prove the audit gap is recorded.

7. **Batch auto-approve for read-only ops via PreToolUse**
   Optimization after the manual path and audit are solid.

8. **Managed runtime registry for multi-session support**
   Multiple concurrent managed sessions with independent approval queues.

This order proves the riskiest assumptions first (hook behavior, merge semantics, permission mode) and adds audit persistence immediately after the first working approval loop.
