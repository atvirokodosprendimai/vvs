---
tldr: If a .env file exists in the working directory, load it into the environment before CLI flag parsing
status: active
---

# Plan: Load .env file if present

## Context

App reads all config from env vars (via `cli.EnvVars(...)` in `cmd/server/main.go`).
No `.env` support currently — dev must export vars manually or wrap in a shell script.

## Phases

### Phase 1 - Implement - status: open

1. [ ] Add `github.com/joho/godotenv` dependency
   - `go get github.com/joho/godotenv`

2. [ ] Load `.env` in `main()` before CLI runs
   - call `godotenv.Load()` (no-op if file absent — returns error only if file exists but is malformed)
   - log a line when a `.env` file is successfully loaded
   - add `.env` to `.gitignore` (keep secrets out of git)

## Verification

1. Create `.env` with `VVS_DB_PATH=./test.db`
2. Run `./vvs server` — server uses `test.db` without any `export`
3. Delete `.env` — server starts fine (no error)
4. `go build ./...` passes

## Adjustments

## Progress Log
