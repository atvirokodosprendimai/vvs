---
tldr: CRM module — Customer lifecycle (lead→churned), Contacts, Deals, Tickets, Tasks as satellite hex modules
---

# CRM

Full customer relationship management layered on top of the existing Customer module.
Network provisioning, services, and devices remain in their own satellite modules — they reference CustomerID but are not owned by CRM.

## Target

Operators managing the full customer lifecycle: from first contact as a lead through active service, and across the support/sales surface (deals, tickets, tasks). Replaces the existing Customer list/detail UI with a richer CRM-aware view.

## Behaviour

### Customer lifecycle

- Customer status follows: `lead` → `prospect` → `active` → `suspended` → `churned`
- Transitions:
  - `Qualify()` — lead → prospect
  - `Convert()` — prospect → active
  - `Suspend()` — active → suspended (existing)
  - `Activate()` — suspended → active (existing)
  - `Churn()` — any non-churned → churned
- Existing customers in the DB carry `active` status — no migration needed
- Customer detail page shows a notes/interaction timeline (append-only log)

### Contacts

- A Contact is a named person at a customer organisation
- Each customer can have multiple contacts; at most one is marked `is_primary`
- Setting a new contact as primary clears the previous primary for that customer (enforced at app layer)
- Fields: FirstName, LastName, Email, Phone, Role, IsPrimary
- Contacts cannot be transferred between customers

### Deals

- A Deal tracks a sales opportunity attached to a customer
- Stages (in order): `lead` → `qualified` → `proposal` → `negotiation` → `won` | `lost`
- `won` and `lost` are terminal — a closed deal cannot be re-opened
- Value stored in cents (int64) with currency string; display as decimal
- Each deal has an optional close date and free-text notes
- Deals list page shows a Kanban pipeline view (columns per stage)

### Tickets

- A Ticket is a support request from or about a customer
- Statuses: `open` → `in_progress` → `resolved` → `closed`
- `resolved` can revert to `in_progress` (Reopen); `closed` is terminal
- Priority levels: `low` | `medium` | `high` | `urgent`
- A ticket may reference a Contact (the reporter)
- Ticket comments form an append-only thread; each comment has an author and timestamp
- Assignee is a system user ID (string); unassigned = empty string

### Tasks

- A Task is a follow-up reminder; it can be attached to a Customer, Deal, or Ticket (all optional)
- Statuses: `open` | `done`
- Has an optional due date and assignee (user ID)
- Overdue = due date in the past and status is `open`
- Tasks without a link (no customer/deal/ticket) are global tasks

## Design

### Module structure

Four new hex modules alongside the extended customer module:

```
internal/modules/customer/   ← extended: lead/prospect status + notes timeline
internal/modules/contact/    ← Contact aggregate
internal/modules/deal/       ← Deal aggregate
internal/modules/ticket/     ← Ticket + TicketComment
internal/modules/task/       ← Task aggregate
```

Each follows the standard hexagonal shape: `domain/`, `app/commands/`, `app/queries/`, `adapters/persistence/`, `adapters/http/`, `migrations/`.

Cross-module reads (e.g. customer name on deal list) go through the shared SQLite reader (`gdb.R.Raw(...)`), never through another module's domain layer — consistent with [[spec - architecture - system design and key decisions]].

### Status machines

```
Customer:  lead ──qualify──▶ prospect ──convert──▶ active ──suspend──▶ suspended
                                                      ▲                     │
                                                      └────activate──────────┘
           any (non-churned) ──churn──▶ churned (terminal)

Deal:      lead → qualified → proposal → negotiation → won (terminal)
                                                      → lost (terminal)

Ticket:    open → in_progress → resolved → closed (terminal)
                      ▲             │
                      └──reopen─────┘
```

### Events

Each module publishes NATS events on mutation — subjects follow `isp.<module>.<verb>` convention per [[spec - events - event driven module boundaries and nats subject taxonomy]]:

- `isp.customer.qualified`, `isp.customer.converted`, `isp.customer.churned`
- `isp.contact.added`, `isp.contact.removed`
- `isp.deal.created`, `isp.deal.stage_advanced`, `isp.deal.won`, `isp.deal.lost`
- `isp.ticket.opened`, `isp.ticket.assigned`, `isp.ticket.resolved`, `isp.ticket.closed`
- `isp.task.created`, `isp.task.completed`

### CRM Dashboard `/crm`

Aggregates read views from all modules:
- Pipeline summary: deal count + total value per stage
- Open ticket count grouped by priority
- Tasks due today / overdue for current user
- Recent activity feed: last N notes + status changes across all customers

## Verification

1. `go test ./internal/modules/customer/... ./internal/modules/contact/... ./internal/modules/deal/... ./internal/modules/ticket/... ./internal/modules/task/... -race` — all green
2. `templ generate && go build ./...` — clean
3. Browser flow:
   - Create lead → qualify → convert to active customer
   - Add two contacts, set one as primary
   - Create deal for customer → advance to proposal → mark won
   - Open support ticket → assign → add comment → resolve
   - Create task linked to ticket → mark done
   - `/crm` dashboard reflects all above

## Interactions

- Depends on [[spec - architecture - system design and key decisions]] — hexagonal structure, CQRS, single-writer
- Depends on [[spec - events - event driven module boundaries and nats subject taxonomy]] — NATS subject naming
- Coexists with [[spec - network - mikrotik arp provisioning with netbox]] — network module reads CustomerID, unaffected by CRM status changes

## Mapping

> [[internal/modules/customer/domain/customer.go]]
> [[internal/modules/customer/adapters/http/handlers.go]]
> [[internal/modules/customer/adapters/http/templates.templ]]

## Future

{[!] Email integration — log inbound/outbound emails as timeline entries}
{[!] SLA timers on tickets — escalate when high/urgent breaches threshold}
{[?] Bulk lead import from CSV}
{[?] Deal revenue forecasting by close date}
