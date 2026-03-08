# Tower Docs Map

This file explains what each Markdown file is for and which ones should drive implementation.

## Source-of-truth rule

If you are building Tower, prefer the repository docs over any hidden Copilot session scratchpad.

- Repo-visible source of truth for execution: `docs\requirements\roadmap.md`
- Repo-visible source of truth for product boundary: `docs\engineering\architecture-decisions.md`
- Repo-visible source of truth for technical foundation: `docs\engineering\foundation-spec.md`
- Repo-visible source of truth for the hardest v1 slice: `docs\engineering\claude-managed-adapter-design.md`

Hidden Copilot session files like `plan.md`, checkpoints, and SQL todo state are working artifacts, not canonical project docs.

## Recommended reading order for a new implementation session

1. `README.md`
2. `docs\requirements\v1-scope.md`
3. `docs\engineering\architecture-decisions.md`
4. `docs\engineering\foundation-spec.md`
5. `docs\engineering\claude-managed-adapter-design.md`
6. `docs\requirements\roadmap.md`

Read the brainstorm files only if you need historical context or want to revisit why a decision was made.

## Markdown file purposes

### Root

- `README.md`  
  Public-facing overview of Tower: problem, value proposition, repo layout, and key docs.

### docs\engineering

- `docs\engineering\architecture-decisions.md`  
  Canonical durable architecture decisions. This is the shortest statement of the rules the implementation should not violate.

- `docs\engineering\foundation-spec.md`  
  Concrete technical foundation for v1: stack, repo layout, data layout, identity model, event envelope, state model, and core contracts.

- `docs\engineering\claude-managed-adapter-design.md`  
  Deep implementation design for `tower run claude`: managed runtime ownership, PTY/ConPTY boundary, approval detection, event payloads, recovery, and trust constraints.

- `docs\engineering\claude-code-review-round2.md`  
  Critical review of the first implementation pass plus the response/actions taken. Useful for understanding why execution was reprioritized toward a runtime-first spike.

### docs\requirements

- `docs\requirements\v1-scope.md`  
  Execution-facing statement of what v1 promises, what is in scope, what is not, and the support matrix.

- `docs\requirements\roadmap.md`  
  Repo-visible execution plan and current sequencing. This should answer "what next?" for the team.

### docs\brainstorm-product

- `docs\brainstorm-product\session-context.md`  
  Long-form discovery transcript and original framing of the problem.

- `docs\brainstorm-product\research.md`  
  Research notes on prior art and integration surfaces such as Entire.io and Claude hooks.

- `docs\brainstorm-product\review-gpt.md`  
  GPT architectural review of the concept and its trust/control-plane implications.

- `docs\brainstorm-product\review-gemini.md`  
  Gemini architectural review of the concept.

- `docs\brainstorm-product\gpt-questions.md`  
  The 10 forcing questions that drove the architecture decisions.

- `docs\brainstorm-product\copilot-prompt.md`  
  Bootstrap prompt for bringing a fresh Copilot session up to speed on the project.

- `docs\brainstorm-product\claude-feedback-on-architecture-decisions.md`  
  Claude Opus critique of the architecture doc plus the response and actions taken.

These files are mainly historical context and decision provenance. They are useful, but they are not the primary implementation contract.

## Adjacent repo docs outside `docs\`

- `schemas\README.md`  
  Explains the purpose of versioned schemas for normalized data contracts.

- `test\fixtures\README.md`  
  Explains the fixture families used for demo replay, approval parsing, conflicts, parked sessions, and summaries.

## Practical guidance

- If a file in `docs\brainstorm-product\` disagrees with a file in `docs\engineering\` or `docs\requirements\`, follow the engineering/requirements doc.
- If the hidden session scratchpad disagrees with a repo doc, update the repo doc and treat the repo doc as canonical.
- If new implementation decisions are made, prefer updating the smallest canonical doc that owns that concern rather than adding more scattered notes.
