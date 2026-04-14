# Pull — system design and decisions overview

## Targets
All Go source under `internal/` (~70 files, 8 distinct concerns)

## Territory Map

### 1. System architecture
- Single binary: Go + embedded NATS + embedded SQLite (no external services)
- Hexagonal architecture: domain ← application ← adapters (ports & adapters)
- CQRS with separated write serializer and read-only DB connection
- Composition root in `internal/app/app.go` — wires all modules

### 2. Shared kernel
- `Money` value object: int64 cents + ISO 4217 string, never floats
- `CompanyCode` value object: prefix-NNNNN format (e.g. CLI-00001)
- `Pagination` helper
- Generic `CommandHandler[C]` / `QueryHandler[Q,R]` interfaces
- `DomainEvent` + `EventPublisher` / `EventSubscriber` interfaces

### 3. Infrastructure
- **SQLite**: two connections — writer (WAL) via `WriteSerializer` goroutine, reader (WAL read-only)
- **WriteSerializer**: channel-based, buffers 64 requests, single goroutine, each write is a transaction
- **NATS**: embedded server + client; subjects follow `isp.{module}.{event}`
- **HTTP**: chi router, chi middleware (Logger, Recoverer, RealIP), SSE via Datastar Go SDK
- **Migrations**: Goose per-module with separate version tables (`goose_{module}`)

### 4. Customer module
- Aggregate: `Customer` with status (active/suspended/churned), TaxID field used for itax.lt matching
- Sequential company codes: atomic sequence via DB UPDATE RETURNING
- Commands: create, update, delete; publishes `isp.customer.{created,updated,deleted}`

### 5. Product module
- Aggregate: `Product` with types (internet/voip/hosting/custom), Money price + billing period
- Commands: create, update, delete

### 6. Invoice module
- Aggregate: `Invoice` with `InvoiceLine[]`, status lifecycle: draft → finalized → paid/void
- Auto-recalculate on line add/remove: subtotal → taxAmount (21% default) → total
- Invoice numbers: INV-{YEAR}-{NNNNN} sequential
- Linked to RecurringInvoice via optional `RecurringID`

### 7. Recurring module
- Aggregate: `RecurringInvoice` with `Schedule{Frequency, DayOfMonth}` and `RecurringLine[]`
- Frequencies: monthly/quarterly/yearly; DayOfMonth capped at 28
- Status: active/paused/cancelled; `IsDue(asOf)` predicate for job scheduling
- `calculateNextRunDate` advances by one period when current period already passed

### 8. Payment module
- Aggregate: `Payment` with statuses: imported → matched/unmatched/manually_matched
- SEPA CSV import via `importers.Importer` interface (pluggable format)
- Manual matching: operator links payment to invoice + customer

### 9. Debt management
- `DebtStatus` per customer: `OverCreditBudget bool`, synced from itax.lt
- `DebtorProvider` port interface: `FetchDebtors(ctx) → []DebtorRecord{ClientCode, OverCreditBudget}`
- Matching by customer `TaxID` field (Lithuanian company registration code)
- Stub implementation live; HTTP client skeleton ready for real credentials
- SSE handler keeps connection open, re-renders on `isp.debt.synced` NATS event

### 10. Frontend / UI
- Datastar v1 RC.8 SSE: `data-signals`, `data-bind:field`, `data-init="@get(...)"`, `data-on:submit__prevent`
- `ReadSignals` MUST be called before `NewSSE` (NewSSE closes request body)
- `PatchElementTempl` morphs DOM by element `id`
- Templ type-safe HTML components; dark mode with orange accents (Tailwind v4 CDN)
- Long-lived SSE handlers subscribe to NATS and re-render on events

## Existing Specs
None (eidos just initialised)

## Subsections for Follow-up

| # | Concern | Files |
|---|---------|-------|
| A | System architecture + infrastructure | app/, infrastructure/ |
| B | Shared kernel | shared/ |
| C | Customer + Product modules | modules/customer/, modules/product/ |
| D | Invoice + Recurring modules | modules/invoice/, modules/recurring/ |
| E | Payment module | modules/payment/ |
| F | Debt + itax.lt integration | modules/debt/, infrastructure/itaxtlt/ |
| G | Frontend / UI design system | infrastructure/http/*.templ, adapters/http/*.templ |
