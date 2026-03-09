Session is ending. Do the following in order:

## 1. Verify
Run `go test ./...` and `go vet ./...`. Fix anything broken.

## 2. Session Retro
Create `docs/retro/sessions/YYYY-MM-DD-<short-slug>.md` with this exact structure:

```markdown
# Session Retro: <date> — <what the session was about>

## What got done
- [concrete deliverables with file paths]

## What worked
- [patterns, tools, approaches that were effective]

## What didn't work
- [wasted time, wrong turns, failed approaches — be specific]

## Where the agent drifted
- [hallucinations caught, scope creep attempted, assumptions made without evidence]
- [times the agent over-engineered, under-tested, or ignored CLAUDE.md rules]
- If none, say "None detected" — don't fabricate problems to look thorough.

## Honest assessment
- [rate the session: was this productive or spinning wheels?]
- [what percentage of time was actual progress vs. overhead/confusion?]

## Learnings to keep
- [specific rules or patterns to add to CLAUDE.md]
- [things the next session should know that aren't captured elsewhere]
```

Be raw. Don't soften. If the session was unproductive, say so and say why. If the agent made a mistake, name the exact mistake and what triggered it. The point is to get better, not to look good.

## 3. Compound the learnings
If the retro identified new rules or corrections, add them to CLAUDE.md now. This is the compounding step — learnings that don't make it into CLAUDE.md are lost.

## 4. Update PROGRESS.md
Current slice, what got done, what's next, blockers, decisions made.

## 5. Commit everything
Stage retro, PROGRESS.md, CLAUDE.md updates, and any code changes. Single commit.

## 6. One-liner
Show where the next session picks up.