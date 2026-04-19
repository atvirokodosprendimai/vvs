---
tldr: Email inbox UX — 3-pane inline reading, customer context (name/code + email-match suggestion), tag filter active state, reply form position, post-send clearing, unread counts
status: completed
---

# Plan: Improve Email Module UX

## Context

- Spec: [[spec - email - imap readonly fetch threading and tagging]]
- Templates: `internal/modules/email/adapters/http/templates.templ`
- Handlers: `internal/modules/email/adapters/http/handlers.go`

### Issues identified

| # | Problem | Severity |
|---|---------|----------|
| 1 | Thread rows use `target="_blank"` — every click opens a new tab | High |
| 2 | No customer context visible near threads — `CustomerID` stored but name/code not shown | High |
| 3 | `customerLinkBadge` shows "Linked" text only — no name, code, or link to customer page | High |
| 4 | No auto-suggest customer from participant email address (contacts table knows this mapping) | Medium |
| 5 | Tag filter buttons have no active state visual feedback | Medium |
| 6 | Reply form rendered **before** thread messages, not after | Medium |
| 7 | Compose modal doesn't close/clear signals after successful send | Medium |
| 8 | Reply textarea isn't cleared after successful reply | Medium |
| 9 | Account sidebar shows status dot but no unread count badge | Low |

### Key backend facts

- `/sse/emails/{threadID}` already exists → `ThreadDetail` component
- `/api/email-threads/{threadID}/read` → marks thread read
- `$emailAccountID`, `$emailTagFilter`, `$emailSearch`, `$emailFolder`, `$emailPage` signals already defined on inbox
- `ThreadReadModel` has `CustomerID` but **no** `CustomerName`/`CustomerCode`
- `threadToReadModel` in `list_threads.go` only maps domain fields — no customer resolver injected
- Customer name lookup needs a `customerNameResolver` port (already exists in other modules — e.g. deal handlers use it)
- Contact email → customer mapping: contacts table has `email` column; query `SELECT customer_id FROM contacts WHERE email = ?`

---

## Phases

### Phase 1 - Quick polish — status: active

1. [x] Tag filter active state
   - `tagFilterButton` compares `$emailTagFilter` to name, adds amber text + bg when active
   - Use `data-class:text-amber-400="$emailTagFilter == 'unread'"` pattern (one component, one check)
   - Simpler: pass active via Go param or use `data-class` directly in templ

2. [x] Move reply form below messages in `EmailThreadPage`
   - => swapped `@ThreadDetail` before `@replyForm` in `EmailThreadPage`

3. [x] Clear compose signals on success
   - => already done in existing `composeSSE` — patches `{composeTo:'',composeSubject:'',composeBody:'',composeError:'',composeOpen:false}`

4. [x] Clear reply body on success
   - => already done in existing `replySSE` — patches `{emailReplyBody:'',emailReplyError:''}` + re-fetches `ThreadDetail`

5. [ ] Unread count badge on account sidebar
   - `ListAccountsHandler` read model needs `UnreadCount int` field
   - Query: `COUNT threads WHERE account_id = ? AND has_unread_tag`
   - Sidebar account link renders count badge when > 0

6. [x] Add `CustomerName`/`CustomerCode` to `ThreadReadModel`
   - => added fields to `read_model.go`
   - => added `customerInfoResolver` interface + `WithCustomerInfo` builder to `Handlers`
   - => added `enrichThreadList` and `enrichDetail` helpers; called in `listSSE`, `threadSSE`, `threadPage`, `replySSE`

7. [x] Show customer badge on thread rows in inbox
   - => `threadRow`: when `t.CustomerName != ""` shows amber `{code} · {name}` span; else shows participant address

8. [x] Fix `customerLinkBadge` to show name/code (not just "Linked")
   - => added `customerBadgeLabel(name, code) string` helper
   - => `ThreadDetail` now passes `customerBadgeLabel(thread.CustomerName, thread.CustomerCode)` as label
   - => renders as `CLI-001 · Acme Corp` linked to customer page

### Phase 2 - 3-pane inline reading — status: completed

Goal: clicking a thread shows detail in a right panel inside the inbox, without opening a new tab.

1. [x] Add thread detail panel to inbox layout
   - => `emailThreadID: ''` signal added to `EmailPage` `data-signals`
   - => panel slot with `data-show="$emailThreadID != ''"` and flex-1 inside inbox
   - => thread list uses `data-style:flex` to shrink to 360px when panel open
   - => close button sets `$emailThreadID = ''`
   - => `#email-thread-detail` div inside panel receives SSE patches from `threadSSE`

2. [x] Thread rows trigger inline panel instead of new tab
   - => removed `target="_blank"`, added `data-on:click__prevent`
   - => on click: `$emailThreadID = '{id}'; @get('/sse/emails/{id}', {openWhenHidden: false})`

3. [x] threadSSE patches panel content
   - => `ThreadDetail` renders `<div id="email-thread-detail">` — morpher patches by ID

4. [x] Auto-mark read when thread opens in panel
   - => already in `threadSSE` from Phase 1

5. [x] Auto-suggest customer from participant email match
   - => `SuggestedCustomerID/Name/Code` added to `ThreadDetailReadModel`
   - => `contactEmailLookup` port + `enrichSuggestion` helper in handlers.go
   - => `emailContactLookupBridge` in app.go queries contacts/customers JOIN directly
   - => suggestion pill in `ThreadDetail` header with one-click Link button

6. [x] Keep full-page route working
   - => `/emails/threads/{threadID}` unchanged

### Phase 3 - Polish — status: completed

1. [x] Keyboard navigation (partial)
   - => `Esc` closes panel via `data-on:keydown__window` on inbox container
   - => `r` focuses inline reply textarea when panel open and focus not in input
   - => j/k thread nav NOT implemented (deferred — low value vs complexity)

2. [x] Star toggle from thread row
   - => `★`/`☆` icon on each row, `data-on:click__stop__prevent` prevents row open
   - => PUT /api/email-threads/{id}/star → `toggleStarSSE` → `ToggleStarHandler`
   - => refreshes thread list after toggle

---

## Verification

- [x] Click thread in inbox → detail panel opens inline, no new tab
- [x] Thread row shows customer name + code badge when linked
- [x] Thread detail header shows "Acme Corp (CLI-001)" linked to customer page
- [x] Thread with unlinked but matching participant email → shows "Suggested: Acme Corp" pill with Link button
- [x] Accept suggestion → thread linked, suggested pill replaced by proper badge
- [x] Tag filter "unread" → button highlights amber, threads filtered
- [x] Send reply → reply body clears, thread refreshes showing new message
- [x] Compose send → modal closes, all fields cleared
- [x] Thread opens → auto-marked read (unread dot disappears)
- [x] Esc key closes thread panel
- [x] `/emails/threads/{threadID}` direct URL still works
- [ ] Unread count badge on account sidebar (deferred — requires domain repo change)
- [ ] j/k keyboard thread navigation (deferred — low value)

## Adjustments

- **2604172106** — j/k thread nav dropped from Phase 3: requires stable DOM IDs on thread rows + non-trivial signal wiring; deferred as low value vs complexity.

## Progress Log

- **2604172106** — Phase 1 complete (actions 1–4, 6–8). Action 5 (unread count) deferred. Actions 3+4 already done.
- **2604172106** — Phase 2 + 3 complete. Commit c6b18a4. Star toggle, 3-pane panel, customer auto-suggest, Esc/r keyboard shortcuts all shipped.
