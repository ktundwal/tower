# Tower — Project Rules

## What Tower Is

Local control plane for developers running multiple AI coding agents on one machine. Go binary, Bubble Tea TUI, SQLite persistence. See `docs/` for architecture.

## Hard Rules

- **TDD first**: Write a failing test before writing implementation code. Run the test, confirm it fails for the right reason. Then write the minimum code to pass. No exceptions.
- **One task per sub-agent**: When using the Agent tool, each agent gets exactly one focused task. Never bundle multiple concerns.
- **Cross-model review required**: Every meaningful design decision, new package, or non-trivial logic change must be reviewed via `/review` before committing. Trivial formatting or comment changes are exempt.
- **No hallucinating hook payloads**: Never assume Claude Code hook payload structure. Reference captured fixtures in `test/fixtures/` or the design doc at `docs/engineering/claude-managed-adapter-design.md` section 4.2–4.4.
- **No scope creep**: Before writing code, check if the work is in v1 scope. Run `/tower-scope` if unsure. If it's deferred or out of scope, stop and say so.
- **Plan mode for new slices**: Use plan mode (`shift+tab`) before starting any new implementation slice. Get approval before writing code.
- **Tests must pass before committing**: `go test ./... && go vet ./...` must pass. Do not commit broken code.

## Learned Rules
- Commands (`.claude/commands/`) are user-invoked via `/name`. Skills (`.claude/skills/`) are agent-loaded context. Don't mix them up.
- Default to the simpler option. Let the user ask for more complexity if needed.
- Go binary lives at `/c/Program Files/Go/bin/go.exe` — prepend to PATH in bash: `export PATH="/c/Program Files/Go/bin:$PATH"`
- `PreCommit` is NOT a valid Claude Code hook event. To gate `git commit`, use `PreToolUse` with matcher `Bash(git commit*)`.
- **CLAUDECODE env var**: When spawning Claude as a subprocess (smoke tests, managed launch), strip `CLAUDECODE` from the environment. Otherwise Claude rejects with "nested session" error.
- **MINGW64 paths**: Always use forward slashes in bash instructions (`/c/tmp/` not `C:\tmp\`). Backslashes get mangled (`\t` = tab).
- **Claude HTTP hooks confirmed**: `type: "http"` works in `.claude/settings.local.json`. Claude blocks on sync hooks (PreToolUse, PermissionRequest). `allowedEnvVars` is required for env var expansion in headers.
- **Hook payload fixtures**: Live-captured payloads in `test/fixtures/hooks/`. Always reference these for payload structure — never guess. Regression tests in `internal/daemon/regression_test.go` replay them.
- **Smoke test**: `go test -tags smoke -run TestSmoke -count=1 -v ./test/smoke/` — runs real Claude (Haiku) against a live daemon. Requires `claude` on PATH + auth. Not part of normal `go test ./...`.

## Go Conventions

- Module: `tower`
- Go 1.22
- Standard library preferred over third-party unless there's a strong reason
- Table-driven tests using `t.Run` subtests
- Test files colocated: `foo.go` → `foo_test.go`
- No `interface{}` or `any` when concrete types are known
- Error messages: lowercase, no punctuation, descriptive
- Package names: short, lowercase, no underscores

## Session Protocol

- **Start**: Read `PROGRESS.md` before starting work. Scan recent retros in `docs/retro/sessions/` if debugging a recurring issue.
- **End**: Run `/wrap`. This updates PROGRESS.md, writes a session retro, and compounds learnings into this file.

## Architecture Reference

- Load `/tower-arch` before any design or implementation work
- Load `/tower-scope` before starting any new slice
- Locked design docs live in `docs/engineering/` — these are source of truth
- `internal/contracts/` defines all shared types — read before adding new types

## Verification

```bash
go test ./...
go vet ./...
```

## Commit Convention

Conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:` with optional `(scope):`
No co-author lines. No AI attribution in commits.
