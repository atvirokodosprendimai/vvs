---
tldr: Add Makefile targets to build and run all VVS binaries (core + portal) in one command for local dev
status: completed
---

# Plan: Makefile dev spin-up — all services in one command

## Context

Three binaries (two now, stb planned):
- `cmd/server` → `bin/vvs-core` — admin backend, :8080, embedded NATS :4222
- `cmd/portal` → `bin/vvs-portal` — customer portal, :8081, connects to NATS localhost:4222
- `cmd/stb` → `bin/vvs-stb` — IPTV STB API (not built yet, placeholder)

Local dev: all on localhost, no WireGuard, no NATS auth token needed.
Portal needs NATS URL pointing to core's embedded server.

## Phases

### Phase 1 — Update Makefile — status: completed

1. [x] Add `build-all`, `run-all`, `run-core`, `run-portal` targets to Makefile
   - `build-all`: build vvs-core + vvs-portal (generate templ first)
   - `run-all`: build-all then launch both with `&`, trap SIGINT to kill both
   - `run-core` / `run-portal`: individual targets with dev env
   - Keep existing `build` / `run` targets unchanged for backward compat
   - Local dev env: `VVS_DB_PATH=./data/dev.db`, `NATS_LISTEN_ADDR=127.0.0.1:4222`,
     `NATS_URL=nats://127.0.0.1:4222`, `PORTAL_INSECURE_COOKIE=true`
   - => `sleep 1` between core+portal starts so embedded NATS is ready before portal connects
   - => `DEV_DB/DEV_ADDR/DEV_PORTAL/DEV_NATS` all overrideable via env
   - => `trap 'kill %1 %2 2>/dev/null; echo "stopped"' INT TERM` — clean Ctrl-C
   - => verified: `curl localhost:8080/login` → 200, `curl localhost:8081/portal/auth` → 200
   - => commit: 52ad85e

## Verification

```bash
make build-all   # both binaries compile, no error
make run-all     # core on :8080, portal on :8081; Ctrl-C kills both cleanly
curl localhost:8080/login  # → 200
curl localhost:8081/portal/auth  # → 200
```

## Adjustments

## Progress Log

- 2026-04-19: Phase 1 complete. All targets added, smoke-tested, committed (52ad85e).
