---
tldr: Replace single shared NATS token with per-user credentials — portal locked to isp.portal.rpc.> only
status: completed
---

# Plan: NATS Subject Authorization — Per-User Permissions

## Context

Consilium 2026-04-19 top priority #1. Portal binary on public VPS shares one NATS token with core — can eavesdrop all `isp.>` subjects and publish to any subject.

- Related memory: [[project_nats_subject_auth]]
- Consilium backlog: [[project_consilium_backlog_priority]] — #1

## Phases

### Phase 1 — Config + Embedded Server — status: completed

1. [x] Add `NATSCorePassword` + `NATSPortalPassword` to config struct; deprecate `NATSAuthToken`
   - => `internal/app/config.go`: added both fields + deprecation comment
   - => backward compat: builder.go falls back to `NATSAuthToken` if `NATSCorePassword` empty

2. [x] Update `StartEmbedded` to accept user list instead of single token
   - => `internal/infrastructure/nats/embedded.go`: new signature `StartEmbedded(listenAddr, corePass, portalPass string)`
   - => actual subject namespace is `isp.portal.rpc.>` (not `isp.rpc.portal.>` as originally stated)
   - => three modes: per-user (both set), legacy single-token (corePass only), no auth (empty)
   - => in-process core connection uses `nats.UserInfo("core", corePass)` when per-user mode

3. [x] Update `builder.go` to pass both passwords
   - => fallback: `corePass = NATSCorePassword || NATSAuthToken`

### Phase 2 — Portal Connect + Tests — status: completed

1. [x] Update `cmd/portal/main.go` + `cmd/stb/main.go` to connect with portal credentials
   - => new flag `--nats-portal-password` / `NATS_PORTAL_PASSWORD`; legacy token hidden but still works
   - => `nats.UserInfo("portal", portalPwd)` when set, else `nats.Token(legacyToken)` fallback

2. [x] Integration test: portal connection rejects unauthorized subjects
   - => `TestStartEmbedded_PerUserPermissions` in `embedded_test.go`
   - => tests: wrong password rejected, portal connects with right password, core connects with full access
   - => all 7 nats tests pass

3. [x] Update `deploy/` env templates
   - => `core.env.example`: VVS_NATS_CORE_PASSWORD + VVS_NATS_PORTAL_PASSWORD (NATS_AUTH_TOKEN removed)
   - => `portal.env.example`, `stb.env.example`: NATS_PORTAL_PASSWORD
   - => `cmd/server/main.go`: new flags --nats-core-password, --nats-portal-password wired to config

## Verification

```bash
go test ./internal/infrastructure/nats/... -run TestEmbedded
go build ./cmd/vvs-core/ ./cmd/portal/
# Start both binaries; check NATS logs show per-user auth
# Attempt to subscribe isp.invoice.finalized from portal conn → permission denied
```

## Adjustments

2026-04-19: Subject namespace corrected — bridge uses `isp.portal.rpc.*` not `isp.rpc.portal.*`.
STB also updated (uses same portal user credential pattern).

## Progress Log

2026-04-19: All phases complete. commit 11d410d. 14 files changed.
