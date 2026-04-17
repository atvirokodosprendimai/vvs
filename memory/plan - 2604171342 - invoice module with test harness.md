---
tldr: Invoice module — domain aggregate with status machine, line items from products/subscriptions, CRM tab, SSE, TDD. Preceded by shared test harness.
status: active
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

### Phase 1 - Shared test harness — status: open

1. [ ] Create `internal/testutil/` package with helpers
   - in-memory SQLite database factory (WAL mode, shared cache)
   - embedded NATS test server (start/stop per test)
   - `EventPublisher` / `EventSubscriber` test doubles
   - helper to run goose migrations for a given module
2. [ ] Add integration test for existing customer module as proof
   - test CreateCustomer command end-to-end (command → repo → DB)
   - validates the harness works before building invoice on it

### Phase 2 - Invoice domain — status: open

1. [ ] Define `Invoice` aggregate + `LineItem` value object in `internal/modules/invoice/domain/`
   - Invoice fields: ID, CustomerID, CustomerName, Code (INV-001 auto-sequence), IssueDate, DueDate, Status, Notes, TotalAmount, Currency, CreatedAt, UpdatedAt
   - Status machine: draft → finalized → paid | void (finalized can also → void)
   - LineItem: ID, ProductID, ProductName, Description, Quantity, UnitPrice, TotalPrice
   - Domain methods: Finalize(), MarkPaid(), Void(), AddLineItem(), RemoveLineItem(), Recalculate()
   - Validation: can't finalize with 0 line items, can't modify after finalized
2. [ ] Write domain unit tests (TDD)
   - status transitions (valid + invalid)
   - line item add/remove/recalculate
   - total calculation (sum of qty * unit_price)
   - edge cases: void from draft, void from paid (not allowed)
3. [ ] Define `InvoiceRepository` port interface

### Phase 3 - Persistence + migrations — status: open

1. [ ] Create goose migration: `invoices` + `invoice_line_items` + `invoice_code_sequences` tables
2. [ ] Implement GORM repository (`internal/modules/invoice/adapters/persistence/`)
3. [ ] Write integration tests using Phase 1 test harness
   - save/load invoice with line items
   - list by customer, list by status
   - code auto-increment (INV-001, INV-002...)

### Phase 4 - Commands — status: open

1. [ ] `CreateInvoice` command — manual creation with customer ID + line items
2. [ ] `GenerateFromSubscriptions` command — create invoice from customer's active services
   - query active services for customer, create line item per service
   - set issue date = now, due date = now + 30 days
3. [ ] `FinalizeInvoice` command — lock invoice, set status
4. [ ] `MarkPaid` command — record payment date
5. [ ] `VoidInvoice` command — cancel invoice
6. [ ] `AddLineItem` / `RemoveLineItem` commands — modify draft invoices
7. [ ] Write command tests (TDD with test harness)

### Phase 5 - Queries — status: open

1. [ ] `ListInvoicesForCustomer` query — by customer ID, ordered by issue date desc
2. [ ] `ListAllInvoices` query — global list, filterable by status/date range
3. [ ] `GetInvoice` query — single invoice with line items

### Phase 6 - HTTP handlers + templates — status: open

1. [ ] Register invoice module in `cmd/server/` wiring
2. [ ] Invoice list page (`GET /invoices`) with SSE live updates
   - table: code, customer, issue date, due date, total, status badge
   - filter by status (all/draft/finalized/paid/overdue/void)
3. [ ] Invoice detail page (`GET /invoices/{id}`) with line items table
   - status actions: Finalize, Mark Paid, Void (contextual by current status)
4. [ ] Create invoice form — customer autocomplete + dynamic line items
   - reuse pattern from prior plans: backend-driven autocomplete
   - product search per line item, auto-fill unit price from product
   - add/remove line rows dynamically
5. [ ] "Generate from subscriptions" button on customer detail page
   - one-click: creates draft invoice from active services
6. [ ] Add "Invoices" as 7th CRM tab on customer detail page
   - update `CRMTabBar` counts, `crmLiveSSE` state struct
7. [ ] SSE live list endpoint (`/sse/invoices`)
8. [ ] NATS events: add `InvoiceCreated`, `InvoiceFinalized`, `InvoicePaid`, `InvoiceVoided` to subjects.go

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

<!-- Timestamped entries tracking work done. Updated after every action. -->
