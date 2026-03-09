# /review — Cross-Model Review with GPT 5.3 Codex

Run this after any meaningful code or design change. Writes context to a temp file and invokes Copilot CLI with GPT 5.3 Codex for independent review.

## When to Use

- New package or module
- Design decision with tradeoffs
- Non-obvious logic (concurrency, state machines, error handling)
- Any code touching security (auth tokens, hook validation)
- Changes to contracts or interfaces

## When to Skip

- Formatting-only changes
- Comment updates
- Trivial renames

## Review Protocol

1. Write a review context file to `/tmp/tower-review-<timestamp>.md` containing:
   - **What changed**: File paths and brief description
   - **Why**: The design decision or problem being solved
   - **Key code**: The actual diff or new code (use `git diff` output)
   - **Concerns**: Anything you're unsure about
   - **Architecture constraints**: Relevant locked decisions from `/tower-arch`

2. Invoke the review:
   ```bash
   copilot --model gpt-5.3-codex -p "Read /tmp/tower-review-<timestamp>.md and perform a thorough code review. Focus on: correctness, adherence to Go conventions, error handling, test coverage gaps, security issues, and scope creep. Be direct — flag problems, don't hedge." --allow-all-tools --allow-all-paths
   ```

3. Address feedback before committing. If the reviewer flags a real issue, fix it. If you disagree, note why in the commit message.

## Review Context Template

```markdown
# Tower Code Review

## What changed
- `internal/foo/bar.go`: Added X
- `internal/foo/bar_test.go`: Tests for X

## Why
[1-2 sentences on the design decision]

## Diff
[paste git diff or key code sections]

## Concerns
- [anything you're unsure about]

## Architecture constraints
- [relevant locked decisions that apply]
```
