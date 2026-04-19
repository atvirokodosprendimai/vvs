---
tldr: ISP business management system — hexagonal architecture, CQRS, embedded SQLite + NATS, reactive SSE frontend; deployable as single binary or split core+portal
category: core
---

# Architecture

## Target
Operators managing an ISP business: customers, products, invoices, recurring billing, payments, credit risk.

## Behaviour
- The system runs as a single self-contained binary — no external databases, message brokers, or build steps required to start
- All business data persists across restarts
- The UI updates in real-time when another operator makes a change — no page refresh needed
- Writes never block reads; the UI stays responsive under concurrent use
- Any module can be worked on independently without touching others

## Design

### Single binary
Go binary embeds SQLite (pure-Go, no CGo), NATS server, Goose migrations, and all static assets. Deployment is `scp + systemctl restart`.

### Hexagonal architecture
Each module is a bounded context with the same internal shape:
```
domain/         ← pure Go, no framework deps, TDD
app/commands/   ← write side: validates, delegates to WriteSerializer, publishes NATS event
app/queries/    ← read side: queries reader DB directly, returns read models
adapters/
  persistence/  ← GORM models + repository implementation
  http/         ← Datastar SSE handlers + templ templates
migrations/     ← Goose SQL, embedded via go:embed, separate version table per module
```
Modules share only `internal/shared/` primitives. Cross-module reads go through the shared SQLite reader, never through another module's domain layer.

### CQRS with single writer
- **Write path**: HTTP handler → ReadSignals → Command → `WriteSerializer.Execute` → SQLite → publish NATS event
- **Read path (initial)**: `data-init="@get(...)"` → SSE opens → Query → reader DB → `PatchElementTempl`
- **Read path (live)**: NATS event arrives at open SSE handler → re-query → `PatchElementTempl`
- `WriteSerializer` is a channel-buffered goroutine (capacity 64) — every write is serialised through it and wrapped in a transaction. This eliminates SQLite "database is locked" errors.
- Reader uses a separate `*gorm.DB` opened in WAL read-only mode; many concurrent reads are safe.

### Event bus
NATS embedded server. Subject convention: `isp.{module}.{event}` (e.g. `isp.invoice.finalized`).
Events are published after a successful write. SSE handlers use `ChanSubscription(subject)` which returns a channel + cancel func — caller must `defer cancel()` to avoid subscription leaks.

### Reactive frontend
Datastar v1 RC.8 over SSE. The server owns all HTML; the browser morphs DOM by element `id`.
- `data-init="@get('/api/...')"` — opens SSE connection on element mount
- `data-on:submit__prevent="@post(...)"` — sends all signals as JSON body
- `ReadSignals` extracts signal JSON; **must be called before `NewSSE`** (NewSSE closes the request body)
- Long-lived GET handlers subscribe to NATS and call `PatchElementTempl` on each relevant event, keeping the browser view live without polling

### Migrations
Goose runs automatically at startup per module with isolated version tables (`goose_customer`, `goose_invoice`, etc.). Modules can evolve their schema independently.

## Interactions
- `internal/app/app.go` is the only composition root — all wiring happens there
- `infrastructure/itaxtlt.DebtorProvider` is a port; `StubDebtorProvider` is active until real credentials are configured; swap one line in `app.go`
- Modules communicate exclusively through NATS events — no direct function calls across module boundaries

## Deployment Modes

### Single binary (default, development)
One process, one port. Admin dashboard + customer portal on the same host. Suitable when internal access only or during development.

### Split deployment (production with public portal)
Two binaries on separate hosts — hard network boundary:

```
[ Office / NATed LAN ]                    [ Public VPS ]
┌─────────────────────────┐               ┌───────────────────────────────┐
│  cmd/server (vvs-core)  │◄──WireGuard──►│  cmd/portal (vvs-portal)      │
│  - admin HTTP :8080      │    NATS RPC   │  - portal HTTP :8081           │
│  - SQLite (all data)     │               │  - NO DB                       │
│  - embedded NATS :4222   │               │  - NATS client only            │
│  - NOT internet-facing   │               │  - /portal/* + /i/{token}      │
└─────────────────────────┘               └───────────────────────────────┘
```

**`cmd/server` (core)** — all admin logic, SQLite, embedded NATS exposed on WireGuard interface. Never internet-facing.

**`cmd/portal`** — customer portal only. No DB. Connects to core's NATS as a client. Serves `/portal/*` and `/i/{token}`. All data fetched from core via NATS request/reply.

### NATS Portal RPC

The portal binary communicates with core using 6 request/reply subjects (`isp.portal.rpc.*`), served by `PortalBridge` in core:

| Subject | What it does |
|---------|-------------|
| `isp.portal.rpc.token.validate` | Validate portal session token hash → customerID |
| `isp.portal.rpc.invoices.list` | List invoices for a customer |
| `isp.portal.rpc.invoice.get` | Get invoice detail (with ownership check) |
| `isp.portal.rpc.invoice.token.validate` | Validate public PDF token → invoiceID |
| `isp.portal.rpc.invoice.token.mint` | Mint a new public PDF token |
| `isp.portal.rpc.customer.get` | Get customer name/email for portal header |

### Security (split mode)
- NATS bound to WireGuard interface only (`10.8.0.1:4222`) — never public internet
- Optional `--nats-auth-token` for additional protection
- Nginx on VPS terminates TLS; rate-limits `/portal/auth`

## Mapping
> [[internal/app/app.go]]
> [[internal/infrastructure/database/writer.go]]
> [[internal/infrastructure/database/sqlite.go]]
> [[internal/infrastructure/nats/subscriber.go]]
> [[internal/infrastructure/nats/publisher.go]]
> [[internal/infrastructure/http/router.go]]
> [[internal/infrastructure/itaxtlt/provider.go]]
> [[internal/shared/events/event.go]]
> [[internal/shared/cqrs/command.go]]
> [[cmd/portal/main.go]]
> [[internal/modules/portal/adapters/nats/bridge.go]]
> [[internal/modules/portal/adapters/nats/client.go]]
> [[AGENTS.md]]
