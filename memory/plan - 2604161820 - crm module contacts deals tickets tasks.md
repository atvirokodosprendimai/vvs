---
tldr: Full CRM — extend Customer with lead/prospect status, add Contact/Deal/Ticket/Task satellite modules, CRM dashboard
status: completed
---

# Plan: CRM Module

## Context

Existing `internal/modules/customer/` has Customer aggregate with status (active/suspended/churned),
network provisioning fields, services, devices. All satellite modules (network, service, device) stay
as-is and keep referencing CustomerID. CRM builds on top — no wholesale replacement.

Approach: extend Customer status machine, then add four new satellite modules each as independent
hex module with own persistence + HTTP + UI.

## Phases

### Phase 1 - Spec - status: completed

1. [x] `/eidos:spec` CRM module — domain model, status machines, entity relationships
   - => [[spec - crm - customer lifecycle contacts deals tickets tasks]] created
   - => Customer: lead→prospect→active→suspended→churned + churn terminal
   - => Contact: multiple per customer, one primary
   - => Deal: lead→qualified→proposal→negotiation→won/lost (terminals)
   - => Ticket: open→in_progress→resolved→closed, priority, comments thread
   - => Task: attached to customer/deal/ticket (all optional), due date

### Phase 2 - Customer CRM expansion - status: completed

2. [x] Extend Customer status machine to include `lead` and `prospect`
3. [x] Add customer interaction/notes timeline (`customer_notes` table, AddNoteHandler)

### Phase 3 - Contact module - status: completed

4. [x] Domain: Contact aggregate
   - `internal/modules/contact/domain/contact.go`
   - fields: ID, CustomerID, FirstName, LastName, Email, Phone, Role, IsPrimary, CreatedAt
   - rules: only one primary per customer (enforced at app layer)
   - commands: AddContact, UpdateContact, RemoveContact, SetPrimary

5. [x] Persistence: GORM repo + migration `001_create_contacts.sql`
6. [x] HTTP + UI: contacts section on customer detail page

### Phase 4 - Deal module - status: completed

7. [x] Domain: Deal aggregate
8. [x] Persistence: GORM repo + migration `001_create_deals.sql`
9. [x] HTTP + UI: deals page + customer detail section

### Phase 5 - Ticket module - status: completed

10. [x] Domain: Ticket aggregate + TicketComment
11. [x] Persistence: GORM repo + migrations
12. [x] HTTP + UI: tickets page + customer detail section

### Phase 6 - Task module - status: completed

13. [x] Domain: Task aggregate
14. [x] Persistence: GORM repo + migration
15. [x] HTTP + UI: tasks widget + `/tasks` list page

### Phase 7 - CRM Dashboard - status: completed

16. [x] `/crm` overview page (CRMDashboardPage at `/crm`)
17. [x] Nav link: CRM in sidebar nav

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

- 2604161820 Phase 1 complete — CRM spec written
- 2604162100 All phases complete — Contact/Deal/Ticket/Task/Dashboard all implemented and wired; plan status updated
