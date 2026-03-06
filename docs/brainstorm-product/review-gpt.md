# GPT Cross-Model Review: Tower

## 1) Understanding of the problem space

This resonates strongly. The core problem is not "too many terminals"; it is **human supervision of multiple semi-autonomous agents competing for attention on one machine**. The pain is not lack of notifications. The pain is:

- no reliable bird's-eye view of session state
- no cheap way to rebuild context for a blocked session
- no way to batch low-risk approvals
- no control plane for remote nudges, parking, resuming, and ending sessions

The strongest part of the framing is that you correctly identified the desired surface as a **cockpit**, not a smarter notifier. That is the right abstraction.

What is missing from the current problem space is the fact that this is not just observability. It is a **local control plane** with three separate jobs:

1. **Observe** what sessions are doing
2. **Decide** which sessions deserve attention and which actions are safe
3. **Act** on sessions without breaking the direct terminal workflow

Those are different problems with different technical constraints. Right now the docs blend them together a bit too much.

You are also underspecifying the trust problem. The operator is not only asking "which session is blocked?" They are asking:

- what exactly is being requested?
- how risky is it?
- is it equivalent to the other pending approvals?
- did I already approve this pattern somewhere else?
- what happened after the approval?

That risk/provenance layer will determine whether Tower feels useful or dangerous.

## 2) Feedback on the problem definition

The problem definition is directionally right, but it is still missing critical boundaries and success criteria.

### What is good

- Same-machine first is the correct scope reduction
- "Bird's-eye cockpit" is concrete and opinionated
- Batch approval is a real differentiator, not a UI flourish
- "Summary view" and "session memory" are important because raw logs do not scale
- Extensibility via adapters is the right long-term instinct

### What is missing

#### A. Explicit operator workflows

You should define the top 5 operator loops clearly. For example:

1. Glance at all sessions and find the one that needs me
2. Batch approve identical low-risk actions
3. Zoom into one risky approval and inspect context
4. Send a corrective command to a drifting session
5. Park or terminate a session and retain enough context to resume later

If Tower does not optimize these loops end-to-end, it will become an interesting dashboard instead of a daily tool.

#### B. A hard definition of session state

`working | blocked | idle | done` is too shallow for a real control plane. You need a canonical state machine with well-defined transitions and confidence levels. At minimum:

- starting
- running
- waiting_human
- waiting_tool
- waiting_external
- idle
- completed
- failed
- detached_or_unknown

You also need to represent **confidence**. "Blocked" inferred from lack of output is not the same as "blocked" inferred from a concrete approval prompt.

#### C. Success metrics

You need measurable outcomes now, before implementation. Suggested metrics:

- median time to identify the correct blocked session
- median time to resolve approvals across N concurrent sessions
- number of terminal/window switches avoided per hour
- percentage of approvals eligible for safe batching
- context rebuild time from alert to informed decision
- false-positive and false-negative rate for blocked detection

Without these, the product can feel good in demos and still fail in real use.

#### D. Explicit non-goals

You should state what Tower is not trying to be:

- not a cloud orchestrator
- not a replacement for the agent UI
- not a general-purpose terminal multiplexer
- not a workflow engine for autonomous agents
- not a source-of-truth transcript store for every tool forever

That will keep the architecture from bloating.

#### E. Security and policy model

Batch approval is not a UX feature. It is a policy engine problem. You need a first-class definition of:

- action risk classes
- equivalence classes for batchability
- audit trail requirements
- local-only guarantees
- data retention and redaction rules

Right now that is mostly implicit. It needs to be explicit.

## 3) Feedback on the architecture approach

### The good news

The **side-channel for read path** is exactly right. Monitoring via hooks, logs, process discovery, and extension APIs is the right way to avoid degrading direct terminal use.

The **adapter pattern** is also right, but the current interface is too RPC-shaped and not event-native enough for the actual problem.

### The main architectural issue

Your docs say "side-channel, not wrapper," but your required write-path features are:

- approve/deny/chat responses
- remote commands
- park/resume

Those are not passive capabilities. They are **control-plane operations**. For many tools, especially terminal-native tools, that means owning or mediating the session I/O path. If you need deterministic control, you are no longer purely side-channel.

That is not a philosophical issue. It is a hard technical one.

### Recommendation: split the system into read plane and control plane

Do not model Tower as one thing. Model it as two planes:

#### Read plane

- process discovery
- hook ingestion
- log ingestion
- summary generation
- state inference
- timeline reconstruction

#### Control plane

- approval response
- command injection
- pause/park/resume
- termination
- takeover and handoff

This split matters because different sessions will support different levels of control.

### Recommendation: distinguish observed sessions from managed sessions

This is the most important product and architecture distinction missing from the docs.

#### Observed session

- discovered from the side
- readable
- state may be inferred
- may expose pending actions
- not guaranteed remotely controllable

#### Managed session

- launched under Tower supervision or via a Tower-compatible shim
- fully controllable
- approvals and command injection are deterministic
- stronger session identity and audit guarantees

If you do not make this distinction, you will promise capabilities you cannot reliably deliver across tools.

### Recommendation: make the adapter contract capability-based

The current interface is too generic and too imperative:

```text
discover_sessions()
get_session_state(id)
get_pending_actions(id)
get_activity_log(id)
respond_to_action(id, action_id, response)
send_command(id, command)
get_session_summary(id)
```

This will become brittle because adapters vary wildly in what they can know and do.

A better model is:

```text
discover_sessions() -> session descriptors
subscribe_events() -> event stream
get_snapshot(session_id) -> current materialized state
list_capabilities(session_id) -> set of supported actions
perform(session_id, action) -> result
```

And the event schema should be canonical, for example:

- `session.discovered`
- `session.updated`
- `session.ended`
- `state.changed`
- `approval.requested`
- `approval.resolved`
- `summary.updated`
- `command.received`
- `command.applied`
- `error.reported`

Build the UI from materialized state derived from an append-only event log. Do not make the UI poll a bag of synchronous getters and hope the world stays consistent.

### Push vs pull

Do not pick one. Use a hybrid model:

- **push** when hooks, pipes, or extension events exist
- **pull** for reconciliation, liveness, and missed-event recovery

Pure polling will feel laggy and imprecise. Pure push will be fragile and lose truth after crashes or restarts.

### Major risks

1. **Trying to support Claude Code, Copilot CLI, and VS Code equally in v1**
   - These are three very different integration surfaces
   - One terminal CLI with hooks, one CLI with unclear hooks, one IDE with extension APIs
   - This is too much surface area for a v1 if you also need reliable control operations

2. **Brittle scraping**
   - If the system relies on screen scraping, log parsing without schemas, or keystroke injection, it will rot quickly

3. **Approval races**
   - User answers in terminal while Tower shows pending approval
   - Tower batch-approves stale prompts
   - Prompt text changes between request detection and action

4. **Session identity drift**
   - PID is not identity
   - Window title is not identity
   - Wrapper processes make identity messy

5. **Summary trust**
   - If summaries are not source-attributed and incrementally updated, users will stop trusting them

### Strong recommendation for v1

Do **one deep integration** first:

- Claude Code as a managed session
- passive observation only for everything else

That gets you a truthful v1 instead of a broad but unreliable one.

## 4) The hardest problem: approval proxy / stdin interception

This is the hardest problem because it is the one place where the current architecture story is weakest.

### Direct answer

If a tool blocks on stdin for approval and exposes no external control API, **you cannot build a reliable remote approval mechanism without being in the I/O path or without tool cooperation**.

That is the reality.

There is no magical generic side-channel that can safely:

- observe the pending prompt
- know exactly which response maps to which prompt
- inject the response
- guarantee it was consumed by the right process at the right time
- avoid racing with local user input

### What this means architecturally

For approval and command injection, you have only three real options:

1. **Own the PTY/ConPTY from launch**
   - robust
   - deterministic
   - effectively a wrapper/shim, even if UX stays transparent

2. **Use a tool-native external control API**
   - ideal
   - currently unavailable for the tools you described

3. **Use OS-level input injection into an existing console**
   - fragile
   - race-prone
   - security-sensitive
   - not a sound foundation for an open-source product

### Windows-specific reality

On Windows, if Tower launches the session under **ConPTY**, Tower can own the pseudoconsole and still let the user interact through a terminal. That is the correct technical foundation for managed sessions.

For already-running sessions in arbitrary consoles, anything based on:

- `AttachConsole`
- writing console input records
- foreground window automation
- simulated keystrokes

is brittle and should not be the core design.

### The right product decision

Do not promise remote approvals for arbitrary already-running sessions.

Instead, define two modes clearly:

#### Observe-only mode

- discover session
- infer state
- show activity and summaries
- maybe deep-link to the terminal
- no guaranteed remote approval

#### Managed mode

- session launched by Tower or a Tower shim
- full approval proxy
- full command injection
- deterministic audit trail
- safe handoff between cockpit and terminal

This is the honest and technically defensible line.

### If you insist on side-channel control

Then require explicit tool cooperation. For example:

- a hook or plugin emits an approval request with a stable action ID
- the tool blocks on a named pipe / local socket response instead of raw stdin
- Tower responds through that brokered channel

That would preserve the non-wrapper story better, but it requires vendor/tool support. Without that support, stdin interception remains wrapper territory.

### Additional hard problems inside approval handling

Even after you solve transport, you still need:

- stable action IDs
- action TTLs and staleness detection
- idempotent approve/deny operations
- proof that the action shown in Tower still matches the live prompt
- clear conflict resolution when local terminal input wins first
- grouping rules strict enough to avoid catastrophic batch approvals

The real problem is not "how do I send `y`?" The real problem is "how do I prove that this `y` applies to the right request?"

## 5) Prior art or projects you may have missed

The most important missed prior art is not another AI tool. It is **terminal control and session management software**.

### Highly relevant prior art

#### tmux / screen / zellij

These are not AI supervisors, but they solve:

- session identity
- pane/session attachment and detachment
- remote input routing
- persistence
- status surfaces

You should study their control models even if you do not copy their UX.

#### WezTerm / kitty remote control

Modern terminal emulators expose rich control primitives:

- enumerate panes/tabs
- send text
- capture output
- manage sessions

If Tower ever wants deep terminal integration without owning every tool directly, terminal-emulator APIs are more promising than trying to scrape random consoles.

#### OpenHands

OpenHands is different in product shape, but relevant in one key way: it treats agent runs as structured executions with explicit state and control surfaces. That mindset is closer to Tower than raw CLI wrappers are.

#### Continue / Cline / Roo Code / Cursor-style IDE agents

These are useful as examples of:

- IDE-native event models
- long-lived assistant state
- approval/request UX inside editor surfaces

Their extension patterns matter for the VS Code adapter.

#### Supervisord / systemd / process supervisors

Not AI-specific, but relevant for:

- lifecycle state models
- health checks
- restart semantics
- managed vs unmanaged process distinction

Tower is effectively borrowing process-supervisor ideas and applying them to human-in-the-loop agent sessions.

### What not to overlearn from

- post-hoc transcript tools alone
- notification systems alone
- generic dashboard frameworks

Those solve only slices of the problem.

## 6) Platform concerns: Windows-first and cross-platform

Windows-first is not a minor implementation detail. It changes the architecture.

### Good news

Building Windows-first forces rigor around process boundaries, console ownership, and IPC. That is good.

### Hard realities

#### A. ConPTY vs PTY is a real split

You need a platform abstraction for:

- pseudoterminal hosting
- signal and interrupt semantics
- process tree management
- encoding/ANSI behavior
- session attachment/detachment

Do not design a Unix PTY architecture and "port it later." Start with a clean abstraction now.

#### B. Windows dev boxes are often mixed environments

Your target user is very likely running some mix of:

- PowerShell
- Windows Terminal
- Git Bash / MSYS2 / MINGW64
- WSL
- VS Code remote/dev containers

That means "same machine" does **not** imply one process model or one path model. Session discovery and command routing across native Windows, MSYS, and WSL boundaries is messy.

You need to decide explicitly:

- Is v1 native Windows terminals only?
- Is WSL included in v1?
- Are remote/dev-container sessions only observed, or fully managed?

If you do not draw that line early, platform complexity will eat the project.

#### C. Named pipes are the right default on Windows

For Windows-local IPC, named pipes are the correct default. Build a transport abstraction so Unix domain sockets can be the Unix implementation later.

#### D. Job Objects matter

If Tower manages processes on Windows, use Job Objects for lifecycle control and cleanup. Otherwise process trees will leak and status inference will get messy.

#### E. Console and permission edge cases

Be careful with:

- per-user console isolation
- access rights for process handles
- handle inheritance
- ANSI enablement differences
- path normalization across shells

These are not edge cases in practice. They are daily reality on Windows development machines.

### Cross-platform recommendation

Support matrix should be explicit from day one:

1. **Tier 1**: native Windows managed sessions
2. **Tier 2**: native Windows observed sessions
3. **Tier 3**: WSL observed sessions
4. **Tier 4**: Unix/macOS later

Do not pretend all of these are equivalent.

## 7) Additions to the problem space you have not fully considered

### A. Conflict detection across agents

This is a major missing piece.

If five agents are working across the same repo, branch, or file set, the supervisor needs to know:

- overlapping file edits
- competing git operations
- rebase/merge risk
- lock contention
- duplicated effort

This is not a nice-to-have. It is one of the core reasons a supervisor cockpit is valuable.

### B. Risk classification and policy

Approvals should not be grouped only by string similarity. They need canonical action classification:

- read-only filesystem
- write filesystem
- git read
- git mutating
- network read
- network write
- package install
- process execution
- secret access

Batching should happen by policy, not by vibes.

### C. Provenance and auditability

Tower should record:

- what was requested
- which session requested it
- what context was shown to the operator
- who approved it
- when it was approved
- what happened afterward

Without this, batch approval becomes operationally scary.

### D. Attention ranking, not just state display

A cockpit should prioritize. Not all blocked sessions are equally important. You need an urgency model based on:

- human waiting
- action risk
- time blocked
- repo conflict risk
- likely completion value
- repeated failed attempts

Otherwise the user still has to manually sort noise.

### E. Durable session identity

You need stable logical session IDs that survive:

- PID changes
- wrapper layers
- terminal reattachment
- process restarts

This identity model should be designed before adapters are written.

### F. Summary trust model

Summaries must be:

- incremental
- source-attributed
- easy to drill into
- clearly separated between raw facts and synthesized interpretation

If summaries are just LLM prose without traceability, they will become decorative.

### G. Adapter ecosystem design

If community adapters matter, publish:

- a versioned adapter protocol
- a simulator/test harness
- fixture logs and example sessions
- capability negotiation rules
- conformance tests

Otherwise the adapter ecosystem will fragment immediately.

## Bottom line

The problem is real, important, and sharply defined enough to justify building. The strongest insight is the shift from notifications to a cockpit. The weakest part is the current insistence on "side-channel, not wrapper" while also requiring deterministic approval and command injection.

My blunt recommendation:

1. **Adopt a two-tier model now: observed sessions vs managed sessions**
2. **Build the first managed integration around Claude Code with Tower-owned ConPTY/PTY**
3. **Make the architecture event-first, capability-based, and policy-aware**

If you do that, Tower has a credible path to becoming the local control plane for AI coding agents. If you do not, it risks becoming a fragile dashboard built on terminal automation tricks.
