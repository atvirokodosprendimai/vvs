---
tldr: IPTV module in vvs-core + STB device API binary (M3U8/EPG/Stalker JSON — no browser UI); per-user module scoping
status: active
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

No HTML, no cookies, no session pages. Authentication is MAC-based (device identity).

### IPTV domain concepts

| Concept | What |
|---------|------|
| **Channel** | Single broadcast stream — name, logo URL, stream URL, category, EPG source |
| **Package** | Bundle of channels with a price — customers subscribe to packages |
| **Subscription** | Customer ↔ Package link — status (active/suspended/cancelled), dates |
| **STB** | Set-Top Box — MAC address (device identity), model, firmware, customer assignment |
| **STBSession** | Device session token — MAC authenticates → gets short-lived token for playlist/EPG URLs |

### STB API protocols served by cmd/stb

| Protocol | Endpoint | Used by |
|----------|----------|---------|
| **M3U8 playlist** | `GET /stb/playlist/{mac}.m3u8` | VLC, Tivimate, IPTV apps |
| **M3U8 + token** | `GET /stb/playlist?token={t}&format=m3u8` | Apps with auth |
| **XMLTV EPG** | `GET /stb/epg/{mac}.xml` | Any EPG consumer |
| **Stalker handshake** | `POST /portal/server.php?action=handshake` | MAG 250/500/522, Formuler |
| **Stalker profile** | `GET /portal/server.php?action=get_profile` | MAG boxes |
| **Stalker channels** | `GET /portal/server.php?action=get_all_channels` | MAG boxes |
| **JSON REST** | `POST /stb/api/auth`, `GET /stb/api/channels` | Custom integrations |

Authentication: STB sends its MAC → core validates MAC is registered + subscription active → returns session token → token embedded in stream URLs or used as Bearer.

### NATS RPC for STB API (`isp.stb.rpc.*`)

| Subject | Request | Reply |
|---------|---------|-------|
| `isp.stb.rpc.auth` | `{mac, serial?}` | `{token, expiresAt, packageID}` or error if not registered/expired |
| `isp.stb.rpc.playlist.get` | `{mac}` | `{channels: [{name, streamURL, logoURL, epgID}]}` |
| `isp.stb.rpc.epg.get` | `{packageID, days}` | `{xmltv: string}` (XMLTV XML) |
| `isp.stb.rpc.channel.stream` | `{mac, channelID}` | `{streamURL}` (single channel, Stalker create_link) |
| `isp.stb.rpc.subscription.check` | `{mac}` | `{active: bool, packageID, expiresAt}` |

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

12. [ ] `internal/modules/iptv/domain/stb_session.go`
    - Device-side auth token — MAC authenticates → gets 24h session token for stream URLs
    - `STBSession{ID, MAC, Token, ExpiresAt, CreatedAt}` (Token = 32-byte random hex)
    - `STBSessionRepository` interface (Save, FindByToken, DeleteByMAC, PruneExpired)
    - `NewSTBSession(mac string, ttl duration) (*STBSession, error)`

13. [ ] Domain unit tests for Channel, Subscription, STBSession

---

## Phase 4 — Persistence + Migrations — status: open

14. [ ] `internal/modules/iptv/migrations/001_iptv_tables.sql`
    ```sql
    CREATE TABLE iptv_channels (id, name, logo_url, stream_url, category, epg_source, active, created_at);
    CREATE TABLE iptv_packages (id, name, price_cents, description, created_at);
    CREATE TABLE iptv_package_channels (package_id, channel_id, PRIMARY KEY(package_id, channel_id));
    CREATE TABLE iptv_subscriptions (id, customer_id REFERENCES customers, package_id, status, starts_at, ends_at, created_at);
    CREATE TABLE iptv_stbs (id, mac UNIQUE, model, firmware, serial, customer_id, assigned_at, notes);
    CREATE TABLE iptv_stb_sessions (id, mac REFERENCES iptv_stbs(mac), token UNIQUE, expires_at, created_at);
    ```

15. [ ] Persistence adapters:
    - `internal/modules/iptv/adapters/persistence/channel_repository.go`
    - `internal/modules/iptv/adapters/persistence/package_repository.go`
    - `internal/modules/iptv/adapters/persistence/subscription_repository.go`
    - `internal/modules/iptv/adapters/persistence/stb_repository.go`
    - `internal/modules/iptv/adapters/persistence/stb_session_repository.go`

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
    - `AssignSTBHandler` (register MAC → customer + subscription)
    - `UnassignSTBHandler` (revoke device access — blacklist)

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

## Phase 6 — STB NATS Bridge + cmd/stb API Binary — status: open

19. [ ] `internal/modules/iptv/adapters/nats/stb_bridge.go` (core side)
    - Subscribe to `isp.stb.rpc.*` subjects
    - `handleAuth`: lookup MAC in `iptv_stbs` → verify subscription active → create STBSession → return token
    - `handlePlaylist`: lookup MAC → get subscription → build channel list with stream URLs
    - `handleEPG`: lookup packageID → build XMLTV XML string (stub initially, real EPG later)
    - `handleChannelStream`: single channel URL for Stalker `create_link`
    - `handleSubscriptionCheck`: quick active/expired check
    - Same `bridgeReply(msg, data, err)` pattern as `portal/adapters/nats/bridge.go`

20. [ ] `internal/modules/iptv/adapters/nats/stb_bridge_test.go`
    - Tests: auth valid MAC, auth unknown MAC → error, auth expired subscription → error
    - playlist returns channels, subscription check active/expired

21. [ ] `internal/modules/iptv/adapters/nats/stb_client.go` (API side)
    - `STBNATSClient` with `rpc()` helper
    - Methods: `Auth(mac) (token, packageID, error)`, `GetPlaylist(mac) ([]Channel, error)`
    - `GetEPG(packageID, days) (string, error)`, `CheckSubscription(mac) (bool, error)`

22. [ ] `cmd/stb/main.go` — STB device API binary (NO HTML templates)
    - Flags: `--addr`, `--nats-url`, `--nats-auth-token`, `--base-url`
    - Routes (machine API):
      - `GET /stb/playlist/{mac}.m3u8` — M3U8 playlist by MAC
      - `GET /stb/playlist` — M3U8 playlist by `?token=` or `?mac=`
      - `GET /stb/epg/{mac}.xml` — XMLTV EPG by MAC
      - `GET /stb/epg` — XMLTV EPG by `?token=`
      - `GET /portal/server.php` — Stalker/MAG portal protocol (action= dispatch)
      - `POST /stb/api/auth` — JSON `{mac, serial}` → `{token, expiresAt}`
      - `GET /stb/api/channels` — JSON channel list (Bearer token)
    - Zero DB imports — NATS client only
    - Response: `Content-Type: application/x-mpegURL` for M3U8, `text/xml` for EPG, `application/json` for REST

23. [ ] `deploy/stb.env.example`
    ```env
    STB_ADDR=:8082
    NATS_URL=nats://10.0.0.1:4222
    NATS_AUTH_TOKEN=...
    VVS_BASE_URL=https://stb.example.com
    ```

24. [ ] `deploy/vvs-stb.service`, `deploy/nginx-stb.conf`
    - Note: no SSE/long-poll buffering needed (unlike vvs-portal) — standard short-lived requests

---

## Phase 7 — Integration + Tests — status: open

25. [ ] Integration test: `go test ./internal/modules/iptv/...`
26. [ ] Integration test: `go test ./cmd/stb/...`
27. [ ] Wire IPTV migration into `internal/app/app.go` with correct goose table `goose_iptv`
28. [ ] STB device auth flow: register MAC in admin → `curl /stb/api/auth` with MAC → get token → `curl /stb/playlist?token=` → M3U8 returned

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
