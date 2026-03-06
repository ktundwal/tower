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

## Net result

- Tower is a local control plane with a hard line between observed and managed sessions.
- V1 goes deep on truthful managed control where Tower owns the session, and uses observe-only integration elsewhere.
- The architecture is event-first, policy-aware, and designed to preserve operator trust.
