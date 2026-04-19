---
tldr: Customers open and track support tickets from vvs-portal via NATS RPC bridge — 3 new subjects, portal HTTP + templates
status: completed
---

# Plan: Customer Portal — Support Ticket Submission

## Context

Consilium 2026-04-19 priority #6 (CEO: "reduces inbound phone/email volume, single queue for staff").

Portal today: magic-link auth, invoice list + PDF download, customer header. No support channel.
CRM tickets exist fully in core (open/comment/status/list-for-customer). Bridge pattern proven.

**Bridge pattern reference:**
- Core: `internal/modules/portal/adapters/nats/bridge.go` — `PortalBridge` subscribes to `isp.portal.rpc.*`
- Portal: `internal/modules/portal/adapters/nats/client.go` — `PortalNATSClient.rpc()` helper

**Existing ticket building blocks:**
- `OpenTicketCommand{CustomerID, Subject, Body, Priority}` + `OpenTicketHandler`
- `ListTicketsForCustomerHandler` → `[]TicketReadModel`
- `GetTicketHandler` → `*TicketReadModel` (with `Comments []CommentReadModel`)
- `AddCommentHandler` + `AddCommentCommand{TicketID, Body, AuthorID}`

- Consilium backlog: [[project_consilium_backlog_priority]] — #6

## Phases

### Phase 1 — Bridge: 3 new NATS RPC subjects (core side) — status: open

1. [ ] Add 3 new subject constants to `bridge.go`
   - `SubjectTicketsList   = "isp.portal.rpc.tickets.list"`
   - `SubjectTicketOpen    = "isp.portal.rpc.ticket.open"`
   - `SubjectTicketComment = "isp.portal.rpc.ticket.comment.add"`

2. [ ] Add ticket interfaces + fields to `PortalBridge` struct
   - `ticketLister  ticketListerForCustomer` — wraps `ListTicketsForCustomerHandler`
   - `openTicket    ticketOpener` — wraps `OpenTicketHandler`
   - `addComment    ticketCommenter` — wraps `AddCommentHandler`
   - Define minimal local interfaces (same pattern as `invoicesByCustomerLister`)

3. [ ] Implement 3 bridge handlers
   - `handleTicketsList`: req `{customerID}` → list tickets filtered by customerID (ownership enforced)
   - `handleTicketOpen`: req `{customerID, subject, body, priority}` → `OpenTicketCommand` → return new ticket ID
   - `handleTicketCommentAdd`: req `{customerID, ticketID, body}` → verify ticket.CustomerID == customerID → `AddCommentCommand`
   - All 3 require non-empty `customerID` → `errForbidden` if missing

4. [ ] Register 3 handlers in `PortalBridge.Register()`

5. [ ] Write bridge unit tests (extend `bridge_test.go`)
   - list: returns only tickets for that customer
   - open: creates ticket, returns ID
   - comment: rejects if ticket belongs to different customer
   - all 3: reject empty customerID

### Phase 2 — Client: `PortalNATSClient` methods (portal side) — status: open

1. [ ] Add 3 methods to `PortalNATSClient` in `client.go`
   ```go
   func (c *PortalNATSClient) ListTickets(ctx context.Context, customerID string) ([]PortalTicket, error)
   func (c *PortalNATSClient) OpenTicket(ctx context.Context, customerID, subject, body, priority string) (string, error)
   func (c *PortalNATSClient) AddTicketComment(ctx context.Context, customerID, ticketID, body string) error
   ```
   - Define `PortalTicket` + `PortalTicketComment` structs (minimal: ID, Subject, Status, Priority, CreatedAt, Comments)

2. [ ] Unit test client: happy path + error envelope propagation

### Phase 3 — Portal HTTP + Templates — status: open

1. [ ] Add ticket routes to portal HTTP handlers
   - `GET  /portal/tickets`        → list tickets for authenticated customer
   - `POST /portal/tickets`        → open ticket (form: subject, body, priority select)
   - `GET  /portal/tickets/{id}`   → ticket detail + comments
   - `POST /portal/tickets/{id}/comment` → add comment (SSE or redirect)
   - All require portal auth middleware (customerID from context)

2. [ ] Add ticket list + open-form template (`portal_tickets.templ`)
   - List: table of tickets (subject, status badge, priority, date)
   - Empty state: "No tickets yet — open one below"
   - Open form: subject input, body textarea, priority select (low/medium/high)
   - Submit via Datastar `@post` → SSE redirect to detail on success

3. [ ] Add ticket detail template (`portal_ticket_detail.templ`)
   - Show ticket fields: subject, status badge, priority, opened date
   - Comments thread (chronological)
   - Add comment form at bottom

4. [ ] Add "Support" nav link to portal layout
   - Extend portal nav component; link to `/portal/tickets`

### Phase 4 — Wiring — status: open

1. [ ] Pass ticket handlers to `PortalBridge` in `wire_infra.go`
   - `openTicketCmd` (from `wire_crm.go`'s `crmWired`)
   - `listTicketsForCustomerQuery` (from `wire_crm.go`)
   - `addCommentCmd` (from `wire_crm.go`)

2. [ ] Pass `PortalNATSClient` ticket methods to portal HTTP handlers in `cmd/portal/main.go`

3. [ ] `go build ./... && go test ./internal/modules/portal/...`

## Verification

```bash
go test ./internal/modules/portal/adapters/nats/... -v
go build ./cmd/portal/ ./cmd/vvs-core/
# Log in to portal as customer
# Navigate to /portal/tickets → see empty state
# Open ticket → appears in list + in core CRM under customer's tickets tab
# Staff adds comment in core → visible in portal ticket detail
# Customer adds comment in portal → visible in core CRM ticket
```

## Adjustments

## Progress Log
