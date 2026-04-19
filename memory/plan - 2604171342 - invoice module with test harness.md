---
tldr: Invoice module — domain aggregate with status machine, line items from products/subscriptions, CRM tab, SSE, TDD. Preceded by shared test harness.
status: completed
---

# Plan: Invoice module with test harness

## Context

- Consilium decision (2026-04-17): unanimous vote for invoicing as next feature
- Prior plans (completed but code lost): [[plan - 2604141322 - invoice form autocomplete backend driven]], [[plan - 2604141353 - invoice line items dynamic table]]
- Related memory: [[project_consilium_invoicing.md]]
- Pattern reference: all 12 existing modules follow hexagonal DDD/CQRS (domain, commands, queries, persistence, http, migrations)

**Scope IN:** Invoice aggregate, line items, status machine, generate from subscriptions, manual create, CRM tab, SSE, NATS events
**Scope OUT:** PDF generation, payment gateways, automated dunning, proration, payment module (follow-up)

## Phases

### Phase 1 - Shared test harness — status: completed

1. [x] Create `internal/testutil/` package with helpers
   - => `db.go`: NewTestDB — temp-file SQLite, WAL, reader/writer split matching production
   - => `nats.go`: NewTestNATS — real embedded NATS, returns production Publisher/Subscriber
   - => `migration.go`: RunMigrations — goose provider with fs.FS and per-module table name
2. [x] Add integration test for existing customer module as proof
   - => `customer/app/commands/create_customer_test.go`: 3 integration tests (create, validation, sequential codes)
   - => all passing alongside 19 existing domain tests

### Phase 2 - Invoice domain — status: completed

1. [x] Define `Invoice` aggregate + `LineItem` value object in `internal/modules/invoice/domain/`
   - => `invoice.go`: full aggregate with status machine (draft→finalized→paid|void), 5 domain errors
   - => domain methods: NewInvoice, AddLineItem, RemoveLineItem, Recalculate, Finalize, MarkPaid, Void, IsOverdue
2. [x] Write domain unit tests (TDD)
   - => `invoice_test.go`: 15 test functions (19 subtests), all passing
   - => covers: transitions, line items, recalculate, edge cases
3. [x] Define `InvoiceRepository` port interface
   - => `repository.go`: Save, FindByID, ListByCustomer, ListAll, NextCode

### Phase 2b - Scaffolding — status: completed

1. [x] Database migrations
   - => `invoice/migrations/001_create_invoices.sql`: invoices, invoice_line_items, invoice_code_sequences tables
   - => `invoice/migrations/embed.go`: FS variable for embedded SQL
2. [x] NATS subjects added to `subjects.go`
   - => InvoiceAll, InvoiceCreated, InvoiceUpdated, InvoiceFinalized, InvoicePaid, InvoiceVoided
3. [x] Module directory skeleton
   - => persistence, http, commands, queries packages with empty declarations

### Phase 3 - Persistence + migrations — status: completed

1. [x] Create goose migration — done in Phase 2b
2. [x] Implement GORM repository
   - => Save (upsert with tx, Omit LineItems to avoid FK conflicts), FindByID, ListByCustomer, ListAll, NextCode
   - => Internal GORM models (invoiceModel, lineItemModel) with conversion functions
3. [x] Write integration tests — 6 tests: save/load, updates, list by customer, list all, next code sequence, not found

### Phase 4 - Commands — status: completed

1. [x] CreateInvoice — UUID + NextCode, add line items, recalculate, publish InvoiceCreated
2. [x] GenerateFromSubscriptions — ActiveServiceLister interface, line item per service, DueDate = now+30d
3. [x] FinalizeInvoice — load, Finalize(), save, publish InvoiceFinalized
4. [x] MarkPaid — load, MarkPaid(), save, publish InvoicePaid
5. [x] VoidInvoice — load, Void(), save, publish InvoiceVoided
6. [x] AddLineItem / RemoveLineItem — load, mutate, Recalculate(), save, publish InvoiceUpdated
7. [x] Command tests — 10 integration tests (happy paths + error cases)

### Phase 5 - Queries — status: completed

1. [x] ListInvoicesForCustomer — by customer ID, issue_date DESC, preload line items
2. [x] ListAllInvoices — optional status filter, created_at DESC
3. [x] GetInvoice — by ID with preloaded line items

### Phase 6 - HTTP handlers + templates — status: completed

1. [x] Register invoice module in `cmd/server/` wiring
   - => app.go: repo, 7 commands, 3 queries, HTTP handlers, migration entry
   - => Fixed email wiring regression from parallel dev agent
2. [x] Invoice list page (`GET /invoices`) with SSE live updates
   - => InvoiceListPage with status filter tabs (all/draft/finalized/paid/void)
   - => InvoiceTable with code, customer, issue date, due date, total, status badge
3. [x] Invoice detail page (`GET /invoices/{id}`) with line items table
   - => InvoiceDetailPage with invoiceStatusActions (contextual by current status)
   - => invoiceDetailContent with line items table
4. [x] Create invoice form — customer autocomplete + dynamic line items
   - => CreateInvoicePage with 3 static line item rows
5. [x] "Generate from subscriptions" — wired via WithGenerateCmd setter
6. [x] Add "Invoices" as 7th CRM tab on customer detail page
   - => CRMTabBar updated to 7 params, InvoiceSection component
   - => crmLiveSSE: added email + invoice SSE patching, fixed email count bug (was 0)
   - => Replaced hardcoded NATS strings with typed events constants
7. [x] SSE live list endpoint (`/sse/invoices`)
8. [x] NATS events already in subjects.go from Phase 2b

### Phase 7 - REST API + NATS RPC — status: open

1. [ ] REST API endpoints (`/api/v1/invoices/`)
   - GET list, GET by ID, POST create, PUT finalize, PUT mark-paid, PUT void
2. [ ] NATS RPC subjects (`isp.rpc.invoice.*`)
3. [ ] CLI subcommands (`vvs invoice list|get|create|finalize`)

## Verification

- `templ generate && go build ./...` — clean compile
- `go test ./...` — all tests pass, including new integration tests
- Manual: create customer → assign services → generate invoice → finalize → mark paid
- Manual: invoice tab visible on customer detail with live count updates
- Manual: invoice list page with status filters and SSE updates
- `grep -r '"isp\.invoice' internal/ --include='*.go'` — only subjects.go (typed constants)

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

- 2604171342: Plan created
- 2604171400: Phase 1 + 2 + 2b completed in parallel (3 dev agents in worktrees). Test harness, domain TDD, scaffolding all merged. Build clean, 22 new tests passing.
- 2604171430: Phase 3 + 4 + 5 completed in parallel (3 dev agents). GORM repo (6 tests), commands (10 tests), queries (3 handlers). All merged, build clean, all tests pass. Next: Phase 6 (HTTP/templates).
- 2604171500: Phase 6 completed. Invoice HTTP handlers (11 routes), templates, app.go wiring, CRM 7th tab. Fixed regressions: email wiring, NATS hardcoded strings, email count in tab bar, invoice field names (IssueDate/TotalAmount), status badges (finalized/void).
