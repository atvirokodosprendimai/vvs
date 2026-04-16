---
tldr: Search email attachments by filename/extension — new /attachments page with live SSE search
status: active
---

# Plan: Attachment search by filename and extension

## Context

Email module already has `email_attachments` table with `filename`, `mime_type`, `size`.
No search capability exists — `EmailAttachmentRepository` only has `Save`, `FindByID`, `ListForMessage`.

Pattern to follow: URL-based navigation (`/attachments?account=X&q=Y`), SSE for live results.

## Phases

### Phase 1 - Domain + persistence — status: open

1. [ ] Add `SearchAttachments` to `EmailAttachmentRepository` port + GORM impl
   - query: join `email_attachments → email_messages → email_threads`
   - filter: `filename LIKE ?` (contains) scoped to `account_id`
   - result: filename, mime_type, size, thread_id, thread subject, from_addr, received_at
   - order: `received_at DESC`, limit 100
   - add `SearchAttachments(ctx, accountID, query string) ([]*AttachmentSearchRow, error)` to port

### Phase 2 - App query — status: open

1. [ ] Add `AttachmentSearchRow` read model to `internal/modules/email/app/queries/read_model.go`
   - fields: ID, Filename, MIMEType, Size, ThreadID, ThreadSubject, FromAddr, ReceivedAt
2. [ ] Add `SearchAttachmentsHandler` in `internal/modules/email/app/queries/search_attachments.go`
   - accepts `SearchAttachmentsQuery{AccountID, Query string}`
   - delegates to repo, maps to read model

### Phase 3 - HTTP handler + route — status: open

1. [ ] Add SSE handler `attachmentSearchSSE` in email `handlers.go`
   - reads `?account` and `?q` query params (URL-based, no signals needed)
   - calls `SearchAttachmentsHandler`
   - patches `#attachment-results` fragment
2. [ ] Register route `GET /sse/attachments` in email `routes.go`
3. [ ] Add page handler `attachmentsPage` — renders `/attachments?account=X&q=Y` full page
4. [ ] Register page route `GET /attachments` in router

### Phase 4 - UI template — status: open

1. [ ] Add `AttachmentsPage` templ in email `templates.templ`
   - account selector sidebar (same `<a href>` pattern as email page)
   - search input: `data-bind:q` + `data-on:input__debounce.300ms` triggers `@get('/sse/attachments?account=...', {openWhenHidden:false})`
   - results list fragment `#attachment-results`: filename, extension badge, size, thread subject, date
   - clicking a result opens thread in new tab: `<a href="/emails?account=X&folder=INBOX" target="_blank">`
   - empty/no-query state: "Enter a filename or extension to search"
2. [ ] Add "Attachments" nav item to layout sidebar

### Phase 5 - Wire — status: open

1. [ ] Add `searchAttachments` query handler field to `Handlers` struct and inject in `app.go`

## Verification

1. `go build ./...` — clean
2. Navigate to `/attachments?account=<id>` — page loads with account selected
3. Type `.pdf` in search — results show all PDF attachments with thread link
4. Type partial filename — matching attachments appear
5. Click result row — opens thread in new tab
6. Empty search — shows placeholder, no results

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

<!-- Timestamped entries tracking work done. Updated after every action. -->
