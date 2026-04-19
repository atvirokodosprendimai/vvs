---
tldr: New PageHeaderRow component — title left, filters+buttons right at same height — migrated across all main list pages
status: completed
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

### Phase 1 — Add `PageHeaderRow` component — status: completed

1. [x] Add `PageHeaderRow(title, subtitle string)` to `components.templ`
   - => `flex items-center justify-between mb-6` row
   - => right side: `flex items-center gap-3` with `{ children... }` slot

### Phase 2 — Migrate CRM modules — status: completed

2. [x] Migrate `/customers` list page
   - => search input + New Customer link in PageHeaderRow; signals on data-init div

3. [x] Migrate `/tickets` and `/tasks` standalone pages
   - => tickets: search + New Ticket button in header
   - => tasks: New Task button only (no search filter)

4. [x] Migrate `/deals` standalone page
   - => stage tabs + search moved to PageHeaderRow in DealsPage
   - => signals on outer wrapper; DealsPageContent is now table-only

### Phase 3 — Migrate Finance + Product modules — status: completed

5. [x] Migrate `/invoices` list page
   - => 5 status tabs + New Invoice link in header; consolidated data-class to object form

6. [x] Migrate `/products` list page
   - => search + type dropdown + New Product link in header

### Phase 4 — Migrate Network + System modules — status: completed

7. [x] Migrate `/routers`, `/prefixes`, `/devices` pages
   - => routers: New Router link
   - => prefixes: Add Prefix button; signals on outer wrapper
   - => devices: status tab links + Register Device button; signals on outer wrapper

8. [x] Migrate `/users`, `/cron`
   - => users: Add User button; signals moved before PageHeaderRow
   - => cron: Add Job button; signals on outer wrapper
   - => attachments: sidebar layout (like email inbox) — kept PageHeader, skip

---

## Verification

- [ ] Every main list page has title+subtitle left, actions right, in one row at the same height
- [ ] `go build ./...` passes after each phase
- [ ] No regressions — all actions (search, filters, buttons) still work
- [ ] Pages with no actions (Dashboard, detail pages) unchanged — still use `PageHeader`

## Adjustments

<!-- document plan changes here -->

## Adjustments

- **2604180043** — Attachments page kept with `PageHeader` — it has a 2-column sidebar+content layout where the search lives in the content column, not the page top. Forcing it into PageHeaderRow would be visually wrong.

## Progress Log

- **2604180043** — All 4 phases complete. 11 pages migrated. Commits: 9c26242 (component), 4d4bc3e (CRM), 5ab2b76 (Finance/Product), 840d2bc (Network/System).
