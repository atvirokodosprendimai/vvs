---
tldr: IPTV module in vvs-core + STB device API binary (M3U8/EPG/Stalker JSON — no browser UI); per-user module scoping
status: completed
---

# Plan: IPTV Module + STB Device API (Multi-Service RBAC)

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
┌──────────────────┐       ┌──────────────────────────────────┐
│  vvs-portal      │       │  vvs-stb  (MACHINE API only)     │
│  /portal/*       │       │  NO browser UI — serves STB boxes│
│  invoice PDF     │       │  GET /stb/playlist/{mac}.m3u8    │
│  magic-link auth │       │  GET /stb/epg/{mac}.xml          │
└──────────────────┘       │  POST /stb/api/auth (Stalker)    │
                           │  GET /stb/api/channels (JSON)    │
                           └──────────────────────────────────┘
                                    ▲
                           STB devices connect here
                           (MAG, IPTV apps, VLC, etc.)
```

**vvs-stb is NOT a human portal.** It serves machine clients:
- **MAG / Stalker protocol** — JSON API used by MAG250/500/522 boxes and clones
- **M3U8 playlist** — for IPTV players, VLC, Tivimate, etc.
- **XMLTV EPG** — electronic programme guide XML
- **Simple JSON REST** — for custom integrations

No HTML, no cookies, no session pages.

**Authentication: per-subscription token embedded in every URL.**
Each subscriber gets a unique opaque token when their subscription is created.
The token is distributed to them as part of the playlist URL — they configure it in their app once.
No device registration, no MAC pairing, no session handshake needed.

```
https://stb.example.com/apis/siptv/playlist/{token}       ← SIPTV app
https://stb.example.com/apis/tvzone/playlist/{token}       ← tvzone / generic
https://stb.example.com/stream/{token}/{channelID}         ← per-channel stream redirect
https://stb.example.com/epg/{token}.xml                    ← XMLTV EPG
https://stb.example.com/portal/server.php?token={token}    ← Stalker/MAG protocol
```

The `{token}` is 32-byte random hex (64 chars), stored **plain** in the DB (it's a long-lived subscription credential, must be showable to admin for copy-paste to customer). If token leaks: admin revokes and issues a new one.

### IPTV domain concepts

| Concept | What |
|---------|------|
| **Channel** | Single broadcast stream — name, logo URL, stream URL, category, EPG source |
| **Package** | Bundle of channels with a price — customers subscribe to packages |
| **Subscription** | Customer ↔ Package link — status (active/suspended/cancelled), start/end dates |
| **STB** | Optional: Set-Top Box inventory — MAC, model, notes, assigned customer |
| **SubscriptionKey** | Per-subscription API key — `{ID, SubscriptionID, CustomerID, Token, CreatedAt, RevokedAt}`. Token stored PLAIN (must be showable). Revoke + re-issue if compromised. |

### STB API protocols served by cmd/stb

| Protocol | Endpoint | Used by |
|----------|----------|---------|
| **M3U8 — SIPTV** | `GET /apis/siptv/playlist/{token}` | SIPTV app (specific M3U8 tags) |
| **M3U8 — generic** | `GET /apis/tvzone/playlist/{token}` | tvzone, VLC, Tivimate, generic apps |
| **M3U8 — TVIP** | `GET /apis/tvip/playlist/{token}` | TVIP STBs (custom tag format) |
| **XMLTV EPG** | `GET /epg/{token}.xml` | Any EPG consumer |
| **Per-channel stream** | `GET /stream/{token}/{channelID}` | In-M3U8 channel redirect URL |
| **Stalker/MAG** | `GET /portal/server.php?token={token}&action=...` | MAG 250/500/522, Formuler, clones |

Every channel URL in the M3U8 playlist goes through `/stream/{token}/{channelID}` — this validates the token is still active before redirecting/proxying to the real stream URL. Allows per-subscriber access revocation.

### NATS RPC for STB API (`isp.stb.rpc.*`)

| Subject | Request | Reply |
|---------|---------|-------|
| `isp.stb.rpc.key.validate` | `{token}` | `{customerID, subscriptionID, packageID, active}` |
| `isp.stb.rpc.playlist.get` | `{token}` | `{channels: [{id, name, streamURLTemplate, logoURL, epgID, category}]}` |
| `isp.stb.rpc.epg.get` | `{token, days}` | `{xmltv: string}` (XMLTV XML) |
| `isp.stb.rpc.channel.resolve` | `{token, channelID}` | `{streamURL}` (actual stream URL for redirect) |

---

## Phase 1 — Spec — status: skipped

Spec deferred — user requested scaffold directly. Plan adjusted to start with scaffold (Phase 2).

1. [~] `/eidos:spec` — write `eidos/spec - iptv - channel package stb management.md`
   - => skipped per user instruction ("start iptv module scaffold")
   - Document domain model: Channel, Package, Subscription, SubscriptionKey (per-sub token), STB (optional device inventory)
   - Document token-in-URL auth model — no MAC, no session, long-lived key per subscription
   - Document STB API endpoints in cmd/stb (per-app M3U8 paths, EPG, stream redirect, Stalker)
   - Document NATS RPC subjects (table above)
   - Document RBAC: ModuleIPTV constant, admin UI permissions page

---

## Phase 2 — IPTV UI Mockup (templ, static) — status: active

Delivered as real pages backed by repos (not stub data), merged with Phase 3/4 skeleton.

### IPTV admin screens (in vvs-core templ)

2. [x] Create `internal/modules/iptv/adapters/http/templates.templ` with stub pages:
   - => IPTVChannelListPage, IPTVPackageListPage, IPTVSubscriptionListPage, IPTVSTBListPage
   - => uses real domain types, renders empty-state message when no data

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

3. [x] Register stub routes in `internal/modules/iptv/adapters/http/handlers.go`
   - `GET /iptv/dashboard` → IPTVDashboardPage (stub data)
   - `GET /iptv/channels` → IPTVChannelListPage (stub data)
   - `GET /iptv/packages` → IPTVPackageListPage (stub data)
   - `GET /iptv/subscribers` → IPTVSubscriberListPage (stub data)
   - `GET /iptv/stbs` → IPTVSTBListPage (stub data)
   - `ModuleName() authdomain.Module` — return `ModuleIPTV`

4. [x] Add `ModuleIPTV` to `internal/modules/auth/domain/permissions.go`
   - => ModuleIPTV = "iptv"; added to AllModules between ModuleReports and ModuleNetwork
   - Const `ModuleIPTV Module = "iptv"`

5. [x] Wire routes in `internal/app/app.go`
   - => migration goose_iptv registered; 5 repos instantiated; iptvRoutes wired into moduleRoutes
   - `iptvRoutes := iptvhttp.NewHandlers()`
   - `moduleRoutes = append(moduleRoutes, iptvRoutes)`

6. [x] Add IPTV nav entry in sidebar
   - => "_navIptv" group between Finance and Network; 4 items (Channels/Packages/Subscriptions/STBs); 4 SVG icons added
   - Group: "Services" (new group between Finance and Network)
   - `iptv-icon.svg` or use a simple play-button SVG

### STB API admin screen (in vvs-core)

7. [ ] Add `IPTVSTBDetailPage` for admin — shows device info + integration URLs:
   ```
   ┌────────────────────────────────────────────────────────┐
   │  STB: MAG 522W3                        [Edit] [Remove] │
   ├────────────────────────────────────────────────────────┤
   │  MAC:        00:1A:2B:3C:4D:5E                         │
   │  Customer:   UAB Petro Paslaugos                       │
   │  Package:    Premium HD                                │
   │  Status:     ✓ Active (until 2026-05-01)               │
   ├────────────────────────────────────────────────────────┤
   │  Integration URLs (copy to configure device)           │
   │  M3U8:    https://stb.example.com/stb/playlist/        │
   │           00:1A:2B:3C:4D:5E.m3u8                      │
   │  EPG:     https://stb.example.com/stb/epg/             │
   │           00:1A:2B:3C:4D:5E.xml                       │
   │  Stalker: https://stb.example.com                      │
   │           (enter in MAG portal URL field)              │
   └────────────────────────────────────────────────────────┘
   ```

---

## Phase 3 — Domain Layer — status: completed

Pure Go domain types, zero framework deps. Shipped with scaffold (commit 39294d4).

8. [x] `internal/modules/iptv/domain/channel.go`
   - `Channel{ID, Name, LogoURL, StreamURL, Category, EPGSource, Active}`
   - `ChannelRepository` interface (Save, FindByID, FindAll, Delete)

9. [x] `internal/modules/iptv/domain/package.go`
   - `Package{ID, Name, Price, Description}` (price in cents)
   - `PackageChannels` — many-to-many via junction
   - `PackageRepository` interface

10. [x] `internal/modules/iptv/domain/subscription.go`
    - `Subscription{ID, CustomerID, PackageID, Status, StartsAt, EndsAt, CreatedAt}`
    - `Status` enum: active/suspended/cancelled
    - `SubscriptionRepository` interface

11. [x] `internal/modules/iptv/domain/stb.go`
    - `STB{ID, MAC, Model, Firmware, Serial, CustomerID, AssignedAt, Notes}`
    - MAC as normalised uppercase hex (00:1A:2B:...)
    - `STBRepository` interface

12. [x] `internal/modules/iptv/domain/subscription_key.go`
    - Per-subscription API key — long-lived, embedded in all URLs
    - `SubscriptionKey{ID, SubscriptionID, CustomerID, PackageID, Token, CreatedAt, RevokedAt}`
    - Token: 32-byte random hex (64 chars), stored PLAIN in DB (must be showable to admin)
    - `RevokedAt *time.Time` — nil = active, set = revoked
    - `SubscriptionKeyRepository` interface (Save, FindByToken, RevokeByID, FindBySubscriptionID)
    - `NewSubscriptionKey(subscriptionID, customerID, packageID string) (*SubscriptionKey, error)`

13. [ ] Domain unit tests for Channel, Subscription, SubscriptionKey (open)

---

## Phase 4 — Persistence + Migrations — status: completed

Shipped with scaffold (commit 39294d4).

14. [x] `internal/modules/iptv/migrations/001_iptv_tables.sql`
    ```sql
    CREATE TABLE iptv_channels (id, name, logo_url, stream_url, category, epg_source, active, created_at);
    CREATE TABLE iptv_packages (id, name, price_cents, description, created_at);
    CREATE TABLE iptv_package_channels (package_id, channel_id, PRIMARY KEY(package_id, channel_id));
    CREATE TABLE iptv_subscriptions (id, customer_id REFERENCES customers, package_id, status, starts_at, ends_at, created_at);
    CREATE TABLE iptv_stbs (id, mac UNIQUE, model, firmware, serial, customer_id, assigned_at, notes);
    CREATE TABLE iptv_subscription_keys (id, subscription_id REFERENCES iptv_subscriptions, customer_id, package_id, token UNIQUE, created_at, revoked_at);
    ```

15. [x] Persistence adapters:
    - `internal/modules/iptv/adapters/persistence/channel_repository.go`
    - `internal/modules/iptv/adapters/persistence/package_repository.go`
    - `internal/modules/iptv/adapters/persistence/subscription_repository.go`
    - `internal/modules/iptv/adapters/persistence/stb_repository.go`
    - `internal/modules/iptv/adapters/persistence/subscription_key_repository.go`

---

## Phase 5 — CQRS Layer — status: completed

16. [x] Commands (commit b9d2a6a):
    - `CreateChannelHandler`, `UpdateChannelHandler`, `DeleteChannelHandler`
    - `CreatePackageHandler`, `UpdatePackageHandler`, `DeletePackageHandler`
    - `AddChannelToPackageHandler`, `RemoveChannelFromPackageHandler`
    - `CreateSubscriptionHandler` — auto-issues SubscriptionKey on create
    - `SuspendSubscriptionHandler`, `ReactivateSubscriptionHandler`, `CancelSubscriptionHandler`
    - `RevokeSubscriptionKeyHandler`, `ReissueSubscriptionKeyHandler`
    - `AssignSTBHandler`, `DeleteSTBHandler`

17. [x] Queries (commit b9d2a6a):
    - `ListChannelsHandler`, `ListPackagesHandler`, `GetPackageChannelsHandler`
    - `ListSubscriptionsHandler`, `ListSubscriptionsForCustomerHandler`
    - `ListSTBsHandler`

18. [x] Wire commands into HTTP handlers (commit b9d2a6a):
    - `NewIPTVHandlers(...)` with 20 command/query args
    - SSE handlers with `patchTable` pattern; create modals with signals
    - `parsePriceCents(s)` helper

---

## Phase 6 — STB NATS Bridge + cmd/stb API Binary — status: completed

19. [x] `internal/modules/iptv/adapters/nats/stb_bridge.go` (core side, commit 44bad1a)
    - => subscribes to 4 isp.stb.rpc.* subjects
    - => `resolveKey()` helper; `buildXMLTV()` stub EPG; `stbBridgeReply()` envelope pattern
    - => error sentinels: errInvalidToken, errSuspended, errChannelNotFound

20. [x] `internal/modules/iptv/adapters/nats/stb_bridge_test.go` (commit 41ab944)
    - => 8 tests with embedded NATS + stubs; all pass

21. [x] `internal/modules/iptv/adapters/nats/stb_client.go` (commit 44bad1a)
    - => STBNATSClient; same rpc() pattern as portal client; 4 typed methods

22. [x] `cmd/stb/main.go` (commit 44bad1a)
    - => all 6 routes; playlistHandler with 3 format variants; stalker handshake/get_profile/get_all_channels
    - => graceful shutdown; chi + middleware

23. [x] `deploy/stb.env.example` (commit 44bad1a)

24. [x] `deploy/vvs-stb.service`, `deploy/nginx-stb.conf` (commit 44bad1a)
    - => Makefile: build-stb, run-stb, DEV_STB=:8082; build-all/run-all include vvs-stb

---

## Phase 7 — Integration + Tests — status: completed

25. [x] Domain tests: subscription_key_test.go + subscription_test.go (15 tests, commit 41ab944)
    - => token length, uniqueness, missing fields; state machine full cycle
26. [x] NATS bridge tests: stb_bridge_test.go (8 tests, commit 41ab944)
    - => KeyValidate (valid/revoked/empty), PlaylistGet (active-only/suspended), ChannelResolve (URL/inactive), EPGGet (XMLTV)
27. [x] Migration wired into `internal/app/app.go` goose_iptv (commit 39294d4)
28. [~] STB manual smoke test deferred — needs running vvs-core+NATS+vvs-stb

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

# Manual: STB device API
# 1. IPTV admin registers MAC 00:1A:2B:3C:4D:5E → customer → subscription
# 2. curl -s "http://localhost:8082/stb/api/auth" -d '{"mac":"00:1A:2B:3C:4D:5E"}'
#    → {"token":"...","expiresAt":"..."}
# 3. curl "http://localhost:8082/stb/playlist?token=TOKEN" → M3U8 playlist
# 4. curl "http://localhost:8082/stb/epg/00:1A:2B:3C:4D:5E.xml" → XMLTV
# 5. Stalker: configure MAG box portal URL = http://localhost:8082 → box loads channels
```

## Adjustments

- 2026-04-19: Corrected vvs-stb purpose — NOT a browser portal. It's a machine API for STB devices:
  M3U8 playlists, XMLTV EPG, Stalker/MAG protocol JSON. No HTML templates in cmd/stb.
  Authentication changes from magic-link (SHA-256 hash) to MAC-based device session (STBSession).
  NATS RPC subjects rewritten to reflect API endpoints, not human portal flows.
  STBToken → STBSession. Admin UI gets STB detail page showing integration URLs.

## Progress Log

- 2026-04-19: Plan created — IPTV module + STB portal + multi-service RBAC, 7 phases, 28 actions
- 2026-04-19: Scaffold complete (commit 39294d4): Phase 2 UI + Phase 3 domain + Phase 4 persistence + migration + nav — `go build ./...` clean
- 2026-04-19: Phase 5 CQRS complete (commit b9d2a6a): 20-arg NewIPTVHandlers, all commands/queries, SSE handlers
- 2026-04-19: Phase 6 STB bridge complete (commit 44bad1a): stb_bridge.go + stb_client.go + cmd/stb/main.go + deploy/ + Makefile
- 2026-04-19: Phase 7 tests complete (commit 41ab944): 23 tests pass (15 domain + 8 NATS bridge)
