---
tldr: Full CRM — extend Customer with lead/prospect status, add Contact/Deal/Ticket/Task satellite modules, CRM dashboard
status: active
---

# Plan: CRM Module

## Context

Existing `internal/modules/customer/` has Customer aggregate with status (active/suspended/churned),
network provisioning fields, services, devices. All satellite modules (network, service, device) stay
as-is and keep referencing CustomerID. CRM builds on top — no wholesale replacement.

Approach: extend Customer status machine, then add four new satellite modules each as independent
hex module with own persistence + HTTP + UI.

## Phases

### Phase 1 - Spec - status: open

1. [ ] `/eidos:spec` CRM module — domain model, status machines, entity relationships
   - Customer status expansion: lead → prospect → active → suspended → churned
   - Contact: person at customer, multiple per customer, primary flag
   - Deal: sales pipeline, stages, value
   - Ticket: support, status/priority/SLA, assignee
   - Task: follow-up reminder attached to any of the above

### Phase 2 - Customer CRM expansion - status: open

2. [ ] Extend Customer status machine to include `lead` and `prospect`
   - add `StatusLead`, `StatusProspect` consts
   - add `Qualify()` (lead→prospect), `Convert()` (prospect→active) methods
   - update status badge in templates
   - migration: existing customers stay `active` — no data change needed

3. [ ] Add customer interaction/notes timeline
   - `CustomerNote` entity: ID, CustomerID, Body, AuthorID, CreatedAt
   - own table `customer_notes`; append-only (no update/delete)
   - displayed in customer detail page as a feed

### Phase 3 - Contact module - status: open

4. [ ] Domain: Contact aggregate
   - `internal/modules/contact/domain/contact.go`
   - fields: ID, CustomerID, FirstName, LastName, Email, Phone, Role, IsPrimary, CreatedAt
   - rules: only one primary per customer (enforced at app layer)
   - commands: AddContact, UpdateContact, RemoveContact, SetPrimary

5. [ ] Persistence: GORM repo + migration `001_create_contacts.sql`

6. [ ] HTTP + UI: contacts section on customer detail page
   - `GET /sse/customers/{id}/contacts` — SSE live list
   - `POST /api/contacts` — add
   - `PUT /api/contacts/{id}` — update
   - `DELETE /api/contacts/{id}` — remove
   - inline table + add/edit modal in customer detail page

### Phase 4 - Deal module - status: open

7. [ ] Domain: Deal aggregate
   - `internal/modules/deal/domain/deal.go`
   - fields: ID, CustomerID, Title, Value (int64 cents), Currency, Stage, Notes, CloseDate, CreatedAt
   - stages: `lead` → `qualified` → `proposal` → `negotiation` → `won` | `lost`
   - commands: CreateDeal, UpdateDeal, AdvanceStage, MarkWon, MarkLost

8. [ ] Persistence: GORM repo + migration `001_create_deals.sql`

9. [ ] HTTP + UI: deals page + customer detail section
   - `/deals` list page (all deals, filterable by stage/customer)
   - `GET /sse/customers/{id}/deals` — live deal list on customer detail
   - add/edit modal, stage advancement buttons
   - pipeline board view on `/deals` (columns per stage)

### Phase 5 - Ticket module - status: open

10. [ ] Domain: Ticket aggregate
    - `internal/modules/ticket/domain/ticket.go`
    - fields: ID, CustomerID, ContactID (opt), Title, Body, Status, Priority, AssigneeID, ResolvedAt
    - status: `open` → `in_progress` → `resolved` → `closed`
    - priority: `low` | `medium` | `high` | `urgent`
    - commands: OpenTicket, Assign, Reopen, Resolve, Close
    - TicketComment: append-only thread per ticket

11. [ ] Persistence: GORM repo + migrations for tickets + ticket_comments

12. [ ] HTTP + UI: tickets page + customer detail section
    - `/tickets` list page (filter by status, priority, assignee)
    - `GET /sse/customers/{id}/tickets` — live list
    - ticket detail page with comment thread
    - add/reply modal

### Phase 6 - Task module - status: open

13. [ ] Domain: Task aggregate
    - `internal/modules/task/domain/task.go`
    - fields: ID, Title, DueAt, Done, AssigneeID, CustomerID (opt), DealID (opt), TicketID (opt), CreatedAt
    - commands: CreateTask, Complete, Reopen, Reassign

14. [ ] Persistence: GORM repo + migration

15. [ ] HTTP + UI: tasks widget
    - `/tasks` list page (my tasks, overdue, all)
    - task creation from customer/deal/ticket pages
    - mini task list on customer detail page

### Phase 7 - CRM Dashboard - status: open

16. [ ] `/crm` overview page
    - pipeline summary (deal counts + value by stage)
    - open ticket count by priority
    - tasks due today / overdue
    - recent customer activity feed (notes + status changes)

17. [ ] Nav link: add CRM entry to sidebar nav

## Verification

1. `go test ./internal/modules/customer/... ./internal/modules/contact/... ./internal/modules/deal/... ./internal/modules/ticket/... ./internal/modules/task/... -race` — all pass
2. `templ generate && go build ./...` — clean
3. Browser flow:
   - Create a lead → qualify to prospect → convert to active
   - Add contact to customer
   - Open deal → advance to proposal → mark won
   - Open ticket → add comment → resolve
   - Create task from ticket → mark done
   - `/crm` dashboard shows correct summary

## Adjustments

## Progress Log
