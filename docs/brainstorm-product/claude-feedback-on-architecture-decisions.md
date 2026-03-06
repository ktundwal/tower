# Claude Opus 4.6 Feedback on docs/engineering/architecture-decisions.md

Reviewed by Claude Opus 4.6 (1M context) at Kapil's request after GPT 5.4 produced the architecture decisions doc.

## What's strong

### Architecture decisions (sections 1-10): solid and ready to build against

- **"V1 batch approval limited to read-only actions"** - Right constraint. Avoids the "batch approve rm -rf" nightmare. Expand scope later based on real usage patterns, not speculation.
- **"Degrades to unknown after crashes rather than guessing"** - Correct. A control plane that lies about confidence is worse than no control plane.
- **Observed vs managed as a product-visible distinction** - This is the decision that makes Tower honest. Most tools would hide this behind "coming soon." Tower should wear it as a feature.
- **"Terminal owns input by default"** - Preserves the core requirement that focused work stays untouched.
- **Event-first with polling for reconciliation** - Right hybrid. Pure push is fragile, pure poll is laggy.

No disagreements on the 10 architecture decisions. They're defensible and internally consistent.

## What's weak

### 1. The doc is 3-4 documents smashed into one

Architecture decisions, PRD, marketing copy, tweet drafts, and visual concepts are all in one 910-line file. These serve different audiences and have different lifespans:

- Architecture decisions: for builders, long-lived
- PRD: for product alignment, medium-lived
- Marketing/visuals/tweets: for launch, will change completely once the product exists

Recommendation: split into separate files. Architecture decisions should stand alone and be easy to reference during implementation.

### 2. Marketing content is premature

Launch tweets, social cards, GIF storyboards, "retweet-friendly one-liners", and a homepage hero mockup for a project with zero lines of code. This is low-cost for an LLM to generate and feels productive, but:

- Marketing copy will change once the product is real and you've used it yourself
- The visuals describe features that haven't been designed yet
- Time spent refining copy for a nonexistent product is time not spent on the hard engineering problems

Recommendation: delete or move to a `docs/product-notes.md` parking lot. Revisit when there's a working prototype to screenshot.

### 3. The PRD mostly restates the README

The "Draft product requirements document" section echoes the README and architecture decisions. Now three places say the same things (README, architecture decisions, PRD section). Three places to keep in sync, three places that will drift.

Recommendation: keep the README as the canonical product description. Keep `docs/engineering/architecture-decisions.md` as the canonical technical decisions. Don't duplicate in a third format until there's a team that needs a separate PRD.

### 4. Feature backlog ranking is speculative

P0/P1/P2 ranking without any usage data or a working prototype is guessing. The features are reasonable, but:

- "Attention-ranked inbox" is labeled P0 in the backlog but V1.5 in the roadmap two sections earlier. Internal inconsistency.
- Rankings will shift the moment you use Tower for one real day of work.
- A speculative backlog can create false confidence about what matters.

Recommendation: keep the feature list as an unranked idea pool. Rank after v1 prototype exists and you've used it.

### 5. Self-critique section is performative

GPT critiqued its own pitch, then rewrote it based on the critique, then explained why the rewrite is better. That's ~200 lines of the model doing a victory lap. One strong pitch is better than a critique-rewrite-meta-analysis cycle.

Recommendation: keep the final version of the pitch. Delete the critique and rewrite process. The reasoning is interesting but doesn't belong in a project doc.

### 6. Missing: the hard technical design

The doc answered all 10 architecture questions but stopped before the actual engineering problems. What's missing and what matters next:

- **ConPTY/PTY wrapper design**: How does `tower run claude` actually work? Process spawning, handle management, passthrough mode, approval interception mode, handoff between cockpit and terminal.
- **Event schema with payloads**: Event names are listed (`session.discovered`, `approval.requested`, etc.) but not their concrete shapes. What fields? What types? What's required vs optional?
- **Session discovery on Windows**: How does Tower find running sessions? Process tree walking? Hook registration? Window enumeration? Each has different reliability and permissions requirements.
- **Approval detection from Claude Code**: How does Tower know Claude is asking for approval? Parse the PTY output stream for specific patterns? Hook-mediated structured event? This is the most fragile part of the system.
- **Language and framework choice**: Still open. Python (textual), Go (bubbletea), TypeScript (ink). This unblocks everything.
- **Adapter interface as actual code**: The interface is described in prose. It needs to be a real type definition to validate that it works across tools.

These are the decisions that determine whether Tower is buildable. The marketing can wait.

## Recommended next steps

1. **Split the doc**: Architecture decisions standalone. Product/marketing notes in a separate file.
2. **Pick language and framework**: This unblocks all implementation work.
3. **Design the Claude Code managed adapter**: PTY wrapper, event detection, approval interception. This is the hardest v1 engineering problem.
4. **Prototype the cockpit TUI**: Even a bare-bones display showing 3 fake sessions with state transitions would validate the UX faster than more docs.
5. **Defer marketing**: Revisit when there's something to screenshot.

## Bottom line

The 10 architecture decisions are the right ones. The product vision is clear. What's needed now is less prose and more engineering design. The next doc should be a technical design for the Claude Code managed adapter, not more marketing.

## GPT-5.4 response and actions taken

Added after reviewing the feedback and cleaning up the docs.

### 1. The doc was too many documents smashed together

**Response:** Agree.

**Action taken:**

- `docs/engineering/architecture-decisions.md` was slimmed down to the durable architecture record only.
- It now stops at the 10 decisions plus the net result.

### 2. Marketing content was premature in the architecture doc

**Response:** Mostly agree.

**Action taken:**

- marketing, launch, and visual brainstorming were removed from `docs/engineering/architecture-decisions.md`

**Pushback:**

- early marketing exploration was still useful to sharpen the wedge, demo moments, and proof points
- the mistake was not doing the thinking; the mistake was letting that content live in the core architecture record

### 3. The PRD mostly restated the README

**Response:** Agree.

**Action taken:**

- PRD-style product description was removed from the architecture doc
- README remains the canonical public product description

### 4. Feature backlog ranking was too speculative

**Response:** Mostly agree.

**Action taken:**

- speculative ranking was removed from the durable repo architecture doc

**Pushback:**

- value-first hypotheses are still useful in planning
- they just should not be presented as durable repo truth before prototype usage exists

### 5. The self-critique section was performative

**Response:** Mostly agree.

**Action taken:**

- the critique/rewrite/meta-analysis cycle was removed from the architecture doc

**Pushback:**

- the exercise was useful once to sharpen the pitch
- it was not useful to preserve that reasoning chain in a long-lived project document

### 6. The missing piece is hard technical design

**Response:** Strong agree.

**Action taken:**

- the session execution roadmap now includes a concrete foundation spec in `plan.md`
- the foundation spec locks Go + Bubble Tea + SQLite + Next.js/Vercel and defines repo layout, identity model, event envelope, state model, approval policy, audit model, and demo harness requirements
- the next recommended design artifact is the Claude managed adapter technical design: PTY/ConPTY lifecycle, approval detection, event payloads, discovery and recovery behavior, and concrete adapter types

### Net takeaway

We agree on the main correction:

- keep `docs/engineering/architecture-decisions.md` lean and implementation-facing
- keep README as the public product description
- move from architecture prose into hard technical design next

Where I push back is narrower:

- early product and launch exploration was not wasted effort
- it helped clarify the wedge and demo loops
- it just belonged outside the canonical architecture doc
