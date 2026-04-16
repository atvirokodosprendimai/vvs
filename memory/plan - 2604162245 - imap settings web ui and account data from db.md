---
tldr: IMAP account settings page — list accounts with live status, edit/delete/pause/resume/sync controls
status: active
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

### Phase 1 — Backend commands — status: open

1. [ ] `app/commands/delete_account.go` — `DeleteAccountHandler`
   - calls `repo.Delete(ctx, id)`, publishes `isp.email.account_deleted`
   - inject: `EmailAccountRepository`, `EventPublisher`

2. [ ] `app/commands/manage_account.go` — `PauseAccountHandler` + `ResumeAccountHandler`
   - Pause: load → `account.Pause()` → `repo.Save()`, publish `isp.email.account_paused`
   - Resume: load → `account.Resume()` → `repo.Save()`, publish `isp.email.account_resumed`

### Phase 2 — HTTP routes and handlers — status: open

1. [ ] Add fields to `Handlers` struct: `deleteCmd`, `pauseCmd`, `resumeCmd`; update `NewHandlers`
   - inject: `DeleteAccountHandler`, `PauseAccountHandler`, `ResumeAccountHandler`

2. [ ] Implement handlers:
   - `settingsPage` — GET `/emails/settings` renders `EmailSettingsPage(accounts)`
   - `accountListSSE` — GET `/sse/email-accounts` subscribes `isp.email.*`, patches `#email-account-list`
   - `updateAccountSSE` — PUT `/api/email-accounts/{id}` reuses `ConfigureAccountCommand` with ID from URL
   - `deleteAccountSSE` — DELETE `/api/email-accounts/{id}` → `DeleteAccountHandler`
   - `pauseAccountSSE` — POST `/api/email-accounts/{id}/pause`
   - `resumeAccountSSE` — POST `/api/email-accounts/{id}/resume`
   - `triggerSyncSSE` — POST `/api/email-sync/{accountID}` publishes `isp.email.sync_requested`

3. [ ] Register new routes in `RegisterRoutes`:
   ```
   GET  /emails/settings
   GET  /sse/email-accounts
   PUT  /api/email-accounts/{id}
   DELETE /api/email-accounts/{id}
   POST /api/email-accounts/{id}/pause
   POST /api/email-accounts/{id}/resume
   POST /api/email-sync/{accountID}
   ```

### Phase 3 — Templates — status: open

1. [ ] `EmailSettingsPage(accounts []AccountReadModel)` in `templates.templ`
   - two-section layout: account list + "Add Account" CTA
   - `data-init="@get('/sse/email-accounts')"` on outer wrapper for live updates
   - signals: `{emailSettingsEdit:'', emailName:'', emailHost:'', emailPort:'993', emailUser:'', emailPass:'', emailTLS:'tls', emailFolder:'INBOX'}`

2. [ ] `EmailAccountList(accounts []AccountReadModel)` — SSE-patchable (`id="email-account-list"`)
   - per account card: name, `host:port`, TLS badge, folder, status badge (green/red/gray)
   - last sync time (`formatRelTime`) or "Never"
   - last error shown in red if status == "error"
   - action buttons per row:
     - Edit → sets `$emailSettingsEdit = account.ID`, prefills signals
     - Pause (if active/error) / Resume (if paused)
     - Sync Now button → `@post('/api/email-sync/{id}')`
     - Delete → confirm via JS `confirm()` → `@delete('/api/email-accounts/{id}')`

3. [ ] Edit account modal — shown when `$emailSettingsEdit != ''`
   - same fields as create form, prefilled from signals
   - Save: `@put('/api/email-accounts/$emailSettingsEdit')` → close on success
   - Cancel: `$emailSettingsEdit = ''`
   - Note: password field placeholder "leave blank to keep current"

4. [ ] Add nav link for Settings in `EmailPage` sidebar (or link from `/emails` to `/emails/settings`)

### Phase 4 — Wiring — status: open

1. [ ] `internal/app/app.go`:
   - `deleteEmailAccountCmd := emailcommands.NewDeleteAccountHandler(emailAccountRepo, publisher)`
   - `pauseEmailAccountCmd := emailcommands.NewPauseAccountHandler(emailAccountRepo, publisher)`
   - `resumeEmailAccountCmd := emailcommands.NewResumeAccountHandler(emailAccountRepo, publisher)`
   - pass to `emailhttp.NewHandlers(...)`

2. [ ] `templ generate && go build ./...` — clean build

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

