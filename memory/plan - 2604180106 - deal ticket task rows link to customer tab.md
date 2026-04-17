---
tldr: Clicking deal/ticket/task title in global index opens customer detail at the matching CRM tab
status: active
---

# Plan: Deal/Ticket/Task Rows Link to Customer Tab

## Context

- `/deals`, `/tickets`, `/tasks` are global index pages that aggregate records across all customers
- Customer detail page (`/customers/{id}`) uses `?tab=deals` / `?tab=tickets` / `?tab=tasks` for tab routing
- Currently: deal title → `/customers/{id}` (no tab), ticket subject → `/tickets/{id}` (detail page), task title → no link

## Phases

### Phase 1 — Update row links — status: active

1. [ ] Fix `dealPageRow` in `internal/modules/deal/adapters/http/templates.templ`
   - Deal title `<a>`: `/customers/{id}` → `/customers/{id}?tab=deals`
   - Customer name link stays as `/customers/{id}` (no tab — keeps it as a general link)

2. [ ] Fix `AllTicketList` in `internal/modules/ticket/adapters/http/templates.templ`
   - Subject `<a>`: currently `/tickets/{id}` → `/customers/{id}?tab=tickets` when `tk.CustomerID != ""`
   - Keep `/tickets/{id}` fallback when no customer

3. [ ] Fix `globalTaskRow` in `internal/modules/task/adapters/http/templates.templ`
   - Title `<td>`: currently plain text → add `<a>` to `/customers/{id}?tab=tasks` when `t.CustomerID != ""`
   - Customer column link: `/customers/{id}` → `/customers/{id}?tab=tasks`

## Verification

- Click deal title in `/deals` → lands on `/customers/{uuid}?tab=deals`, deals tab active
- Click ticket subject in `/tickets` → lands on `/customers/{uuid}?tab=tickets`, tickets tab active
- Click task title in `/tasks` → lands on `/customers/{uuid}?tab=tasks`, tasks tab active
- Items with no customer: ticket falls back to `/tickets/{id}`, task title is plain text

## Progress Log

<!-- Updated after every action -->
