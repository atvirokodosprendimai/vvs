---
tldr: Replace single shared NATS token with per-user credentials — portal locked to isp.rpc.portal.> only
status: active
---

# Plan: NATS Subject Authorization — Per-User Permissions

## Context

Consilium 2026-04-19 top priority #1. Portal binary on public VPS shares one NATS token with core — can eavesdrop all `isp.>` subjects and publish to any subject.

- Related memory: [[project_nats_subject_auth]]
- Consilium backlog: [[project_consilium_backlog_priority]] — #1

## Phases

### Phase 1 — Config + Embedded Server — status: open

1. [ ] Add `NATSCorePassword` + `NATSPortalPassword` to config struct; deprecate `NATSAuthToken`
   - config likely in `internal/app/config.go` or similar
   - read from env: `VVS_NATS_CORE_PASSWORD`, `VVS_NATS_PORTAL_PASSWORD`
   - keep backward-compat: if only `NATSAuthToken` set, use it for core password (migration path)

2. [ ] Update `StartEmbedded` to accept user list instead of single token
   - file: `internal/infrastructure/nats/embedded.go`
   - replace `opts.Authorization = authToken[0]` with `opts.Users = []*natsserver.User{...}`
   - core user: pub+sub `isp.>` + `_INBOX.>`
   - portal user: pub+sub `isp.rpc.portal.>` + `_INBOX.>` only
   - function signature: `StartEmbedded(listenAddr string, corePass, portalPass string) (*natsserver.Server, *nats.Conn, error)`

3. [ ] Update `builder.go` to pass both passwords when calling `StartEmbedded`
   - core connects with `nats.UserInfo("core", cfg.NATSCorePassword)` + `nats.InProcessServer(ns)`

### Phase 2 — Portal Connect + Tests — status: open

1. [ ] Update `cmd/portal/main.go` to connect with portal credentials
   - `nats.Connect(cfg.NATSAddr, nats.UserInfo("portal", cfg.NATSPortalPassword))`
   - portal binary needs `NATSPortalPassword` env var

2. [ ] Write integration test: portal connection rejects unauthorized subjects
   - file: `internal/infrastructure/nats/embedded_test.go` (extend existing)
   - start embedded with two users
   - portal conn: subscribe `isp.invoice.finalized` → expect error / no messages
   - portal conn: publish `isp.service.suspended` → expect permission denied
   - core conn: subscribe `isp.>` → still works

3. [ ] Update `deploy/` env templates with new env var names
   - `VVS_NATS_CORE_PASSWORD`, `VVS_NATS_PORTAL_PASSWORD`

## Verification

```bash
go test ./internal/infrastructure/nats/... -run TestEmbedded
go build ./cmd/vvs-core/ ./cmd/portal/
# Start both binaries; check NATS logs show per-user auth
# Attempt to subscribe isp.invoice.finalized from portal conn → permission denied
```

## Adjustments

## Progress Log
