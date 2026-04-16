---
tldr: If a .env file exists in the working directory, load it into the environment before CLI flag parsing
status: completed
---

# Plan: Load .env file if present

## Context

App reads all config from env vars (via `cli.EnvVars(...)` in `cmd/server/main.go`).
No `.env` support currently — dev must export vars manually or wrap in a shell script.

## Phases

### Phase 1 - Implement - status: completed

1. [x] Add `github.com/joho/godotenv` dependency
   - => `github.com/joho/godotenv v1.5.1`

2. [x] Load `.env` in `main()` before CLI runs
   - => `godotenv.Load()` called at top of `main()` — no-op if file absent
   - => logs "Loaded config from .env" on success
   - => `.gitignore` already had `.env` and `.env.*` entries

## Verification

1. Create `.env` with `VVS_DB_PATH=./test.db`
2. Run `./vvs server` — server uses `test.db` without any `export`
3. Delete `.env` — server starts fine (no error)
4. `go build ./...` passes

## Adjustments

## Progress Log

- 2604161815 Phase 1 complete — godotenv added, loaded at top of main()
