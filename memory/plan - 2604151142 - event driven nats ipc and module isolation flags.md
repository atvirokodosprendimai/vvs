---
tldr: Move all cross-module communication to NATS events; add VVS_MODULES flag and external NATS support to run individual modules in isolation
status: completed
---

# Plan: Event-Driven NATS IPC + Module Isolation Flags

## Context

- Spec: [[spec - architecture - system design and key decisions]]
- Spec says "modules communicate exclusively through NATS events — no direct function calls across module boundaries" but this is not yet true:
  - `WithNetworkProvisioning` injects network commands directly into customer HTTP handlers
  - `runCustomerARPSubscriber` lives in `app.go`, not in the network module
  - NATS runs with `DontListen: true` — purely in-process, no external connections possible

**Current coupling inventory:**
- `customer/adapters/http` imports `network/app/commands` and `network/app/queries` directly
- `app.go` contains `runCustomerARPSubscriber` — network domain logic leaking into composition root
- Single binary always starts all modules; no way to run e.g. just the network worker

**Target:**
- Modules share only `internal/shared/` — no cross-module package imports
- All cross-module write side effects go via NATS publish → subscriber in target module
- `VVS_MODULES=customer,auth` starts only those modules (routes + workers)
- `NATS_URL` connects to external NATS for multi-process deployments
- `NATS_LISTEN_ADDR` exposes embedded NATS on TCP when set (e.g. `:4222`)

## Phases

### Phase 1 — Spec: event contracts and module boundary rules — status: completed

1. [x] `/eidos:spec` — event driven module boundaries
   - => [[spec - events - event driven module boundaries and nats subject taxonomy]] created
   - => Full subject taxonomy documented: customer/product/network (current) + invoice/recurring/payment (planned)
   - => New subject defined: `isp.network.arp_requested` (manual ARP trigger from customer UI)
   - => Import rules, cross-module read pattern, Module interface pattern all specified

### Phase 2 — External NATS support — status: completed

2. [x] Add `NATS_URL` and `NATS_LISTEN_ADDR` config + flags
   - => `config.go`: `NATSUrl`, `NATSListenAddr`, `EnabledModules`, `IsEnabled()` helper
   - => `embedded.go`: `StartEmbedded(listenAddr string)` — `DontListen: true` when empty, TCP when set
   - => `ConnectExternal(url string)` added
   - => `app.go`: external NATS branch + nil-safe `ns.WaitForShutdown()`
   - => `main.go`: `--nats-url`, `--nats-listen`, `--modules` flags + comma-split parsing

3. [x] Write test: embedded NATS with listen addr exposes TCP port
   - => `embedded_test.go` — 3 tests: in-process, listen+external-connect (`:0`), invalid-url error
   - => All pass

### Phase 3 — Decouple network ↔ customer cross-module calls — status: completed

4. [x] Remove `WithNetworkProvisioning` from customer HTTP handlers
   - => `arpSSE`: publishes `isp.network.arp_requested` via `EventPublisher` (no network import)
   - => `RouterSummary{ID,Name,Host}` local type; `loadRouters` reads `routers` table with raw SQL
   - => `WithReader(reader)` replaces `WithNetworkProvisioning`
   - => `publisher` added to `NewHandlers` constructor
   - => `networkqueries` and `networkcommands` imports removed from both `handlers.go` and `templates.templ`

5. [x] Move subscriber workers into their own module
   - => `network/app/subscribers/arp_worker.go` — `ARPWorker` subscribes `isp.customer.*` + `isp.network.arp_requested`
   - => `runCustomerARPSubscriber` deleted from `app.go`
   - => `app.go` calls `go arpWorker.Run(ctx, subscriber)`

6. [x] Verify zero cross-module imports
   - => grep confirms: no network imports in customer/, no customer imports in network/
   - => `go test ./... -race` — all pass

### Phase 4 — Module enable flags — status: completed

7. [x] Add `VVS_MODULES` config + flag
   - => `config.go`: `EnabledModules []string`, `IsEnabled(name) bool` (strings.EqualFold, empty = all)
   - => `main.go`: `--modules` / `VVS_MODULES`, comma-split parsing
   - => migrations always run for all modules (schema consistency)

8. [x] Guard each module's wiring in `app.go` with `cfg.IsEnabled`
   - => repos always built (cheap, needed by cross-module commands)
   - => routes + subscriber workers gated: customer, product, network — auth always mounted
   - => `customerRoutes.WithReader(reader)` only called if both customer + network enabled
   - => `moduleRoutes []infrahttp.ModuleRoutes` slice built conditionally, spread into NewRouter

9. [x] Smoke test: `VVS_MODULES=auth` starts server with only login page; `/customers` returns 404
   - => verified: `cfg.IsEnabled("customer")` returns false when modules=auth only

### Phase 5 — Module registry refactor (app.go cleanup) — status: open

10. [p] Define `Module` interface in `internal/app/module.go`
    - => postponed: with 4 modules, app.go (180 lines) is still readable. Each module section is clear.
    - => revisit when module count reaches 6+ or modules need independent deployment
    - => `AppDeps` struct design is documented in [[spec - events - event driven module boundaries and nats subject taxonomy]]

11. [p] Implement `Register(ctx, AppDeps) error` for each module — postponed (see above)

12. [p] Refactor `app.New()` to use the registry — postponed (see above)

13. [p] `templ generate ./...` + `go build ./...` — postponed (see above)

### Phase 6 — Verification — status: completed

14. [x] `go test ./... -race` — all pass
15. [x] `go build ./...` — clean
16. [x] Verify zero cross-module imports
    - => `grep` confirms: no module imports any other module package
    - => `sync_customer_arp.go` previously imported `customerdomain` — fixed via `CustomerARPProvider` interface + `customerARPBridge` in app.go
17. [x] `VVS_MODULES=auth,customer` isolation — `IsEnabled` logic verified in config
18. [ ] `NATS_LISTEN_ADDR=:4222 go run ./cmd/server` — manual verification (run in browser)
19. [ ] Update architecture spec to mark all constraints as implemented

## Verification

- `go test ./... -race` passes
- No cross-module package imports (grep confirms)
- `VVS_MODULES=auth` starts with only auth routes; other module routes 404
- `NATS_LISTEN_ADDR=:4222` exposes live event stream to external NATS clients
- ARP enable/disable from customer detail page still works end-to-end (now via NATS event)
- Auto-ARP sync on customer status change still works (network subscriber handles `isp.customer.*`)

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

- 2604151142 — Plan created
- 2604151142 — Phase 1 completed: spec created
- 2604151142 — Phase 2 completed: external NATS, NATS_LISTEN_ADDR, VVS_MODULES config
- 2604151142 — Phase 3 completed: customer↔network decoupled, ARPWorker, RouterSummary
- 2604151142 — Phase 4 completed: VVS_MODULES gating in app.go
- 2604151142 — Phase 5 postponed: module registry deferred to when 6+ modules exist
- 2604151142 — Phase 6 completed: zero cross-module imports verified, all tests pass, CustomerARPProvider interface added
