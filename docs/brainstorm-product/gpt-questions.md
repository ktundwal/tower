# GPT-5.4: Top 10 Architecture Questions

These are the questions I would push on first, ranked by importance.

1. **What exact capabilities are guaranteed for observed sessions vs managed sessions, and where is the product line drawn publicly?**
   If this boundary is fuzzy, Tower will overpromise and the architecture will rot around exceptions.

2. **Are we willing to make `tower run <tool>` the required path for any session that needs deterministic approvals, command injection, parking, or resume?**
   If the answer is no, then we should explicitly give up on reliable remote control for those sessions.

3. **What is the canonical session identity model, and how does that identity survive PID changes, wrapper layers, terminal moves, and restarts?**
   If session identity is weak, everything above it—state, approvals, summaries, audit, resume—becomes untrustworthy.

4. **What is the approval safety model: risk classes, batch-equivalence rules, staleness checks, and conflict resolution when local terminal input races Tower?**
   The hard problem is not sending `y`; it is proving that this approval is still the right one to send.

5. **What is the authoritative event model and state machine for a session, including confidence levels and recovery after missed events or crashes?**
   A control plane built on ambiguous states like "blocked" or "working" will fail under real load.

6. **What is v1 scope by platform and environment: native Windows only, Windows Terminal only, WSL observed-only, VS Code remote observed-only, etc.?**
   "Same machine" is not a real boundary unless you say exactly which process and console models are supported.

7. **Do we want an event-first architecture with materialized state, or an RPC/polling architecture with adapter getters—and what failure modes are we accepting either way?**
   This decision will shape adapter complexity, UI responsiveness, replay, reconciliation, and debugging.

8. **What are the control-plane semantics for handoff between cockpit and terminal: who owns input, when can ownership switch, and how is that made safe and legible to the user?**
   Managed sessions live or die on whether this handoff feels invisible when it should and explicit when it must.

9. **How will Tower detect and surface cross-session conflicts—overlapping file edits, competing git operations, duplicate work, and resource contention—and is that in v1 or explicitly deferred?**
   If Tower only shows per-session status, it misses one of the biggest reasons to have a supervisor at all.

10. **What audit and trust guarantees are required for every control action: what was requested, what context was shown, who approved it, what happened next, and how long that record is kept?**
    Without a durable provenance story, batch approval and remote control will feel dangerous instead of empowering.
