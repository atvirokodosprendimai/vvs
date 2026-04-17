---
tldr: Clicking deal/ticket/task title in global index opens customer detail at the matching CRM tab
status: completed
---

# Plan: Deal/Ticket/Task Rows Link to Customer Tab

## Context

- `/deals`, `/tickets`, `/tasks` are global index pages that aggregate records across all customers
- Customer detail page (`/customers/{id}`) uses `?tab=deals` / `?tab=tickets` / `?tab=tasks` for tab routing
- Currently: deal title → `/customers/{id}` (no tab), ticket subject → `/tickets/{id}` (detail page), task title → no link

## Phases

### Phase 1 — Update row links — status: completed

1. [x] Fix `dealPageRow` in `internal/modules/deal/adapters/http/templates.templ`
   - => deal title: `/customers/{id}` → `/customers/{id}?tab=deals`
   - => customer name column stays `/customers/{id}` (general link)

2. [x] Fix `AllTicketList` in `internal/modules/ticket/adapters/http/templates.templ`
   - => subject: `/tickets/{id}` → `/customers/{id}?tab=tickets` when `CustomerID != ""`
   - => fallback to `/tickets/{id}` when no customer

3. [x] Fix `globalTaskRow` in `internal/modules/task/adapters/http/templates.templ`
   - => title: plain text → `<a>` to `/customers/{id}?tab=tasks` when `CustomerID != ""`
   - => customer column: `/customers/{id}` → `/customers/{id}?tab=tasks`

## Verification

- Click deal title in `/deals` → lands on `/customers/{uuid}?tab=deals`, deals tab active
- Click ticket subject in `/tickets` → lands on `/customers/{uuid}?tab=tickets`, tickets tab active
- Click task title in `/tasks` → lands on `/customers/{uuid}?tab=tasks`, tasks tab active
- Items with no customer: ticket falls back to `/tickets/{id}`, task title is plain text

## Progress Log

- **2604180106** — All 3 rows updated. Pure template change, no backend needed.
