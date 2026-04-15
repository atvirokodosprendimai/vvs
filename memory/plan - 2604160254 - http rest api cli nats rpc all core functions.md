---
title: HTTP REST API + CLI + NATS RPC for all core functions
status: active
created: 2604160254
---

# Plan — HTTP REST API + CLI + NATS RPC

## Context

The ISP manager currently only exposes functionality through a Datastar SSE web UI.
External systems, scripts, and automation have no way to call commands or queries without
browser-based sessions. This plan adds three new access layers — all additive, no changes
to the existing UI:

1. **REST JSON API** at `/api/v1/` with bearer token auth — for integrations, curl, scripts
2. **NATS request/reply service** on `isp.rpc.*` subjects — for microservices / event-driven tooling
3. **CLI subcommands** in the same binary (`vvs customer list`, `vvs product create`, etc.) —
   uses NATS transport when `--nats-url` is set, falls back to HTTP REST

Related specs:
- [[spec - events - event driven module boundaries and nats subject taxonomy]]
- [[spec - architecture - system design and key decisions]]

---

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| Binary structure | Same binary, new subcommands | Single artifact, urfave/cli v3 already present |
| REST auth | Bearer token (`VVS_API_TOKEN`) | Scriptable without session cookie dance |
| CLI transport | NATS if `--nats-url` set, else HTTP | Works both locally and remotely |
| NATS subject prefix | `isp.rpc.{module}.{action}` | Separate from pub/sub `isp.{module}.*` events |
| JSON response envelope | `{"data": ..., "error": "..."}` | Simple, consistent across all endpoints |

---

## Core Functions Inventory (28 handlers)

### AUTH (7)
- `auth.user.list`, `auth.user.create`, `auth.user.delete`, `auth.user.change-password`
- (login/logout: HTTP-only, not exposed via NATS/CLI)

### CUSTOMER (5)
- `customer.list` (search, page, pageSize), `customer.get` (id)
- `customer.create`, `customer.update`, `customer.delete`

### PRODUCT (5)
- `product.list`, `product.get`
- `product.create`, `product.update`, `product.delete`

### NETWORK / ROUTERS (4)
- `router.list`, `router.get`
- `router.create`, `router.update`, `router.delete`
- `router.sync-arp` (customerID, action enable|disable)

### SERVICE (5)
- `service.list` (customerID)
- `service.assign`, `service.suspend`, `service.reactivate`, `service.cancel`

---

## Architecture

```
External caller
     │
     ├── HTTP  POST /api/v1/customers          → REST JSON API (Bearer token)
     │
     ├── NATS  isp.rpc.customer.create         → NATS RPC server (request/reply)
     │
     └── CLI   vvs customer create ...
                 │  --nats-url set  → NATS request/reply (remote NATS)
                 └  fallback        → HTTP REST /api/v1/  (--api-url + --api-token)

All three paths call the SAME command/query handlers already in app/ packages.
```

### NATS Subject Taxonomy
```
isp.rpc.{module}.{action}

isp.rpc.customer.list        isp.rpc.customer.get
isp.rpc.customer.create      isp.rpc.customer.update    isp.rpc.customer.delete
isp.rpc.product.list         isp.rpc.product.get
isp.rpc.product.create       isp.rpc.product.update     isp.rpc.product.delete
isp.rpc.router.list          isp.rpc.router.get
isp.rpc.router.create        isp.rpc.router.update      isp.rpc.router.delete
isp.rpc.router.sync-arp
isp.rpc.service.list
isp.rpc.service.assign       isp.rpc.service.suspend
isp.rpc.service.reactivate   isp.rpc.service.cancel
isp.rpc.user.list            isp.rpc.user.create        isp.rpc.user.delete
```

### REST Routes
```
GET    /api/v1/customers              → list
POST   /api/v1/customers              → create
GET    /api/v1/customers/{id}         → get
PUT    /api/v1/customers/{id}         → update
DELETE /api/v1/customers/{id}         → delete

GET    /api/v1/products               → list
POST   /api/v1/products               → create
GET    /api/v1/products/{id}          → get
PUT    /api/v1/products/{id}          → update
DELETE /api/v1/products/{id}          → delete

GET    /api/v1/routers                → list
POST   /api/v1/routers                → create
GET    /api/v1/routers/{id}           → get
PUT    /api/v1/routers/{id}           → update
DELETE /api/v1/routers/{id}           → delete
POST   /api/v1/customers/{id}/arp     → sync-arp

GET    /api/v1/customers/{id}/services → list services
POST   /api/v1/customers/{id}/services → assign
PUT    /api/v1/services/{id}/suspend   → suspend
PUT    /api/v1/services/{id}/reactivate
DELETE /api/v1/services/{id}           → cancel

GET    /api/v1/users                  → list
POST   /api/v1/users                  → create
DELETE /api/v1/users/{id}             → delete
```

---

## Files to Create

```
internal/infrastructure/http/jsonapi/
  response.go        — JSON{data, error} envelope, WriteOK(), WriteError(), WriteNotFound()

internal/infrastructure/http/apimw/
  token.go           — BearerTokenMiddleware(token string) func(http.Handler) http.Handler

internal/modules/customer/adapters/http/
  api.go             — REST handlers for customer (no SSE, plain JSON)

internal/modules/product/adapters/http/
  api.go             — REST handlers for product

internal/modules/network/adapters/http/
  api.go             — REST handlers for routers

internal/modules/service/adapters/http/
  api.go             — REST handlers for service

internal/modules/auth/adapters/http/
  api.go             — REST handlers for users

internal/infrastructure/nats/rpc/
  server.go          — NATS RPC server wiring all handlers to isp.rpc.* subjects
  handler.go         — generic Request/Response envelope + RPC handler helper

cmd/server/
  cli_customer.go    — urfave/cli subcommands: customer {list,get,create,update,delete}
  cli_product.go     — product subcommands
  cli_router.go      — router subcommands
  cli_service.go     — service subcommands
  cli_user.go        — user subcommands
  cli_transport.go   — transport abstraction (NATS or HTTP) shared by all CLI cmds
```

## Files to Modify

```
internal/app/config.go         — add APIToken string, APIEnabled bool
internal/infrastructure/http/router.go — register /api/v1/* group with token middleware
internal/app/app.go            — wire NATS RPC server (started alongside HTTP server)
cmd/server/main.go             — wrap existing action into `serve` subcommand; add module subcommands
```

---

## Phases

### Phase 1 — Foundation (config + JSON helpers + auth middleware)
**Status:** open

#### Actions
- [ ] 1a. Add `APIToken` field to `Config` + `--api-token` / `VVS_API_TOKEN` flag
  - File: `internal/app/config.go`, `cmd/server/main.go`
- [ ] 1b. Create `internal/infrastructure/http/jsonapi/response.go`
  - `WriteJSON(w, status, v)`, `WriteError(w, status, msg)`, envelope type
- [ ] 1c. Create `internal/infrastructure/http/apimw/token.go`
  - Reads `Authorization: Bearer <token>`, compares with config token, 401 on mismatch
- [ ] 1d. Register `/api/v1` route group in router with token middleware
  - File: `internal/infrastructure/http/router.go`
  - Pass token string into `NewRouter`

### Phase 2 — REST API handlers (per module)
**Status:** open

#### Actions
- [ ] 2a. `internal/modules/customer/adapters/http/api.go` — 5 endpoints
  - Handlers struct reuse: inject same command/query handlers already on `Handlers`
  - Or: new `APIHandlers` struct (preferred — clean separation)
- [ ] 2b. `internal/modules/product/adapters/http/api.go` — 5 endpoints
- [ ] 2c. `internal/modules/network/adapters/http/api.go` — 6 endpoints (CRUD + sync-arp)
- [ ] 2d. `internal/modules/service/adapters/http/api.go` — 5 endpoints
- [ ] 2e. `internal/modules/auth/adapters/http/api.go` — 3 endpoints (list/create/delete user)
- [ ] 2f. Wire API handlers into router's `/api/v1/` group
  - File: `internal/infrastructure/http/router.go`, `internal/app/app.go`

### Phase 3 — NATS RPC server
**Status:** open

#### Actions
- [ ] 3a. Create `internal/infrastructure/nats/rpc/handler.go`
  - `RPCRequest{Payload []byte}`, `RPCResponse{Data []byte, Error string}`
  - `Handle(subject, fn func([]byte)([]byte, error))` helper — subscribes, decodes, encodes, replies
- [ ] 3b. Create `internal/infrastructure/nats/rpc/server.go`
  - `RPCServer` struct holding all command/query handlers
  - `Register()` method subscribes all 24 subjects
  - `Close()` unsubscribes all
- [ ] 3c. Wire `RPCServer` into `app.go`
  - Started in `Build()` alongside the HTTP server
  - Closed on shutdown
  - Only active when NATS is connected

### Phase 4 — CLI subcommands
**Status:** open

#### Actions
- [ ] 4a. Create `cmd/server/cli_transport.go`
  - `Transport` interface: `Do(ctx, subject string, req, resp any) error`
  - `NATSTransport` (uses `nc.RequestWithContext`, encodes/decodes JSON)
  - `HTTPTransport` (uses `http.Client`, hits `/api/v1/`, passes `Authorization: Bearer`)
  - Factory: `NewTransport(cfg)` — NATS if `--nats-url` set, else HTTP
- [ ] 4b. Refactor `cmd/server/main.go`
  - Wrap existing server start into `vvs serve` subcommand
  - Add global flags: `--nats-url`, `--api-url`, `--api-token` (for CLI mode)
- [ ] 4c. `cmd/server/cli_customer.go` — `vvs customer {list,get,create,update,delete}`
- [ ] 4d. `cmd/server/cli_product.go` — `vvs product {list,get,create,update,delete}`
- [ ] 4e. `cmd/server/cli_router.go` — `vvs router {list,get,create,update,delete,sync-arp}`
- [ ] 4f. `cmd/server/cli_service.go` — `vvs service {list,assign,suspend,reactivate,cancel}`
- [ ] 4g. `cmd/server/cli_user.go` — `vvs user {list,create,delete}`

---

## Verification

1. `go build ./...` — clean build
2. REST API smoke:
   ```
   VVS_API_TOKEN=test ./vvs serve &
   curl -H "Authorization: Bearer test" http://localhost:8080/api/v1/customers
   curl -H "Authorization: Bearer test" -XPOST http://localhost:8080/api/v1/customers \
     -d '{"companyName":"ACME","contactName":"Joe"}'
   ```
3. NATS RPC smoke (with nats-cli or Go test):
   ```
   nats req isp.rpc.customer.list '{}'
   nats req isp.rpc.customer.create '{"companyName":"ACME"}'
   ```
4. CLI smoke:
   ```
   # via NATS
   ./vvs --nats-url nats://localhost:4222 customer list
   ./vvs --nats-url nats://localhost:4222 customer create --company ACME --contact Joe

   # via HTTP
   ./vvs --api-url http://localhost:8080 --api-token test customer list
   ```
5. Existing UI still works — no SSE regressions

---

## Progress Log

| Timestamp | Entry |
|-----------|-------|
| 2604160254 | Plan created |
