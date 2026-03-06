# Cross-Model Review: Tower

## 1. Understanding of the Problem Space

The problem of "agent sprawl" resonates strongly. As developers move from "copilot" (one active helper) to "manager" (orchestrating multiple autonomous agents), the UI paradigm must shift from a single chat window to a control plane. The current friction of `Alt+Tab` -> `scan context` -> `approve` -> `switch back` is O(N) with the number of agents, which puts a hard ceiling on concurrency.

**Resonates:**
*   **The "Toast Notification" Anti-pattern:** Notifications are synchronous interrupts for asynchronous tasks. Converting these into a passive "inbox" of approvals is the correct UX shift.
*   **Batch Approvals:** This is the killer feature. Approving 5 `grep` commands individually is toil; approving them in batch is management.

**What's Missing:**
*   **Resource Contention:** 7 agents running on one box will contend for CPU, disk I/O, file locks, and API rate limits. Tower is positioned to visualize this but the current scope focuses only on logical blocking (user input), not system blocking.
*   **Agent Drift/Conflict:** If Agent A and Agent B are modifying the same codebase, they *will* collide. A supervisor needs to detect if Agent A is editing a file that Agent B is reading/planning on.

## 2. Feedback on Problem Definition

The definition is "complete enough" for a v1, but has a few blind spots:

*   **Completeness:** The focus on "side-channel" vs "wrapper" is a crucial distinction, but might be technologically self-defeating (see section 4).
*   **Blind Spots:**
    *   **The "Context Restoration" Problem:** When zooming into a session, seeing just the "recent activity" might not be enough. The user needs to know *why* the agent is stuck. Tower needs to pull not just the last log line, but the *reasoning trace* leading up to the request.
    *   **Zombie Processes:** With 70+ processes, cleanup is a major issue. Tower should explicitly handle "kill session" which entails cleaning up the process tree (node, workiq, etc.), not just the parent CLI.

## 3. Feedback on Architecture Approach

**The "Side-Channel" Risk:**
You stated: *"Tower monitors from the side, doesn't wrap terminal I/O."*
**Critique:** This is the most dangerous architectural decision. While "no wrapper" sounds clean, it makes state tracking probabilistic rather than deterministic. If the user interacts with the terminal directly (bypassing Tower), Tower's state model (e.g., "waiting for approval") becomes stale immediately.

**Alternatives:**
*   **Transparent Wrapper:** Instead of "side-channel", use a transparent PTY wrapper (like `tower run claude`). It acts as a passthrough for standard usage but allows Tower to reliably intercept triggers and inject input. This is how `script`, `screen`, and `tmux` work. It is robust.
*   **The Adapter Pattern:** This is solid. However, the interface needs an event subscription model (`on_state_change`), not just polling methods (`get_state`), to make the TUI responsive.

## 4. The Hardest Problem: Approval Proxy / Stdin Interception

You correctly identified this as the hardest problem.

**The Reality:**
Injecting stdin into an arbitrary running process on Windows *without* wrapping it is essentially malware behavior. You would need to use `WriteConsoleInput` (which requires attaching to the console, potentially detaching the user) or a debugger API (slow, invasive). On Linux, `TIOCSTI` is deprecated.

**Recommendation:**
**Abandon the "Pure Side-Channel" constraint for the Write path.**
You cannot robustly "inject" answers to a process you don't own/parent.
*   **Solution A (Wrapper):** `tower launch claude`. Tower owns the stdin/stdout handles. It passes them through to the real terminal normally, but can hijack them when it needs to send an approval. This is reliable.
*   **Solution B (IPC/API):** If the tools (Claude/Copilot) expose a local HTTP server or named pipe for control, use that. If they only listen on stdin, you *must* control stdin.

**If you stick to side-channel:** You will likely resort to sending virtual keystrokes to the specific window HWND. This is fragile (requires window focus, breaks if user moves mouse, fails in headless/remote scenarios).

## 5. Prior Art & Missed Projects

*   **Terminal Multiplexers (Tmux / Zellij):** Tower is effectively an "Application-Layer Tmux". Zellij has a plugin system (WebAssembly) that allows creating "panes" that react to content in other panes. You could potentially build Tower *as a Zellij plugin* rather than a standalone app.
*   **Process Supervisors (Supervisord / Circus):** These manage process lifecycle, restarts, and stdout/err capture.
*   **OpenAI Swarm / LangGraph:** While these are libraries, their UI patterns for "handoffs" and "human-in-the-loop" are relevant.
*   **DevSpace / Tilt:** Tools for managing multi-service dev environments (Kubernetes focus, but the "dashboard for multiple running streams" UX is identical).

## 6. Platform Concerns (Windows)

*   **Console API:** Windows Console architecture is distinct from PTYs. If you go the "Wrapper" route, use the **ConPTY** API (introduced in Windows 10/Server 2019). It provides a pseudo-console infrastructure similar to Unix PTYs.
*   **Named Pipes:** For IPC, Windows Named Pipes are the standard and work well.
*   **Pathing:** Ensure all adapters handle `\` vs `/` robustly.
*   **Signals:** Windows does not have full POSIX signals (SIGINT, SIGTERM behave differently). "Pausing" a process (SIGSTOP/SIGCONT) is not natively supported in the same way; you might need to use `DebugActiveProcess` or specialized Job Objects to freeze agents.

## 7. Additions to the Problem Space

*   **Global Knowledge/Context:**
    *   *Idea:* A "Tower Scratchpad". If Agent A learns that "API endpoint X is deprecated", how does Agent B know? Tower could provide a shared `memory.md` that all agents can read/write to.
*   **Work/IO Throttling:**
    *   If 5 agents try to run `npm install` or `dotnet build` simultaneously, the machine will crawl. Tower needs a "Semaphore" system. "Max 2 concurrent heavy tasks."
*   **Drift Detection:**
    *   Visual indicator if a file currently open in Agent A's context was just modified by Agent B. "Warning: Context Stale."

## Summary Recommendation

The product vision is excellent and necessary. The "Side-Channel" architectural constraint is the main risk. I strongly recommend pivoting to a **"Transparent Wrapper"** architecture (e.g., `tower run <cmd>`) to solve the Stdin/Interception problem robustly while maintaining the user's workflow.
