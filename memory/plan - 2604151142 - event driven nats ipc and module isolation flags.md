---
tldr: Move all cross-module communication to NATS events; add VVS_MODULES flag and external NATS support to run individual modules in isolation
status: active
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

### Phase 1 — Spec: event contracts and module boundary rules — status: open

1. [ ] `/eidos:spec` — event driven module boundaries
   - Define canonical NATS subject taxonomy: `isp.{module}.{verb}` (current subjects + new ones needed)
   - Rule: modules may not import each other's packages; only `internal/shared/` and `internal/infrastructure/`
   - Exception: `app.go` is composition root — it may import all modules for wiring only
   - New subject needed: `isp.network.arp_requested` (manual trigger from customer UI)
   - Document `Module` interface pattern: `Name() string`, `Register(AppDeps) error`

### Phase 2 — External NATS support — status: open

2. [ ] Add `NATS_URL` and `NATS_LISTEN_ADDR` config + flags
   - `config.go`: `NATSUrl string`, `NATSListenAddr string`
   - `main.go`: `--nats-url` / `NATS_URL`, `--nats-listen` / `NATS_LISTEN_ADDR`
   - `embedded.go`: `StartEmbedded(listenAddr string)` — if listenAddr != "", set `opts.Host` + `opts.Port` so TCP is exposed
   - New: `ConnectExternal(url string) (*nats.Conn, error)` — plain `nats.Connect(url)`
   - `app.go`: if `cfg.NATSUrl != ""` → connect external; else → start embedded (with optional listen addr)

3. [ ] Write test: embedded NATS with listen addr exposes TCP port
   - Start embedded with `:0`, verify `nats.Connect("nats://127.0.0.1:<port>")` works from a second goroutine

### Phase 3 — Decouple network ↔ customer cross-module calls — status: open

4. [ ] Remove `WithNetworkProvisioning` from customer HTTP handlers
   - Customer ARP enable/disable endpoint (`POST /api/customers/{id}/arp`) currently calls `syncARPCmd` directly
   - Replace: publish `isp.network.arp_requested` with payload `{customer_id, action}` instead
   - Remove network module imports from `customer/adapters/http/handlers.go`
   - Customer form router dropdown: read `routers` table directly via SQLite reader (allowed per arch spec — "cross-module reads go through shared SQLite reader, never through another module's domain layer")
     - Add `ListRouters(ctx, reader) ([]RouterRow, error)` function in customer module reading `routers` table by SQL
     - No import of network module needed

5. [ ] Move subscriber workers into their own module
   - Create `network/app/subscribers/arp_worker.go`: `ARPWorker` struct with `Run(ctx, sub, cmd)`
   - Subscribes to both `isp.customer.*` (auto-sync on status change) and `isp.network.arp_requested` (manual trigger)
   - Replaces `runCustomerARPSubscriber` in `app.go` — delete it from there
   - `app.go` just calls `go arpWorker.Run(ctx, subscriber, syncARPCmd)`

6. [ ] Verify zero cross-module imports
   - `grep -r "modules/" --include="*.go" internal/modules/customer/` must show no `network` imports
   - `grep -r "modules/" --include="*.go" internal/modules/network/` must show no `customer` imports

### Phase 4 — Module enable flags — status: open

7. [ ] Add `VVS_MODULES` config + flag
   - `config.go`: `EnabledModules []string` — empty = all enabled
   - `main.go`: `--modules` / `VVS_MODULES` (comma-separated: `auth,customer,product,network`)
   - Helper: `Config.IsEnabled(name string) bool` — true if list empty or name in list
   - Migrations always run for all modules regardless (schema must be consistent)

8. [ ] Guard each module's wiring in `app.go` with `cfg.IsEnabled`
   - HTTP routes only mounted if module enabled
   - NATS subscribers (workers) only started if module enabled
   - Repos + commands + queries still built (they're cheap); only side effects are gated
   - Example: `if cfg.IsEnabled("network") { go arpWorker.Run(...); networkRoutes.Register(router) }`

9. [ ] Smoke test: `VVS_MODULES=auth` starts server with only login page; `/customers` returns 404

### Phase 5 — Module registry refactor (app.go cleanup) — status: open

10. [ ] Define `Module` interface in `internal/app/module.go`
    ```go
    type AppDeps struct {
        DB, Reader *gorm.DB
        Writer     *database.WriteSerializer
        Publisher  events.EventPublisher
        Subscriber events.EventSubscriber
        Router     *infrahttp.Router
        Config     Config
    }
    type Module interface {
        Name() string
        Register(ctx context.Context, deps AppDeps) error
    }
    ```

11. [ ] Implement `Register(ctx, AppDeps) error` for each module
    - `auth.Module`, `customer.Module`, `product.Module`, `network.Module`
    - Each registers its own routes + subscribers inside `Register`
    - Migrations keep running at startup for all modules regardless of enable flag

12. [ ] Refactor `app.New()` to use the registry
    - Build `AppDeps`
    - `modules := []Module{&auth.Module{}, &customer.Module{}, ...}`
    - `for _, m := range modules { if cfg.IsEnabled(m.Name()) { m.Register(ctx, deps) } }`
    - `app.go` becomes a thin bootstrap; all domain wiring lives in each module's `Register`

13. [ ] `templ generate ./...` + `go build ./...` — clean

### Phase 6 — Verification — status: open

14. [ ] `go test ./... -race` — all pass
15. [ ] `go build ./...` — clean
16. [ ] Verify zero cross-module imports: `grep -rn "modules/" internal/modules/*/` shows no cross-module deps
17. [ ] `VVS_MODULES=auth,customer go run ./cmd/server` — only customer + auth routes available
18. [ ] `NATS_LISTEN_ADDR=:4222 go run ./cmd/server` — `nats-cli sub "isp.>"` from terminal receives events
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
