---
tldr: Email inbox UX — 3-pane inline reading, customer context (name/code + email-match suggestion), tag filter active state, reply form position, post-send clearing, unread counts
status: active
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

### Phase 2 - 3-pane inline reading — status: open

Goal: clicking a thread shows detail in a right panel inside the inbox, without opening a new tab.

1. [ ] Add thread detail panel to inbox layout
   - In `EmailPage`: add `$emailThreadID: ''` signal to inbox `data-signals`
   - Add panel slot: `<div id="email-thread-panel" data-show="$emailThreadID != ''" style="display:none" class="...">`
   - Panel width: `w-[520px] flex-shrink-0` or `flex-1` — pick visually
   - Show close button `×` that sets `$emailThreadID = ''`
   - Add `id="email-thread-panel-content"` inner div that receives SSE patches

2. [ ] Thread rows trigger inline panel instead of new tab
   - Remove `target="_blank"` from `threadRow`
   - Change `<a href=...>` to `<button>` or `<a>` with `data-on:click`
   - On click: `$emailThreadID = '{id}'; @get('/sse/emails/{id}', {openWhenHidden: false})`
   - Use `data-on:click={ fmt.Sprintf(...) }` pattern

3. [ ] `threadSSE` patches panel content instead of full page
   - Currently `threadSSE` patches `#email-thread-detail` (which is inside `ThreadDetail` component)
   - `ThreadDetail` root `id` is `email-thread-detail` — this already works for panel patching
   - Wrap panel content in `<div id="email-thread-panel-content">` and patch into that

4. [ ] Auto-mark read when thread opens in panel
   - In `threadSSE`: after patching thread detail, also fire `markReadCmd.Handle(ctx, threadID)`
   - Saves the separate manual "Mark read" click

5. [ ] Auto-suggest customer from participant email match
   - Add `SuggestedCustomerID`, `SuggestedCustomerName`, `SuggestedCustomerCode` to `ThreadDetailReadModel`
   - New port: `type contactEmailLookup interface { FindCustomerByContactEmail(ctx, email string) (id, name, code string, err error) }`
   - In `GetThreadHandler` (or in `threadSSE` handler): when `CustomerID == ""`, iterate participant addresses, call lookup for each, first match → suggestion
   - Template: show "Suggested: [CustomerName]" pill (neutral/yellow tone) below `customerLinkBadge`, with "Link" button calling existing `/api/email-threads/{id}/link`
   - One-click accept: on click → `@post('/api/email-threads/{id}/link')` with `customerID` signal pre-set to suggestion ID

6. [ ] Keep full-page route working
   - `/emails/threads/{threadID}` stays as-is for direct links / bookmarks
   - No changes needed to `threadPage` handler

### Phase 3 - Polish — status: open

1. [ ] Keyboard navigation
   - `data-on:keydown__window`: `j` → next thread, `k` → prev thread, `Esc` → close panel, `r` → focus reply
   - Thread rows need stable IDs or data attributes for j/k traversal
   - Consider `data-on:keydown__window` on inbox container

2. [ ] Star toggle from thread row
   - Thread row: show star icon (⭐ / ☆) that calls `@post('/api/email-threads/{id}/tags')` with tag=starred
   - Or `@delete(...)` to unstar
   - Update row reactively after star toggle

---

## Verification

- [ ] Click thread in inbox → detail panel opens inline, no new tab
- [ ] Thread row shows customer name + code badge when linked
- [ ] Thread detail header shows "Acme Corp (CLI-001)" linked to customer page
- [ ] Thread with unlinked but matching participant email → shows "Suggested: Acme Corp" pill with Link button
- [ ] Accept suggestion → thread linked, suggested pill replaced by proper badge
- [ ] Tag filter "unread" → button highlights amber, threads filtered
- [ ] Send reply → reply body clears, thread refreshes showing new message
- [ ] Compose send → modal closes, all fields cleared
- [ ] Thread opens → auto-marked read (unread dot disappears)
- [ ] Esc key closes thread panel
- [ ] `/emails/threads/{threadID}` direct URL still works

## Adjustments

<!-- document plan changes with timestamps -->

## Progress Log

- **2604172106** — Phase 1 complete (actions 1–4, 6–8). Action 5 (unread count) deferred — requires domain repo interface change. Actions 3+4 were already done in existing code.
