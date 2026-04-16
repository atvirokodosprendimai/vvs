---
tldr: Search email attachments by filename/extension — new /attachments page with live SSE search
status: completed
---

# Plan: Attachment search by filename and extension

## Context

Email module already has `email_attachments` table with `filename`, `mime_type`, `size`.
No search capability exists — `EmailAttachmentRepository` only has `Save`, `FindByID`, `ListForMessage`.

Pattern to follow: URL-based navigation (`/attachments?account=X&q=Y`), SSE for live results.

## Phases

### Phase 1 - Domain + persistence — status: completed

1. [x] Add `SearchByFilename` to `EmailAttachmentRepository` port + GORM impl
   - => `AttachmentSearchRow` domain type added to `domain/repository.go`
   - => SQL: join `email_attachments → email_messages → email_threads`, LIKE filter, scoped to `account_id`, ORDER BY `received_at DESC` LIMIT 100
   - => method name: `SearchByFilename(ctx, accountID, query string) ([]*AttachmentSearchRow, error)`

### Phase 2 - App query — status: completed

1. [x] Add `AttachmentSearchRow` read model to `internal/modules/email/app/queries/read_model.go`
2. [x] Add `SearchAttachmentsHandler` in `internal/modules/email/app/queries/search_attachments.go`
   - => empty query returns nil (no results), not an error

### Phase 3 - HTTP handler + route — status: completed

1. [x] Add SSE handler `attachmentSearchSSE` + page handler `attachmentsPage` in email `handlers.go`
   - => reads `?account` and `?q` URL params
   - => `WithSearchAttachments(q)` builder pattern (no constructor signature change)
2. [x] Routes registered in `RegisterRoutes`: `GET /attachments`, `GET /sse/attachments`

### Phase 4 - UI template — status: completed

1. [x] `AttachmentsPage` + `AttachmentResults` templ components in email `templates.templ`
   - => `data-bind:q` (kebab — won't lowercase) + debounce 300ms + `openWhenHidden:false`
   - => `mimeShort()` helper for readable MIME display
   - => thread link: `/emails?account=X&thread=Y` target=_blank
   - => `AttachmentResults` takes `accountID string` param (needed for thread link)
2. [x] "Attachments" nav item + paperclip SVG icon in `layout.templ`

### Phase 5 - Wire — status: completed

1. [x] `searchAttachmentsQuery` wired in `app.go` via `.WithSearchAttachments(searchAttachmentsQuery)`

## Verification

1. [x] `go build ./...` — clean
2. Navigate to `/attachments?account=<id>` — page loads with account selected
3. Type `.pdf` in search — results show all PDF attachments with thread link
4. Type partial filename — matching attachments appear
5. Click result row — opens thread in new tab
6. Empty search — shows placeholder, no results

## Adjustments

- 2604170151: Shipped all phases in single commit `068c458`

## Progress Log

- 2604170151: All phases complete — single session, all 5 phases done in one commit
