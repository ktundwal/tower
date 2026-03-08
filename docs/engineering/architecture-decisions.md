# Tower Architecture Decisions

This file is the canonical durable record of Tower's architecture decisions. It intentionally keeps only the technical and product-boundary decisions the implementation should build against.

## 1. Observed vs managed sessions

- Tower has two public session classes: `observed` and `managed`.
- Observed sessions are read-mostly. Tower may show discovery, identity, liveness, best-effort state with confidence, recent activity, summaries if available, and deep-link/jump to the terminal or IDE.
- Tower does not publicly promise remote approve/deny, command injection, park/resume, or other deterministic control for observed sessions unless a tool exposes a stable external control surface.
- Managed sessions are the controllable tier.

## 2. `tower run <tool>` is required for deterministic control

- Any session that needs deterministic approvals, command injection, parking, or resume must be launched through `tower run <tool>`.
- If Tower did not launch the session, it remains observe-only unless the tool provides a real external control API.

## 3. Session identity model

- Managed sessions get a Tower-issued durable session ID at launch.
- That managed identity survives PID changes, wrapper layers, terminal/tab moves, and restarts.
- Observed sessions may be correlated best-effort across rediscovery, but that continuity is not guaranteed and must be shown with low confidence when inferred rather than proven.

## 4. Approval safety model

- V1 batch approval is limited to read-only actions.
- Any write, network, or otherwise mutating action requires individual review.
- Any stale prompt or any prompt that may already have been answered locally must fall back to individual review.
- Batch approval is policy-based, not string-similarity-based.

## 5. Authoritative event and state model

- Tower uses a richer session state model with substates and confidence levels.
- After missed events, crash gaps, or Tower restarts, sessions degrade to `unknown` / low-confidence until reconciliation.
- Tower should not guess its way back into a strong state.

## 6. V1 platform and environment scope

- This was treated as an engineering scope decision.
- V1 supports managed sessions on native Windows and native macOS.
- WSL sessions are observe-only in v1.
- VS Code remote / dev-container sessions are observe-only in v1.

## 7. Core architecture direction

- Tower defaults to an event-first architecture with materialized state.
- Polling/getters are reserved for reconciliation, liveness checks, and recovery.
- A simpler polling-heavy mode may exist only as a lab/experimental feature that can be turned on or off.

## 8. Cockpit vs terminal input ownership

- The terminal remains the default owner of input during focused work.
- Tower may take temporary control only for an explicit cockpit action.
- Handoffs must be clear and reversible.
- Tower should be able to read session logs later without getting in the way of direct terminal use.

## 9. Cross-session conflict handling

- Tower should warn about likely cross-session collisions in v1.
- Initial target conflicts include same repo, same file, same branch, and competing git activity.
- Tower should not only show sessions independently; it should also surface likely coordination risks.

## 10. Audit and trust guarantees

- Tower keeps a durable local record for every control action.
- That record includes what was requested, what context was shown, what the operator chose, and what happened next.
- Records are retained until the user deletes them.
- Batch approval and remote control depend on this provenance model.

## 11. Process topology: daemon + managed launcher + hooks

- Tower runs as a persistent **daemon process** on the local machine.
- The daemon owns the event store, materialized session state, policy engine, audit log, and the TUI cockpit.
- The daemon listens on a local HTTP endpoint for events from hooks running inside agent sessions.
- `tower run <command>` is a thin managed launcher that:
  - Starts the daemon if not already running.
  - Spawns the given command under a Tower-owned PTY/ConPTY.
  - Auto-injects hook configuration into the spawned session (via env vars or temp settings) pointing hooks at the daemon's local API.
  - Registers the session as managed with a durable session ID.
- `tower run` treats everything after it as the command to exec. It composes with any wrapper:
  - `tower run claude` - direct Claude Code
  - `tower run copilot --model gpt-5.4` - direct Copilot CLI
  - `tower run <any-wrapper> claude --flags` - any tool wrapper that spawns an agent underneath
- Tower does not parse or interpret what comes after `tower run`. It spawns the command, owns the PTY, and injects hooks. Env vars propagate to child processes, so hooks work through wrapper layers.

## 12. Hook-mediated event stream and approval channel

- Agent tool hooks (Claude Code hooks, Copilot CLI hooks) are the primary integration surface for both the read path (monitoring) and write path (approvals).
- Hooks fire on tool calls, permission requests, notifications, session lifecycle events, and emit structured JSON.
- Hooks POST events to the Tower daemon's local HTTP API.
- For approvals, the hook receives the tool call details (tool name, args, cwd, risk context) and can return a decision (allow, deny, or queue for human review) based on the daemon's response.
- This means approval handling is **hook-mediated**, not PTY-output-parsing-based. Hooks provide structured data natively; PTY parsing is a fallback for tools that don't support hooks.
- The PTY wrapper remains valuable for: session identity, terminal bridging (detach/reattach), process lifecycle management, and env var injection for hook configuration.

## 13. Adapter hook injection

- Each adapter knows how to inject hook configuration for its tool:
  - Claude Code: hooks in `.claude/settings.json` or via `CLAUDE_HOOKS_*` env vars
  - Copilot CLI: hooks via plugin system or config
  - Future tools: adapter implements hook injection for that tool's config format
- `tower run` delegates hook injection to the adapter for the detected tool.
- Users never manually configure hooks. `tower run` handles it.
- Observed sessions (not launched via `tower run`) have no hooks unless the user has configured them globally. This is why observed sessions are visibility-only by default.

## Net result

- Tower is a local control plane with a hard line between observed and managed sessions.
- Tower runs as a persistent daemon with a local HTTP API that receives hook events from all managed sessions.
- `tower run <command>` auto-injects hooks and owns the PTY. It composes with any tool wrapper.
- Approval handling is hook-mediated, not stdin-interception-based.
- V1 goes deep on truthful managed control where Tower owns the session, and uses observe-only integration elsewhere.
- The architecture is event-first, hook-driven, policy-aware, and designed to preserve operator trust.
