---
tldr: ISP business management system — single Go binary, hexagonal architecture, CQRS, embedded SQLite + NATS, reactive SSE frontend
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
> [[AGENTS.md]]
