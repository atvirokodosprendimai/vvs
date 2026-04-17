---
tldr: Global task/ticket/deal indexes default to active items; tab to reveal done/closed; tasks gains search
status: active
---

# Plan: Hide Done Items by Default in Global Indexes

## Context

- `/tasks` — currently shows everything, no search; `done` + `cancelled` = terminal
- `/tickets` — has search, no status tab; `resolved` + `closed` = terminal
- `/deals` — has stage tabs + search; `won` + `lost` = terminal; default `dealStageFilter: 'all'`

## Approach

Each index gets an "Active" default tab that excludes terminal statuses and a second tab to show terminal-only.
Search stays in the header alongside the tabs.
Signal name drives both the tab UI and backend filter — changing tab re-opens the SSE with new signal values.

### Terminal statuses per entity

| Entity  | Terminal values        | Tab label |
|---------|------------------------|-----------|
| Tasks   | `done`, `cancelled`    | Done      |
| Tickets | `resolved`, `closed`   | Closed    |
| Deals   | `won`, `lost`          | Closed    |

## Phases

### Phase 1 — Tasks: search + status filter — status: open

1. [ ] Backend: `listAllSSE` reads `taskSearch` + `taskStatusFilter` signals; filter: active = exclude done/cancelled, done = only done/cancelled
   - file: `internal/modules/task/adapters/http/handlers.go`
   - signal names: `taskSearch` (string), `taskStatusFilter` ("active" | "done")
   - also update `listPage` to pass empty initial tasks so page renders immediately

2. [ ] Frontend: `TasksPage` — add `taskStatusFilter: 'active'` + `taskSearch: ''` signals; add search input + Active/Done tabs in PageHeaderRow; tabs call `@get('/api/tasks', ...)`
   - file: `internal/modules/task/adapters/http/templates.templ`

### Phase 2 — Tickets: status filter tab — status: open

3. [ ] Backend: `listAllSSE` reads `ticketStatusFilter` signal; update `filterTickets` signature; active = exclude resolved/closed, closed = only resolved/closed
   - file: `internal/modules/ticket/adapters/http/handlers.go`
   - signal name: `ticketStatusFilter` ("active" | "closed")

4. [ ] Frontend: `TicketsPage` — add `ticketStatusFilter: 'active'` signal; add Active/Closed tabs next to search in PageHeaderRow; tabs call `@get('/sse/tickets')`
   - file: `internal/modules/ticket/adapters/http/templates.templ`

### Phase 3 — Deals: change default + Active tab — status: open

5. [ ] Backend: `filterDeals` — handle `stageFilter = "active"` (exclude won/lost)
   - file: `internal/modules/deal/adapters/http/handlers.go`

6. [ ] Frontend: `DealsPage` — change default `dealStageFilter: 'active'`; replace "All" tab with "Active" tab; keep individual stage tabs + Won/Lost
   - file: `internal/modules/deal/adapters/http/templates.templ`

## Verification

- `/tasks`: default view hides done/cancelled; "Done" tab shows only done/cancelled; search filters within current tab
- `/tickets`: default view hides resolved/closed; "Closed" tab shows only resolved/closed; search still works in both tabs
- `/deals`: default view hides won/lost; individual stage tabs work; "Won"/"Lost" still filterable; "Active" tab = new+qualified+proposal+negotiation

## Progress Log

<!-- Updated after every action -->
