# Tower Progress

## Current Slice

Slice 1: Tower daemon HTTP endpoint + hook config injection

## Status

Not started. Setup complete — ready for implementation.

## What's Done

- Architecture decisions locked (`docs/engineering/`)
- Contract types and adapter interface (`internal/contracts/`)
- Bootstrap CLI with `tower run claude` and `tower-demo` entrypoints
- Demo fixture harness (`test/fixtures/demo/six-session-mixed.json`)
- Event envelope schema (`schemas/event-envelope-v1.schema.json`)
- Memory-backed repository (SQLite deferred)
- Project CLAUDE.md, skills (`/tower-arch`, `/tower-scope`, `/review`), settings
- Go 1.26.1 installed

## What's Next

1. Daemon HTTP server (`internal/daemon/`) — TDD: write test for health endpoint first
2. Hook endpoints per design doc section 6.4
3. Hook config injection (`internal/adapters/claude/hooks.go`)
4. Wire `tower run claude` to start daemon + inject hooks + exec claude

## Blockers

None.

## Decisions

- Go module version in go.mod says 1.22 but Go 1.26.1 is installed. May need to update go.mod.
