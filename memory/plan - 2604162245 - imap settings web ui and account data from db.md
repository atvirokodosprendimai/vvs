---
tldr: IMAP account settings page — list accounts with live status, edit/delete/pause/resume/sync controls
status: completed
---

# Plan: IMAP Settings Web UI

## Context

- Spec: [[spec - email - imap readonly fetch threading and tagging]]

**Current state:**
- `ConfigureAccountHandler` handles create + update (by ID)
- `ListAccountsHandler` returns full `AccountReadModel` (host, port, status, lastSync, lastError)
- `EmailPage` sidebar lists accounts with status dot + "Add" modal (create-only)
- `POST /api/email-accounts` route exists; no edit/delete/pause/resume routes
- Worker listens to `isp.email.sync_requested` for manual sync trigger

**What's needed:**
1. Commands: delete, pause, resume
2. Routes: settings page, SSE account list, PUT update, DELETE, pause/resume, manual sync
3. Templates: settings page with account cards, live status stream, edit modal

---

## Phases

### Phase 1 — Backend commands — status: completed

1. [x] `app/commands/delete_account.go` — `DeleteAccountHandler`
   - => calls `repo.Delete(ctx, id)`, publishes `isp.email.account_deleted`

2. [x] `app/commands/manage_account.go` — `PauseAccountHandler` + `ResumeAccountHandler`
   - => Pause/Resume load account, call domain method, save, publish event

### Phase 2 — HTTP routes and handlers — status: completed

1. [x] Add fields to `Handlers` struct: `deleteCmd`, `pauseCmd`, `resumeCmd`; update `NewHandlers`

2. [x] Implement handlers:
   - => `settingsPage`, `accountListSSE`, `updateAccountSSE`, `deleteAccountSSE`
   - => `pauseAccountSSE`, `resumeAccountSSE`, `triggerSyncSSE`

3. [x] Register new routes in `RegisterRoutes`
   - => 7 new routes registered

### Phase 3 — Templates — status: completed

1. [x] `EmailSettingsPage` — full settings page with `data-init="@get('/sse/email-accounts')"` for live updates
2. [x] `EmailAccountList` — SSE-patchable (`id="email-account-list"`)
3. [x] `emailAccountCard` — per-account card with name, host:port, TLS, folder, status badge, last sync, error
4. [x] Edit account modal shown when `$emailSettingsEdit != ''`; password "leave blank to keep"
5. [x] `EmailPage` sidebar "Settings" link replaced "Add" button → `/emails/settings`

### Phase 4 — Wiring — status: completed

1. [x] `internal/app/app.go`: `deleteAccountCmd`, `pauseAccountCmd`, `resumeAccountCmd` wired
2. [x] `templ generate && go build ./...` — clean build

---

## Verification

1. `GET /emails/settings` renders account list with host, port, TLS, folder, status, last sync
2. SSE: trigger manual sync → status badge updates live (no page reload)
3. Add account → appears immediately in list via SSE patch
4. Edit existing account (change name/folder) → updates in list
5. Pause account → status changes to "paused" badge
6. Resume account → status returns to "active"
7. Delete account → row removed from list
8. Sync Now button → status updates, last sync time changes
9. Error account: red badge + error message displayed
10. `go build ./...` clean

---

## Progress Log

- 2604162245 — Plan created
- 2604162300 — All 4 phases completed: commands, HTTP handlers, templates, wiring; clean build

