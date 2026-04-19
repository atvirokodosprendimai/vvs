---
tldr: Global task/ticket/deal indexes default to active items; tab to reveal done/closed; tasks gains search
status: completed
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

### Phase 1 — Tasks: search + status filter — status: completed

1. [x] Backend: `listAllSSE` reads `taskSearch` + `taskStatusFilter` signals; `filterTasks` helper added
   - => `listPage` no longer pre-fetches tasks; `TasksPage()` takes no args
   - => `filterTasks`: active = exclude done/cancelled, done = only done/cancelled

2. [x] Frontend: `TasksPage()` — Active/Done tabs + search input + signals in PageHeaderRow

### Phase 2 — Tickets: status filter tab — status: completed

3. [x] Backend: `filterTickets` updated to accept statusFilter; `domain` import added
   - => active = exclude resolved/closed, closed = only resolved/closed

4. [x] Frontend: `TicketsPage` — `ticketStatusFilter: 'active'` signal + Active/Closed tabs; search width reduced to w-48

### Phase 3 — Deals: change default + Active tab — status: completed

5. [x] Backend: `filterDeals` — `"active"` excludes won/lost; `""` and `"all"` show everything

6. [x] Frontend: `DealsPage` — default `dealStageFilter: 'active'`; "All" tab → "Active" tab

## Verification

- `/tasks`: default view hides done/cancelled; "Done" tab shows only done/cancelled; search filters within current tab
- `/tickets`: default view hides resolved/closed; "Closed" tab shows only resolved/closed; search still works in both tabs
- `/deals`: default view hides won/lost; individual stage tabs work; "Won"/"Lost" still filterable; "Active" tab = new+qualified+proposal+negotiation

## Progress Log

- **2604180109** — All 3 modules done in one pass. 6 files changed (3 handlers + 3 templates).
