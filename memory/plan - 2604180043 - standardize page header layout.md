---
tldr: New PageHeaderRow component — title left, filters+buttons right at same height — migrated across all main list pages
status: active
---

# Plan: Standardize Page Header Layout

## Context

- Shared components: `internal/infrastructure/http/templates/components.templ`
- Problem: `PageHeader(title, subtitle)` only renders title+subtitle. Every list page then adds its own separate `flex items-center justify-between mb-4` row for filters/buttons, each with slightly different spacing, button sizes, and structure.
- Goal: one row — title+subtitle on the left, filters+action buttons on the right, all at the same vertical height.

### New component: `PageHeaderRow`

```go
templ PageHeaderRow(title, subtitle string) {
    <div class="flex items-center justify-between mb-6">
        <div>
            <h1 ...>{ title }</h1>
            <p ...>{ subtitle }</p>   // omitted when empty
        </div>
        <div class="flex items-center gap-3">
            { children... }           // filters + buttons go here
        </div>
    </div>
}
```

`PageHeader` stays for pages with no actions (Dashboard, CRM overview, detail/form pages).

### Scope

**Migrate** (main list pages with actions):
- `/customers` — search input + New Customer button
- `/tickets` — search input + New Ticket button
- `/tasks` — search input + filter + New Task button
- `/deals` (standalone) — filter + New Deal button
- `/invoices` — status tab filters + New Invoice button
- `/products` — search input + type dropdown + New Product button
- `/routers` — New Router button
- `/devices` — action button (TBD after reading template)
- `/prefixes` — action button (TBD after reading template)
- `/users` — New User button
- `/cron` — New Job button
- `/attachments` — search input

**Keep `PageHeader`** (no action row needed):
- `/` Dashboard
- `/crm` CRM overview
- Customer/Product/Router detail pages
- Form/edit pages
- Email thread page
- Email settings

---

## Phases

### Phase 1 — Add `PageHeaderRow` component — status: active

1. [ ] Add `PageHeaderRow(title, subtitle string)` to `components.templ`
   - Left: title + subtitle (same markup as existing `PageHeader`)
   - Right: `{ children... }` slot in `flex items-center gap-3`
   - Full row: `flex items-center justify-between mb-6`

### Phase 2 — Migrate CRM modules — status: open

2. [ ] Migrate `/customers` list page
   - Replace `PageHeader` + separate filter row with `PageHeaderRow { search + New Customer }`

3. [ ] Migrate `/tickets` and `/tasks` standalone pages
   - Tickets: search + New Ticket
   - Tasks: search + status filter + New Task

4. [ ] Migrate `/deals` standalone page
   - Filter tabs + New Deal (or modal trigger)

### Phase 3 — Migrate Finance + Product modules — status: open

5. [ ] Migrate `/invoices` list page
   - Status tab filter buttons + New Invoice

6. [ ] Migrate `/products` list page
   - Search + type dropdown + New Product

### Phase 4 — Migrate Network + System modules — status: open

7. [ ] Migrate `/routers`, `/prefixes`, `/devices` pages
   - Routers: New Router
   - Prefixes: New Prefix
   - Devices: New Device (check template for actual button)

8. [ ] Migrate `/users`, `/cron`, `/attachments`
   - Users: New User
   - Cron: New Job
   - Attachments: search only (no primary action)

---

## Verification

- [ ] Every main list page has title+subtitle left, actions right, in one row at the same height
- [ ] `go build ./...` passes after each phase
- [ ] No regressions — all actions (search, filters, buttons) still work
- [ ] Pages with no actions (Dashboard, detail pages) unchanged — still use `PageHeader`

## Adjustments

<!-- document plan changes here -->

## Progress Log

<!-- entries added after each action -->
