# GPT Review: Hook-First Claude Managed Adapter Design

Reviewed against:

- `docs\engineering\claude-managed-adapter-design.md`
- `docs\engineering\architecture-decisions.md`
- `docs\engineering\foundation-spec.md`

Reviewer stance:

- focus on what needs to change
- push on fragile assumptions
- do not spend time re-validating what already looks good

## Bottom line

The hook-first rewrite is directionally cleaner than the previous PTY-interception design, but it still assumes more control over Claude's permission system, restart behavior, and hook injection path than the current evidence proves.

The biggest pushback is this:

**"Hooks exist" is not yet the same thing as "Tower owns approvals."**

That only becomes true if Tower can prove:

- `PermissionRequest` reliably fires for managed sessions
- hook injection is stable and compatible with real Claude settings behavior
- startup-time hook snapshots do not break the runtime model
- Tower can correlate `PreToolUse` and `PermissionRequest` safely
- loss of the daemon or hook channel degrades honestly rather than pretending control is intact

> **Claude response:** Agree with the framing. All five items are prototype checkpoints, not design assumptions. The doc should label them as "must validate" not "confirmed." The implementation order already puts hook endpoint proof first, but the assumptions table in section 0 should be more honest about what's unproven.

## 1. Architectural gaps

### 1.1 PTY ownership is contradictory

The document says different things in different places:

- topology implies the daemon owns the durable runtime
- `4.1`, `5.2`, and `11.5` make `tower run` the PTY owner
- `4.7` says the daemon reconnects to a PTY helper process that is no longer clearly part of the design

That is not a small wording issue. It affects:

- detach / reattach
- restart recovery
- child cleanup
- session identity durability

The design needs one explicit answer:

- either `tower run` is just a bridge and a separate durable runtime owns the PTY
- or `tower run` is the owner and detach / daemon-restart recovery must be described much more conservatively

Right now it mixes both.

> **Claude response:** Accept. This is a real contradiction. The previous design had a separate "runtime helper" process that outlived `tower run`. The rewrite collapsed that but didn't clean up all the references. The v1 answer: **`tower run` owns the PTY. Period.** Detach/reattach is deferred to v2. If `tower run` exits, the Claude session dies. Daemon restart recovery means the daemon rebuilds state from hook events that resume arriving, not from reconnecting to a helper process. Will fix the doc.

### 1.2 Recovery is overstated

The doc currently frames daemon restarts and hook outages as graceful degradation, but it underplays what is lost:

- approval provenance during the outage
- visibility into tool activity during the outage
- certainty about whether pending approvals were answered locally

That means this is not just a temporary visibility gap. It is an **audit gap** and a **trust gap**.

The state model should make that explicit with a degraded managed state such as:

- `managed`
- `managed_degraded`
- `managed_unverified`

Or at least an explicit `hook_channel_lost` condition that materially lowers confidence.

> **Claude response:** Accept. Daemon outage is an audit gap, not just a visibility gap. Will add `managed_degraded` state that triggers when hooks stop arriving but the PTY is still alive. The state model should lower confidence to reflect what Tower actually knows during the gap.

### 1.3 Approval correlation is underspecified

`PreToolUse` includes `tool_use_id`.
`PermissionRequest` does not.

That creates ambiguity when:

- multiple similar tool calls happen close together
- subagents trigger tool calls
- the same command appears repeatedly
- retries happen after timeouts or restarts

The design currently jumps from hook receipt to `action_id` creation without a robust correlation story.

You need an explicit rule for how Tower decides:

- whether a `PermissionRequest` corresponds to a known `PreToolUse`
- when it does not
- how duplicate or near-duplicate requests are handled

> **Claude response:** Accept the problem statement, defer the solution. V1 treats each `PermissionRequest` as an independent approval event. Tower assigns a new `action_id` to each one. Correlation with `PreToolUse` is an optimization for batch policy and can use temporal proximity + tool_name matching, but it's not required for the basic approve/deny path. The doc should say this explicitly rather than leaving it ambiguous.

### 1.4 Managed approval is not actually guaranteed yet

The document treats managed Claude sessions as approval-controllable because hooks are injected.

That is not enough.

If the session is running with:

- permissive project/user rules
- `dontAsk`
- equivalent bypass-oriented settings

then `PermissionRequest` may never fire.

That means Tower could launch a session it calls `managed` without actually owning the approval surface it claims to own.

The launch path needs a gate:

- if Tower cannot guarantee hook-mediated approval control for the session, it should not claim full managed approval semantics

> **Claude response:** Good catch. If Claude runs in `dontAsk` or `bypassPermissions` mode, `PermissionRequest` never fires. Tower should gate at launch: detect the permission mode and refuse managed status if it's permissive. `PreToolUse` still fires in all modes (it's pre-execution, not permission-related), so Tower retains visibility, but it can't claim approval control. Will add a launch precondition check.

## 2. Hook assumptions that may be wrong or fragile

### 2.1 Synchronous blocking is only half the behavior

Yes, hooks are synchronous in the sense that Claude waits for the HTTP response.

But the docs also say:

- non-2xx is non-blocking
- connection failure is non-blocking
- timeout is non-blocking

The current design assumes that non-blocking failure means:

- Claude shows the normal terminal prompt
- the user can safely continue there

That exact fallback behavior is not strongly proven in the current doc. It may be true, but it still needs validation.

You should not design around "probably shows the normal prompt."
You should prototype and document the exact observed behavior.

> **Claude response:** Fair. The Claude docs say non-2xx and connection failures are "non-blocking errors" and "execution continues." That's the documented contract. But "execution continues" could mean different things: shows the terminal prompt, skips the tool call, or something else. Prototype checkpoint 1 already covers this ("verify what Claude shows to the user on failure"), but the design doc shouldn't assume a specific fallback behavior until validated. Will soften the language.

### 2.2 Env-based hook injection is treated as more real than the docs prove

The official settings docs strongly support hooks in:

- user settings
- project settings
- local settings
- managed settings
- plugins / skills / agents

The current design still talks about env-based hook injection as a plausible first-class route, but that support is not clearly established in the cited docs.

If env injection is real, the doc should cite the exact mechanism.
If not, stop treating it as equivalent to settings-based injection.

> **Claude response:** Accept. The doc mentions env vars as an injection path but the Claude hooks reference only documents settings files (user, project, local, managed, plugin). Will remove env-based injection as a first-class option and standardize on `.claude/settings.local.json` which is the documented local-only, gitignored settings path. If env-based injection exists, it's a bonus, not the plan.

### 2.3 Hook configuration is snapshotted at startup

The official hook docs say hook config is captured at session startup and used throughout the session.

That has consequences the design does not fully absorb:

- changing the endpoint mid-session will not take effect
- changing tokens mid-session will not take effect
- changing hook policy mid-session will not take effect without restart/review

So any recovery story that depends on the daemon coming back with a different port, token, or config needs to assume the running Claude process will keep using the original hook configuration.

> **Claude response:** This is a strong catch. Direct consequence: Tower daemon must restart on the same port with the same auth token. The port and token must be persisted (lockfile or config) so restarts are deterministic. If the daemon can't reclaim its port, existing managed sessions are orphaned. Will add this as a hard constraint in the doc.

### 2.4 Existing hooks are a real conflict surface

The current design does not say what happens if the user or project already has:

- their own `PreToolUse` hooks
- their own `PermissionRequest` hooks
- plugin hooks that also return decisions

Claude runs matching hooks in parallel.

That means Tower is not the only possible decision-maker unless the hook injection strategy explicitly controls precedence or constrains the configuration environment.

This is a serious fragility point for production.

> **Claude response:** Valid concern. Claude runs all matching hooks in parallel and merges decisions. If a user's existing `PermissionRequest` hook returns `{behavior: "deny"}` and Tower returns `{behavior: "allow"}`, Claude takes the most restrictive decision (deny wins). That's actually safe for Tower, Tower can't override a stricter hook. The risk is the reverse: another hook returns `{behavior: "allow"}` before Tower responds, and Claude proceeds without waiting for Tower. Need to validate: does Claude wait for ALL hooks to respond, or does the first "allow" win? This is a prototype checkpoint.

## 3. Over-engineering

### 3.1 Per-session, per-event HTTP endpoints are unnecessary

One authenticated endpoint is enough:

- `POST /hooks/<session_id>`

The body already includes `hook_event_name`.

Splitting every event into its own route adds surface area without adding meaningful value in v1.

> **Claude response:** Disagree. Per-event routes make the Go HTTP handler cleaner: each route maps to a typed handler method with its own request/response types. A single endpoint with event-name switching is the same code with more `switch` statements. Low-cost either way, keeping per-event routes.

### 3.2 The payload catalog is too broad for the first proof

The document includes a large amount of payload detail for:

- reconnect
- park/resume
- rich lifecycle transitions
- subagent activity

That is fine eventually, but for v1 proof-of-architecture the only payloads that really matter first are:

- `session.started`
- `approval.requested`
- `approval.resolved`
- `session.ended`
- one or two state transitions

Everything else can follow after the control path is real.

> **Claude response:** Disagree on deleting them. The payloads are field lists, not implementation. They cost nothing to keep in the doc and they prevent "what fields do we need?" questions later. Implementation order already sequences the core approval path first. The doc is a design record, not a sprint plan.

### 3.3 Batch auto-approve appears too early

The doc moves quickly from hook proof to `PreToolUse`-based auto-approval.

That is not the riskiest thing to prove first.

The first proof should be:

- one real `PermissionRequest`
- one real cockpit decision
- one real Claude outcome

Only after that should you optimize with batch/auto-approve.

> **Claude response:** Agree on sequencing. The implementation order (section 13) already puts manual approve/deny before batch. But the design doc should describe both because they use different hooks (`PermissionRequest` for manual, `PreToolUse` for batch) and both are v1 features. Describing them isn't the same as implementing them simultaneously.

### 3.4 Detach/reattach + restart + park/resume is too much in one v1 slice

The hook-first rewrite successfully removed one complexity class.

Do not immediately reintroduce complexity by insisting the same first implementation slice also prove:

- detach/reattach
- daemon restart recovery
- park/resume

Those can be sequenced after the basic hook approval path is real.

> **Claude response:** Agree. Per the PTY ownership fix (1.1), detach/reattach is deferred to v2. Park/resume payloads stay in the doc as field definitions but won't be implemented in the first slice. Daemon restart recovery is simplified: daemon restarts on the same port, hooks resume, no PTY reconnection needed.

## 4. Under-engineering

### 4.1 No concurrency model for pending approvals

The design does not say:

- how many `PermissionRequest` HTTP requests may be outstanding at once
- where they wait
- how the cockpit resolves them
- what happens if the operator never answers
- what happens if the cockpit disconnects but the daemon is still running
- what happens if two approvals arrive at nearly the same time for the same session

This needs an explicit pending-approval model, not just a happy-path description.

> **Claude response:** Accept. V1 model: one pending approval per session at a time. Claude is single-threaded per session, it blocks on the `PermissionRequest` hook and doesn't fire another until the current one resolves. So the concurrency is naturally bounded. But subagents could create parallel approvals across sessions. The daemon needs a per-session pending queue. If the operator never answers: the hook times out (600s default), Claude falls back to terminal prompt. Will add this to the doc.

### 4.2 No strong degraded state machine for hook loss

The document mentions `hook_channel_lost`, but not with enough rigor.

You need explicit transitions for cases like:

- hooks stop arriving but PTY is still alive
- daemon restarts and registry is not yet rebuilt
- Claude keeps running with local prompts during the gap
- approvals were granted locally while Tower was blind

Without this, the materialized state will be more confident than the evidence deserves.

> **Claude response:** Accept. Already agreed in 1.2. Will add explicit state transitions: `managed` -> `managed_degraded` (when hooks stop arriving but PTY alive) -> `managed` (when hooks resume). During `managed_degraded`, confidence drops, approval history shows a gap, and any approvals granted locally during the gap are marked `unverified`.

### 4.3 Data minimization is not enforced

The document says to persist bounded excerpts and structured metadata, but `approval.requested` currently includes full `tool_input`.

That is dangerous.

For tools like:

- `Write`
- `Edit`
- `Bash`

the full input may contain:

- file contents
- secrets
- destructive commands
- sensitive paths

The design needs a clear rule for:

- what is stored in full
- what is hashed
- what is excerpted
- what is excluded entirely

> **Claude response:** Accept. Will add a data minimization rule to the design doc:
> - `tool_name`, `risk_class`, `cwd`, `action_id`: stored in full
> - `tool_input.command` (Bash): stored in full (needed for audit of what was approved)
> - `tool_input.content` (Write), `tool_input.old_string`/`new_string` (Edit): SHA-256 hash only, not stored in full
> - `tool_input.file_path`: stored in full (path, not content)
> - Full hook JSON: held in memory for the duration of the pending approval, then discarded. Only the structured audit fields persist to SQLite.

### 4.4 Launch-time preconditions are missing

Managed launch should fail fast unless Tower can prove the session is entering a valid managed mode.

Examples:

- daemon endpoint is reachable
- hook auth is installed
- hook config is active
- port/token is stable for the session
- Claude settings/permission mode are compatible with Tower approval control

Right now the doc is too willing to say "launch succeeds and we hope the hook path is there."

> **Claude response:** Accept. Will add explicit launch preconditions that must pass before `tower run` spawns Claude:
> 1. Daemon is reachable on its HTTP port
> 2. Hook config file was written successfully
> 3. Daemon port and auth token are stable (persisted in lockfile)
> 4. Claude's permission mode is checked post-launch via `SessionStart` hook, if it's `dontAsk` or `bypassPermissions`, Tower downgrades the session from `managed` to `managed_visibility_only` (hooks provide observation but not approval control)

## 5. Security

### 5.1 Session ID in the URL is not enough

The doc already hints at this, but it undersells the issue.

`localhost` plus a session ID is not meaningful authentication.

Any local process that learns:

- the port
- the session ID

can spoof hook requests.

The design should require:

- a per-session secret
- carried in an HTTP header, not just the URL
- injected through Claude's supported HTTP hook header mechanism

> **Claude response:** Accept. Claude's HTTP hooks support `headers` with `allowedEnvVars` for interpolation. Tower can inject a per-session bearer token via env var and reference it in the hook header config. The daemon validates the token on every request. Will add this to the design.

### 5.2 Localhost-only still has attack surface

Even if the server binds only to `127.0.0.1`, attack surface remains:

- local unprivileged processes
- malicious browser pages targeting localhost
- unrelated software on the same machine

So "localhost-only" is necessary but not remotely sufficient.

> **Claude response:** Agree. The per-session bearer token (5.1) addresses local process spoofing. Browser-based localhost attacks are mitigated by the token requirement (browsers can't set arbitrary Authorization headers in cross-origin requests). The remaining attack surface is a local process that reads the token from the env or config file, which requires same-user access, at that point the attacker already owns the user's session.

### 5.3 Raw payload persistence is a larger risk than socket binding

The largest security/privacy risk in this design may not be the endpoint at all.

It may be SQLite storing raw hook payloads containing:

- command contents
- file contents
- path structures
- secret-bearing edits

That needs a stricter persistence policy than the doc currently gives it.

> **Claude response:** Accept. Covered in 4.3 response. The data minimization rule ensures file contents and edit diffs are hashed, not stored. Bash commands are stored in full because that's the audit value (what was approved to run). Path structures are stored because they're needed for conflict detection.

### 5.4 Unknown-session POSTs should not disappear silently

Returning `200 empty` for unknown session IDs may be operationally convenient, but it hides:

- spoof attempts
- stale-session behavior
- daemon-registry drift

If you keep the behavior, it should still emit an internal error/security event.

> **Claude response:** Accept. Will change behavior: return 200 empty (so Claude isn't blocked) but log an internal `error.reported` event with `code=unknown_session_hook`. If it happens repeatedly for the same session ID, escalate to a security warning in the cockpit.

## 6. Implementation order

The current order is close, but I would change it.

### Recommended order instead

1. **Hook endpoint proof with real Claude hooks**
   - confirm exact `PermissionRequest` and `PreToolUse` payloads
   - confirm exact timeout / non-2xx / connection-failure behavior
   - confirm what Claude shows to the user on failure

2. **Hook injection + config merge/precedence + auth-header proof**
   - prove how Tower installs hooks
   - prove compatibility with existing user/project/plugin hooks
   - prove per-session auth header works

3. **Permission-mode proof**
   - prove managed sessions actually produce `PermissionRequest` when Tower expects them to
   - identify incompatible Claude permission modes and fail fast

4. **Minimal `tower run` PTY wrapper**
   - spawn Claude
   - bridge stdin/stdout/resize
   - record stable session identity

5. **Single-session manual approve/deny path**
   - one blocked request
   - one cockpit action
   - one durable audit entry
   - one observed Claude outcome

6. **Daemon-restart degradation and reconnect**
   - prove the degraded state is honest
   - prove reconnect story without overstating continuity

7. **Only then add PreToolUse auto-approve / batch policy**
   - optimization after truth, not before it

> **Claude response:** GPT's order is better than mine. Key improvements:
> - Hook injection + config merge proof as a separate step before the PTY wrapper (my order assumed injection works and jumped to PTY)
> - Permission-mode proof as an explicit step (I had it as a footnote)
> - Batch auto-approve explicitly last
>
> Will adopt this order. One adjustment: I'd merge steps 1-2 into a single "hook proof" spike since you can't test the endpoint without injecting hooks, and you can't test injection without an endpoint to point at.

### One sequencing change I would make immediately

Move persistence earlier.

Audit is a trust guarantee, not polish.

If the first real approval flow is meant to be credible, it should already produce durable audit records.

> **Claude response:** Disagree. The first spike should prove the hook-to-cockpit-to-Claude loop works at all. In-memory audit is fine for that. SQLite adds a dependency (schema, migrations, concurrency) that slows down the proof. Once the loop works, persistence is step 8 in my order and step 5 in yours. Not last, but not first either. Audit is a trust guarantee, but the trust has to exist before you can persist it.

## Most important changes to make in the document

If I had to narrow this down to the most important edits:

1. resolve PTY ownership and restart topology contradictions
2. stop overstating recovery and introduce explicit degraded managed state
3. prove or downgrade the hook injection assumptions
4. add a real correlation story between `PreToolUse` and `PermissionRequest`
5. require authenticated hook requests
6. add a strict data-minimization rule for persisted hook payloads
7. reorder implementation around one real `PermissionRequest -> cockpit -> allow/deny` path before batch approval

## Claude's summary: what will change in the design doc

**Will fix (accept):**
1. Resolve PTY ownership: `tower run` owns PTY, detach/reattach deferred to v2
2. Add `managed_degraded` state for hook outages (audit gap, not just visibility gap)
3. Remove env-based hook injection, standardize on `.claude/settings.local.json`
4. Add daemon port/token persistence constraint (must restart on same port)
5. Add per-session bearer token via HTTP hook headers
6. Add data minimization rules (hash file contents, store commands in full)
7. Add launch preconditions (daemon reachable, hooks written, permission mode check)
8. Add pending approval concurrency model (one per session, timeout to terminal fallback)
9. Log unknown-session POSTs as internal errors
10. Adopt GPT's implementation order (merged steps 1-2)

**Will describe but defer implementation:**
- PreToolUse/PermissionRequest correlation (v1 treats each independently)
- Detach/reattach, park/resume (v2)

**Rejected:**
- Single endpoint vs per-event routes (style preference, keeping per-event)
- Deleting payload catalog (costs nothing to keep, prevents future questions)
- Moving persistence earlier (proof before polish)

## GPT follow-up after Claude inline review

Claude accepted most of the important corrections. That gets the design much closer to something I would build from.

I still want four points treated as hard constraints, not soft implementation notes:

### 1. Hook merge semantics are a release blocker

The biggest unresolved risk is still this:

- Claude runs matching hooks in parallel
- Tower may not be the only hook making a permission decision

If another hook can effectively let execution proceed before Tower's decision matters, then Tower cannot claim deterministic managed approval control in mixed-hook environments.

This is not a nice-to-have prototype detail. It is a release-blocking truth test.

The design should say explicitly:

- if Tower cannot prove safe merge behavior for `PreToolUse` and `PermissionRequest`
- then managed approval control must either require exclusive ownership of those hook types
- or the session must be downgraded to a weaker mode such as `managed_visibility_only`

### 2. Startup snapshot behavior makes endpoint stability non-negotiable

Claude correctly accepted that hook configuration is snapshotted at startup.

The consequence should be stated even more bluntly:

- the daemon must come back on the same port
- with the same auth material
- for the life of the running Claude session

If it cannot, Tower should not quietly "recover."

It should mark the session as degraded/unverified and stop claiming strong managed semantics until the next clean managed launch.

### 3. Correlation can be deferred, but the boundary must be explicit

I am fine with v1 treating each `PermissionRequest` as an independent approval event.

But if that is the v1 decision, the document should say two things clearly:

- manual approval uses `PermissionRequest` only
- batch/auto-approve uses `PreToolUse` only

And:

- v1 does **not** claim a fully proven correlation model between the two

That avoids accidentally implying more determinism than the implementation will actually have.

### 4. I can concede persistence for spike zero, but not for long

I can accept Claude's pushback that the very first wire proof may use in-memory audit.

But I still think the next step after the first successful `PermissionRequest -> cockpit -> allow/deny -> Claude outcome` loop should be durable audit persistence.

Specifically:

- before restart-recovery claims
- before batch approval claims
- before alpha or dogfooding claims

Audit is not polish. It is part of the trust model. I am only conceding that it can move one step later, not many steps later.

## Sign-off position

I would sign off on the design once the main doc is updated to:

1. remove stale PTY-helper / detach-reconnect language and make `tower run` ownership explicit
2. mark hook-control assumptions as "must validate," not as settled facts
3. add degraded managed states for hook/audit gaps
4. require authenticated hook requests with stable port/token behavior across daemon restart
5. state the v1 correlation boundary honestly
6. add the data-minimization rules and launch preconditions Claude accepted
7. keep durable audit immediately after the first real manual approval loop, even if not in spike zero
