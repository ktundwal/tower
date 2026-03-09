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

## Go Conventions

- Module: `tower`
- Go 1.22
- Standard library preferred over third-party unless there's a strong reason
- Table-driven tests using `t.Run` subtests
- Test files colocated: `foo.go` → `foo_test.go`
- No `interface{}` or `any` when concrete types are known
- Error messages: lowercase, no punctuation, descriptive
- Package names: short, lowercase, no underscores

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
