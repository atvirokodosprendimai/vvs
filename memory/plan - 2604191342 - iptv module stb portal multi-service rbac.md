---
tldr: IPTV module in vvs-core + STB self-service portal binary; per-user module scoping so IPTV staff see only IPTV
status: active
---

# Plan: IPTV Module + STB Portal (Multi-Service RBAC)

## Context

- Arch spec: [[spec - architecture - system design and key decisions]]
- Portal split pattern: [[spec - portal - customer self-service access]]
- Memory: [[Core / Portal Deployment Split]] — established vvs-core+vvs-portal pattern to follow
- Memory: [[Per-Role Module Permissions]] — InjectModulePermissions+RequireModuleAccess already built

### The multi-service insight

VVS is growing into a multi-service ISP platform. A company may run both ISP and IPTV services.
Staff roles must be scoped to their vertical:

```
Admin (all)          → sees everything
ISP operator         → sees Customer/Invoice/Service/Network only
IPTV operator        → sees IPTV module only
Billing staff        → sees Invoice/Payment only
```

This is already supported by the **per-role module permissions** system (`role_module_permissions` table).
No new RBAC infra needed — just add `ModuleIPTV` to the module registry.

### Deployment: three binaries

```
[ Office / NATed LAN ]
┌────────────────────────────────────────────┐
│  vvs-core (cmd/server)                     │
│  - all admin modules incl. IPTV dashboard  │
│  - SQLite + NATS :4222 (WireGuard)         │
└────────────────────────────────────────────┘
          ▲ NATS RPC               ▲ NATS RPC
          │ isp.portal.rpc.*       │ isp.stb.rpc.*
          │                        │
[ Public VPS A ]           [ Public VPS B ]
┌──────────────────┐       ┌────────────────────┐
│  vvs-portal      │       │  vvs-stb           │
│  /portal/*       │       │  /stb/*            │
│  invoice PDF     │       │  STB dashboard     │
│  magic-link auth │       │  channel lineup    │
└──────────────────┘       └────────────────────┘
```

### IPTV domain concepts

| Concept | What |
|---------|------|
| **Channel** | Single broadcast stream — name, logo URL, stream URL, category, EPG |
| **Package** | Bundle of channels with a price — customers subscribe to packages |
| **Subscription** | Customer ↔ Package link — status (active/suspended/cancelled), dates |
| **STB** | Set-Top Box — MAC address, model, firmware, serial, customer assignment |
| **STBToken** | Magic-link session token for STB portal (own table, same SHA-256 hash pattern) |

### NATS RPC for STB portal (`isp.stb.rpc.*`)

| Subject | Request | Reply |
|---------|---------|-------|
| `isp.stb.rpc.token.validate` | `{hash}` | `{customerID, expiresAt}` |
| `isp.stb.rpc.subscription.get` | `{customerID}` | `{subscription, package, channels[]}` |
| `isp.stb.rpc.stbs.list` | `{customerID}` | `{stbs[]}` |
| `isp.stb.rpc.channels.list` | `{packageID}` | `{channels[]}` |
| `isp.stb.rpc.customer.get` | `{customerID}` | `{id, CompanyName, Email}` |
| `isp.stb.rpc.stb.link` | `{customerID, mac}` | `{}` — customer links a new STB |

---

## Phase 1 — Spec — status: open

Write the IPTV spec before any code.

1. [ ] `/eidos:spec` — write `eidos/spec - iptv - channel package stb management.md`
   - Document domain model: Channel, Package, Subscription, STB, STBToken
   - Document IPTV admin routes in vvs-core
   - Document STB portal routes in cmd/stb
   - Document NATS RPC subjects (table above)
   - Document RBAC: ModuleIPTV constant, admin UI permissions page

---

## Phase 2 — IPTV UI Mockup (templ, static) — status: open

Deliver visible screens first — no real DB queries yet, all hardcoded stub data.
Goal: stakeholder can see and react to the IPTV admin and STB portal screens.

### IPTV admin screens (in vvs-core templ)

2. [ ] Create `internal/modules/iptv/adapters/http/templates.templ` with stub pages:

   **IPTVDashboardPage** — overview stats + quick actions
   ```
   ┌─────────────────────────────────────────────────────────┐
   │  IPTV                                          [+ Add]  │
   ├──────────┬──────────┬──────────┬───────────────────────┤
   │ Channels │ Packages │  Active  │  Active STBs          │
   │   142    │    8     │   Subs   │      1,204            │
   │          │          │   891    │                       │
   ├──────────┴──────────┴──────────┴───────────────────────┤
   │ Recent activity                                         │
   │ • New STB linked: 00:1A:2B:3C:4D:5E — Petras Petraitis │
   │ • Package "Premium" subscription activated — UAB Geros  │
   │ • Channel "LNK" stream URL updated                      │
   └─────────────────────────────────────────────────────────┘
   ```

   **IPTVChannelListPage** — channel CRUD
   ```
   ┌────────────────────────────────────────────────────────┐
   │ Channels                          [Search...] [+ Add]  │
   ├──────┬────────────────┬──────────┬───────────┬────────┤
   │ Logo │ Name           │ Category │ Packages  │        │
   ├──────┼────────────────┼──────────┼───────────┼────────┤
   │  ▶   │ LNK            │ National │ Basic, HD │ [Edit] │
   │  ▶   │ TV3            │ National │ Basic     │ [Edit] │
   │  ▶   │ BTV            │ Regional │ Basic     │ [Edit] │
   │  ▶   │ ESPN HD        │ Sports   │ Premium   │ [Edit] │
   └──────┴────────────────┴──────────┴───────────┴────────┘
   ```

   **IPTVPackageListPage** — package management
   ```
   ┌────────────────────────────────────────────────────────┐
   │ Packages                                    [+ New]    │
   ├──────────────┬────────┬────────────┬───────────────────┤
   │ Name         │ Price  │ Channels   │ Subscribers       │
   ├──────────────┼────────┼────────────┼───────────────────┤
   │ Basic        │  9.99  │     45     │      612          │
   │ Standard     │ 14.99  │     90     │      201          │
   │ Premium HD   │ 24.99  │    142     │       78          │
   └──────────────┴────────┴────────────┴───────────────────┘
   ```

   **IPTVSubscriberListPage** — customer subscriptions
   **IPTVSTBListPage** — device inventory

3. [ ] Register stub routes in `internal/modules/iptv/adapters/http/handlers.go`
   - `GET /iptv/dashboard` → IPTVDashboardPage (stub data)
   - `GET /iptv/channels` → IPTVChannelListPage (stub data)
   - `GET /iptv/packages` → IPTVPackageListPage (stub data)
   - `GET /iptv/subscribers` → IPTVSubscriberListPage (stub data)
   - `GET /iptv/stbs` → IPTVSTBListPage (stub data)
   - `ModuleName() authdomain.Module` — return `ModuleIPTV`

4. [ ] Add `ModuleIPTV` to `internal/modules/auth/domain/module.go`
   - Const `ModuleIPTV Module = "iptv"`

5. [ ] Wire stub routes in `internal/app/app.go` (no DB yet)
   - `iptvRoutes := iptvhttp.NewHandlers()`
   - `moduleRoutes = append(moduleRoutes, iptvRoutes)`

6. [ ] Add IPTV nav entry in sidebar
   - Group: "Services" (new group between Finance and Network)
   - `iptv-icon.svg` or use a simple play-button SVG

### STB portal screens (cmd/stb templ)

7. [ ] Create `cmd/stb/templates/` (or `internal/modules/iptv/adapters/stbportal/`) with:

   **STBPortalDashboard** — customer's current subscription + devices
   ```
   ┌────────────────────────────────────────────────────────┐
   │  IPTV — Petras Petraitis                [Log out]      │
   ├────────────────────────────────────────────────────────┤
   │  Your subscription: Premium HD  ✓ Active               │
   │  142 channels · Valid until 2026-05-01                 │
   ├────────────────┬───────────────────────────────────────┤
   │ Your Devices   │  Channels                             │
   │                │                                       │
   │ STB #1         │  National  Sports  Movies  Kids  News │
   │ MAG 522W3      │  ─────────────────────────────────    │
   │ 00:1A:2B:...   │  LNK  ▶   TV3  ▶   BTV  ▶           │
   │ ✓ Online       │  ESPN ▶   NBA  ▶   HBO  ▶           │
   │ FW: 3.17.19    │                                       │
   │                │  [View all 142 channels]              │
   │ [+ Link box]   │                                       │
   └────────────────┴───────────────────────────────────────┘
   ```

   **STBAuthPage** — expired link / login entry point
   **STBChannelListPage** — full channel lineup with search/filter

---

## Phase 3 — Domain Layer — status: open

Pure Go domain types, zero framework deps, TDD.

8. [ ] `internal/modules/iptv/domain/channel.go`
   - `Channel{ID, Name, LogoURL, StreamURL, Category, EPGSource, Active}`
   - `ChannelRepository` interface (Save, FindByID, FindAll, Delete)

9. [ ] `internal/modules/iptv/domain/package.go`
   - `Package{ID, Name, Price, Description}` (price in cents)
   - `PackageChannels` — many-to-many via junction
   - `PackageRepository` interface

10. [ ] `internal/modules/iptv/domain/subscription.go`
    - `Subscription{ID, CustomerID, PackageID, Status, StartsAt, EndsAt, CreatedAt}`
    - `Status` enum: active/suspended/cancelled
    - `SubscriptionRepository` interface

11. [ ] `internal/modules/iptv/domain/stb.go`
    - `STB{ID, MAC, Model, Firmware, Serial, CustomerID, AssignedAt, Notes}`
    - MAC as normalised uppercase hex (00:1A:2B:...)
    - `STBRepository` interface

12. [ ] `internal/modules/iptv/domain/stb_token.go`
    - Same pattern as `portal/domain/token.go` — SHA-256 hash, magic link, 24h TTL
    - `STBToken{ID, CustomerID, TokenHash, ExpiresAt, CreatedAt}`
    - `STBTokenRepository` interface (Save, FindByHash, DeleteByCustomerID, PruneExpired)

13. [ ] Domain unit tests for Channel, Subscription, STBToken

---

## Phase 4 — Persistence + Migrations — status: open

14. [ ] `internal/modules/iptv/migrations/001_iptv_tables.sql`
    ```sql
    CREATE TABLE iptv_channels (id, name, logo_url, stream_url, category, epg_source, active, created_at);
    CREATE TABLE iptv_packages (id, name, price_cents, description, created_at);
    CREATE TABLE iptv_package_channels (package_id, channel_id, PRIMARY KEY(package_id, channel_id));
    CREATE TABLE iptv_subscriptions (id, customer_id REFERENCES customers, package_id, status, starts_at, ends_at, created_at);
    CREATE TABLE iptv_stbs (id, mac UNIQUE, model, firmware, serial, customer_id, assigned_at, notes);
    CREATE TABLE iptv_stb_tokens (id, customer_id, token_hash UNIQUE, expires_at, created_at);
    ```

15. [ ] Persistence adapters:
    - `internal/modules/iptv/adapters/persistence/channel_repository.go`
    - `internal/modules/iptv/adapters/persistence/package_repository.go`
    - `internal/modules/iptv/adapters/persistence/subscription_repository.go`
    - `internal/modules/iptv/adapters/persistence/stb_repository.go`
    - `internal/modules/iptv/adapters/persistence/stb_token_repository.go`

---

## Phase 5 — CQRS Layer — status: open

16. [ ] Commands:
    - `CreateChannelHandler`
    - `UpdateChannelHandler`
    - `DeleteChannelHandler`
    - `CreatePackageHandler`
    - `AssignChannelToPackageHandler`
    - `CreateSubscriptionHandler`
    - `SuspendSubscriptionHandler`
    - `AssignSTBHandler` (link STB MAC to customer)
    - `GenerateSTBLinkHandler` (admin generates portal access link)

17. [ ] Queries:
    - `ListChannelsHandler`
    - `ListPackagesHandler`
    - `GetPackageWithChannelsHandler`
    - `ListSubscriptionsHandler`
    - `GetSubscriptionWithPackageHandler`
    - `ListSTBsForCustomerHandler`
    - `ListAllSTBsHandler`

18. [ ] Wire commands into HTTP handlers — replace stub data with real queries
    - SSE create/edit modals for channels, packages, subscriptions, STBs
    - Dashboard with real counts

---

## Phase 6 — STB NATS Bridge + cmd/stb Binary — status: open

19. [ ] `internal/modules/iptv/adapters/nats/stb_bridge.go` (core side)
    - Subscribe to `isp.stb.rpc.*` subjects
    - Handlers use queries/repos injected at startup
    - Same `bridgeReply(msg, data, err)` pattern as `portal/adapters/nats/bridge.go`

20. [ ] `internal/modules/iptv/adapters/nats/stb_bridge_test.go`
    - 8 tests via embedded NATS (StartEmbedded("127.0.0.1:0"))

21. [ ] `internal/modules/iptv/adapters/nats/stb_client.go` (portal side)
    - `STBNATSClient` with `rpc()` helper (same pattern as `portal/adapters/nats/client.go`)
    - Implements interfaces needed by STB portal HTTP handlers

22. [ ] `cmd/stb/main.go` — STB portal binary
    - Flags: `--addr`, `--nats-url`, `--nats-auth-token`, `--base-url`, `--secure-cookie`
    - Routes: `/stb/auth`, `/stb/logout`, `/stb/dashboard`, `/stb/channels`, `/stb/devices`
    - Zero DB imports — NATS client only
    - `cmd/stb/isolation_test.go` — admin routes → 404

23. [ ] `deploy/stb.env.example`
    ```env
    STB_ADDR=:8082
    NATS_URL=nats://10.0.0.1:4222
    NATS_AUTH_TOKEN=...
    VVS_BASE_URL=https://stb.example.com
    ```

24. [ ] `deploy/vvs-stb.service`, `deploy/nginx-stb.conf`

---

## Phase 7 — Integration + Tests — status: open

25. [ ] Integration test: `go test ./internal/modules/iptv/...`
26. [ ] Integration test: `go test ./cmd/stb/...`
27. [ ] Wire IPTV migration into `internal/app/app.go` with correct goose table `goose_iptv`
28. [ ] IPTV admin generates STB portal link → verify it works on cmd/stb

---

## Verification

```bash
# Build all three binaries
go build ./cmd/server && go build ./cmd/portal && go build ./cmd/stb

# IPTV module tests
go test ./internal/modules/iptv/...

# STB isolation
go test ./cmd/stb/...

# Manual: IPTV operator scoped to ModuleIPTV
# 1. Create user, restrict to ModuleIPTV only in /settings/permissions
# 2. Login as that user → should see only IPTV nav group, no CRM/Invoice/Network
# 3. Navigate to /iptv/dashboard → IPTV dashboard loads
# 4. Navigate to /customers → 403 Forbidden

# Manual: STB portal
# 1. IPTV admin generates STB portal link for customer
# 2. Open link → STB portal auth → cookie set
# 3. /stb/dashboard → shows subscription + STBs
# 4. /customers → 404 (admin route not registered)
```

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

- 2026-04-19: Plan created — IPTV module + STB portal + multi-service RBAC, 7 phases, 28 actions
