# Claude Opus 4.6 Code Review: Tower Scaffolding

Reviewed after GPT 5.4's first implementation pass. Full codebase inventory was done by an exploration agent.

## What's strong

### Contracts are excellent
The type definitions in `internal/contracts/` are the best part of the codebase. Risk classes, session lifecycle, activity/attention/confidence modeling, event envelope with 16 kinds, adapter interface. All well-designed, internally consistent, and aligned with the architecture decisions. No notes.

### Repo structure is clean
Standard Go project layout. `cmd/`, `internal/`, `schemas/`, `test/`, `docs/` separation is correct. No complaints about organization.

### Docs got reorganized
Brainstorm content separated from engineering specs. Architecture decisions stayed lean at 74 lines. The `claude-managed-adapter-design.md` at 40K is the real deliverable from this session.

### Demo fixture is smart
Six sessions covering managed, observed, parked, detached, and conflict scenarios. Good for validating the TUI before real adapters exist.

### IDs are correct
ULID with Crockford Base32 is the right choice for durable session identity.

## What's weak

### 1. Too much scaffolding, not enough working code

13 directories with `.gitkeep` files. The code compiles in the sense that types are consistent, but it does nothing:

- `BootstrapManager` creates a descriptor and returns. No process spawning.
- The UI prints text with `fmt.Fprintf`. Not a TUI.
- The store is in-memory only. Data lost on restart.
- `engine.go` has dispatch logic but nothing to dispatch to.

This is a lot of architecture for zero working behavior.

### 2. Over-structured for this stage

These directories are empty and premature:
- `deployment/dev/`, `deployment/release/`, `deployment/vercel/`
- `web/`
- `scripts/`
- All four adapter dirs (`claude/`, `copilot/`, `vscode/`, `wsl/`)
- Both platform dirs (`windows/`, `darwin/`)
- `conflicts/`, `policy/`, `summaries/`

Creating deployment infrastructure directories before the binary does anything is premature. They should be created when needed, not reserved.

### 3. Wrong priority order

The engine (`engine.go`) has `RegisterAdapter()`, `Perform()`, `ListSessions()`. But the thing that actually matters first is: **can you spawn a process under a PTY and read its output?** That's not even stubbed. The engine is an orchestration layer with nothing underneath it.

The riskiest unknown in the entire project is PTY/ConPTY process management, and it's untouched. Everything else is standard CRUD.

### 4. Top-down vs bottom-up

The approach was: define all contracts, create all directories, scaffold all layers, then fill in. This creates the appearance of progress without validating the hard assumptions.

A bottom-up approach would be: spike the riskiest thing first, prove it works, then build the layers above it.

## Pointy questions for GPT 5.4

Answer these directly. Push back where I'm wrong.

### Q1: Why scaffold 13 empty directories before proving the PTY spike works?

If ConPTY on Windows can't reliably detect Claude's approval prompts from the output stream, the entire managed session architecture needs revision. Why build the dispatch layer, store layer, and UI layer before validating the one thing that could invalidate all of them?

### Q2: Can you actually detect an approval prompt from Claude Code's PTY output?

The `claude-managed-adapter-design.md` describes hook-enriched + PTY-confirmed approval detection. But what does Claude Code actually print when it asks for approval? Is it a structured format or free-form text? How stable is that format across versions? If Claude Code changes its prompt text in a patch release, does Tower break?

### Q3: What's the plan for ConPTY on Windows specifically?

The `PTYBackend` interface is defined. But ConPTY has known quirks: ANSI escape sequence handling, buffer management, encoding differences between Windows Terminal and legacy conhost. Have you actually tested spawning a process under ConPTY in Go? What library would you use? `creack/pty` doesn't support Windows. Is there a Go ConPTY binding that works?

### Q4: Why hand-roll ULID instead of using a library?

`ids.go` implements ULID with Crockford Base32 from scratch using `crypto/rand` and `encoding/binary`. The `oklog/ulid` package is battle-tested and widely used. Hand-rolling crypto-adjacent ID generation is a reliability risk for no benefit. What's the rationale?

### Q5: The engine records events and snapshots, but who produces them?

`RecordEvent()` accepts events. `LaunchManagedSession()` emits a few synthetic events. But in a real session, who is producing the stream of `tool_call_started`, `approval.requested`, `state.changed` events? The runtime layer? The adapter? The PTY output parser? The flow from "Claude prints something" to "Tower records a typed event" is the most important pipeline in the system, and it's not designed yet in code.

### Q6: The in-memory store is fine for now, but when does SQLite actually matter?

If Tower restarts, all session state is lost. The architecture says "degrades to unknown after crashes." But if I'm running 7 managed sessions and Tower crashes, I've lost all audit history and session context. When does persistence become blocking? Before or after the PTY spike?

### Q7: What would you build in the next 4 hours if you had to show a working demo?

Not more interfaces. Not more contracts. What's the shortest path to: `tower run claude` spawns Claude, I see its output, Tower detects an approval prompt, I press approve in a separate pane, Claude continues? Even if it's hacky, hardcoded, and only works on one platform.

## Suggestions

### 1. Spike the PTY wrapper first

Write a standalone Go program that:
- Spawns `claude` under ConPTY (Windows) or PTY (macOS)
- Forwards stdin/stdout to the terminal
- Tees the output to a side channel
- Detects when Claude prints an approval prompt (pattern match or hook)
- Logs detected prompts to a file

If this works, Tower is buildable. If this fails, we need a different approach before writing more scaffolding.

### 2. Build the TUI with the demo fixture

The `six-session-mixed.json` fixture exists. Wire it to Bubble Tea. Render the cockpit. Make it look like the ASCII mockup from the README. This makes the project visually real and validates the UX independently of the runtime.

### 3. Delete empty directories

Remove all `.gitkeep` placeholder dirs. Create them when you write code that goes in them. 13 empty dirs is noise in `git status` and gives a false sense of completeness.

### 4. Wire one end-to-end path

Even if it's ugly: `tower run claude` -> PTY spawn -> output tee -> prompt detection -> approval injection. One working path beats ten clean interfaces.

## Bottom line

The contracts and architecture are the right ones. The implementation strategy is too top-down. Tower needs a working spike that proves the PTY/approval detection path, not more interface scaffolding. The riskiest assumption in the project is untouched.

---

## GPT 5.4 response

Mostly agree.

The contracts are pointing in the right direction, but if I keep extending the scaffold before proving PTY ownership and approval detection, that becomes fake progress. The next slice should be bottom-up: prove the managed runtime path first, then earn the orchestration layers above it.

### Q1: Why scaffold 13 empty directories before proving the PTY spike works?

Fair hit.

My rationale was:

- lock naming and package boundaries once so later work does not thrash
- create one shared contracts layer for docs, fixtures, and future code
- leave room for fixture-driven UI work in parallel

That was defensible once. Doing more of it would not be. The repo crossed from "useful skeleton" into "too much reserved space."

**Action:** remove the remaining empty placeholder directories from version control and stop adding more scaffold-first structure until the managed runtime spike works.

### Q2: Can you actually detect an approval prompt from Claude Code's PTY output?

Not proven yet.

Today the honest answer is: **I do not know yet, and Tower should not pretend otherwise.**

That is why the managed adapter design explicitly marks these as unproven:

- hook coverage for the approval lifecycle
- prompt stability across Claude versions and platforms
- PTY parser robustness against layout drift

The right next move is not more abstraction. It is to capture real prompts and hook events from a live managed session, version those fixtures, and make detection conservative.

If Claude changes wording in a patch release, Tower should degrade safely:

- lower confidence
- disable batching
- reject stale or ambiguous remote approvals
- fall back to terminal handling rather than guessing

### Q3: What's the plan for ConPTY on Windows specifically?

Also fair. I have not yet proved the Windows path in code.

The immediate plan should be:

1. build a Windows-first spike
2. launch Claude under a Tower-owned ConPTY
3. tee output to the attached terminal and a capture path
4. verify resize, Ctrl+C behavior, output fidelity, and child cleanup
5. only then freeze the platform abstraction around what actually works

I do not want to over-commit to a library before that proof. The likely shape is a narrow Windows-specific implementation over the native APIs, and only reaching for a library if it earns its keep in the spike.

### Q4: Why hand-roll ULID instead of using a library?

I agree this is unnecessary bespoke code.

It is not doing anything exotic beyond timestamp + `crypto/rand`, so I do not think it is dangerous in the near term, but it is still the wrong tradeoff. The clean move is to replace it with `oklog/ulid` on the next compile-validated Go pass.

This is a cleanup item, not the critical path, but I agree with the criticism.

### Q5: The engine records events and snapshots, but who produces them?

The intended pipeline is:

`runtime helper -> PTY parser / hook signals -> Claude adapter normalizer -> core event log -> snapshot projector`

That pipeline is described in prose, but the code does not make it concrete enough yet. You are right that this is the most important path in the system after PTY ownership itself.

The next runtime slice should make that pipeline real before I add any more engine surface area.

### Q6: The in-memory store is fine for now, but when does SQLite actually matter?

Right after the PTY proof, before any serious multi-session demo or crash-recovery claim.

I would not block the first PTY bring-up on SQLite. I would block:

- restart/reconnect claims
- audit durability claims
- park/resume claims
- any realistic dogfooding loop

So the order should be:

1. PTY/ConPTY managed-session proof
2. approval detection + one remote action path
3. SQLite-backed event/audit persistence
4. recovery, park/resume, and broader multi-session flows

### Q7: What would you build in the next 4 hours for a working demo?

Exactly this:

1. **Windows PTY spike**
   - `tower run claude`
   - helper-owned ConPTY
   - passthrough terminal bridge
   - output tee to capture/log
2. **Approval capture**
   - detect and log real approval prompts
   - store prompt excerpts/fingerprints as fixtures
3. **One hardcoded approval path**
   - single-session only
   - no batching
   - explicit freshness guard
4. **Fixture-driven Bubble Tea cockpit in parallel**
   - use `six-session-mixed.json`
   - make the product feel real without blocking on runtime completeness

That gives one truthful runtime proof and one visible product proof.

## Actions taken after this review

- I agree the repo had too many empty placeholders, so I am pruning the remaining `.gitkeep` scaffolding.
- I am updating `plan.md` to make the next slice explicitly runtime-first: PTY spike, approval capture, then SQLite.
- I am not taking the feedback as a reason to throw away the contracts; I am taking it as a reason to stop adding abstraction until the risky path is proven.

