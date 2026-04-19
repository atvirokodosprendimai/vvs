---
tldr: Add Makefile targets to build and run all VVS binaries (core + portal) in one command for local dev
status: active
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

### Phase 1 — Update Makefile — status: open

1. [ ] Add `build-all`, `run-all`, `dev-all` targets to Makefile
   - `build-all`: build vvs-core + vvs-portal (generate templ first)
   - `run-all`: build-all then launch both with `&`, trap SIGINT to kill both
   - `dev-core` / `dev-portal`: individual air watchers for each binary
   - Keep existing `build` / `run` targets unchanged for backward compat
   - Local dev env: `VVS_DB_PATH=./data/dev.db`, `NATS_LISTEN_ADDR=127.0.0.1:4222`,
     `NATS_URL=nats://127.0.0.1:4222`, `PORTAL_INSECURE_COOKIE=true`

## Verification

```bash
make build-all   # both binaries compile, no error
make run-all     # core on :8080, portal on :8081; Ctrl-C kills both cleanly
curl localhost:8080/login  # → 200
curl localhost:8081/portal/auth  # → 200
```

## Adjustments

## Progress Log
