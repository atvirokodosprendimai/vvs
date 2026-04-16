---
name: IMAP readonly email module
status: open
spec: "[[spec - email - imap readonly fetch threading and tagging]]"
---

# Plan: IMAP Readonly Email Module

## Context

ISP operators need an inbox inside the system — fetch emails from configured IMAP accounts, decode to UTF-8, organize by thread, tag for follow-up, and link threads to customers.
No send, no server-side mutation — strictly readonly IMAP.
See [[spec - email - imap readonly fetch threading and tagging]] for full design.

Architecture constraints:
- Single Go binary, hexagonal arch, SQLite+NATS, Datastar SSE, templ
- Module lives at `internal/modules/email/`
- Domain imports nothing outside `internal/shared/`
- Background sync worker polls per account

---

## Phase 1 — Domain + Contracts
*status: open*

1. [ ] Create domain models: `domain/account.go`, `domain/message.go`, `domain/tag.go`
   - EmailAccount aggregate (ID, credentials enc, status, folder, sync state)
   - EmailThread, EmailMessage, EmailAttachment, EmailTag, EmailThreadTag
   - Status consts: account (`active|paused|error`), system tag names (`unread|starred|archived`)

2. [ ] Create repository port: `domain/repository.go`
   - `EmailAccountRepository` — CRUD + ListActive
   - `EmailMessageRepository` — Save, FindByUID, FindByMessageID
   - `EmailThreadRepository` — Save, FindByMessageID, FindBySubject, ListForAccount, ListForCustomer
   - `EmailAttachmentRepository` — Save, Get
   - `EmailTagRepository` — CRUD + ListForThread

3. [ ] Create read models: `app/queries/read_model.go`
   - `ThreadReadModel` (thread + last message preview + tags + unread count)
   - `MessageReadModel` (full message + attachments)
   - `AccountReadModel`

---

## Phase 2 — Persistence + Migration
*status: open*

4. [ ] Write migration: `migrations/001_create_email_tables.sql`
   - Tables: `email_accounts`, `email_threads`, `email_messages`, `email_attachments`, `email_tags`, `email_thread_tags`
   - Indexes: `(account_id, uid)` on messages, `(customer_id)` on threads, `(thread_id)` on messages
   - `embed.go` with `//go:embed *.sql`

5. [ ] GORM persistence: `adapters/persistence/models.go` + `gorm_repository.go`
   - Map all domain structs to GORM models
   - Implement all repository interfaces
   - Thread finder: query by Message-ID in References, fallback subject match

---

## Phase 3 — IMAP Adapter + Decoder
*status: open*

6. [ ] Charset decoder: `adapters/imap/decoder.go`
   - `DecodeToUTF8(charset, body string) string`
   - Use `golang.org/x/text/encoding/htmlindex` to resolve charset name → decoder
   - Fallback chain: windows-1252 → replace invalid bytes with U+FFFD
   - Handle quoted-printable and base64 via stdlib

7. [ ] Thread assignment: `adapters/imap/threader.go`
   - `Assign(msg EmailMessage, repo ThreadRepository) (threadID string, err error)`
   - Check References/In-Reply-To against existing Message-IDs
   - Fallback: normalized subject match (strip Re:/Fwd: prefixes)
   - Create new thread if no match

8. [ ] IMAP fetcher: `adapters/imap/fetcher.go`
   - `Fetch(ctx, account EmailAccount, repo ...) error`
   - Connect via `go-imap/v2`, SELECT folder readonly
   - SEARCH UID > lastSeen, fetch in batches of 50
   - Parse envelope + bodystructure + text/html parts
   - Call decoder, call threader, persist message + attachments
   - Update account.LastSyncAt + last seen UID
   - Publish `isp.email.synced` via NATS

---

## Phase 4 — Commands + Queries
*status: open*

9. [ ] Commands:
   - `commands/configure_account.go` — create/update account (encrypt password with AES-256-GCM)
   - `commands/apply_tag.go` — add tag to thread
   - `commands/remove_tag.go` — remove tag from thread
   - `commands/mark_read.go` — clear `unread` tag, publish `isp.email.read`
   - `commands/link_customer.go` — set thread.CustomerID

10. [ ] Queries:
    - `queries/list_threads.go` — ListThreadsForAccount(accountID, tagFilter) → []ThreadReadModel
    - `queries/get_thread.go` — GetThread(threadID) → thread + messages + attachments

---

## Phase 5 — Sync Worker
*status: open*

11. [ ] Background worker: `worker/sync_worker.go`
    - One goroutine per active account
    - Polls on configurable interval (default 5 min)
    - Restarts on transient IMAP errors (backoff); sets account.Status=error on persistent failure
    - Manual trigger via `isp.email.sync_requested` NATS subject
    - Started by `app.go` after account repository is ready

---

## Phase 6 — HTTP Layer + UI
*status: open*

12. [ ] Templates: `adapters/http/templates.templ`
    - `EmailPage` — full inbox page: account selector, tag sidebar, thread list
    - `ThreadList` — patchable by `#email-thread-list`
    - `ThreadDetail` — patchable by `#email-thread-{id}`, shows messages + attachments
    - `AccountForm` — modal for add/edit account
    - `TagBadge`, `TagSidebar`

13. [ ] Handlers: `adapters/http/handlers.go`
    - `listSSE` — subscribe `isp.email.*`, patch ThreadList on change
    - `threadSSE` — subscribe `isp.email.*`, patch ThreadDetail on change
    - `configureAccountSSE` — create/update account
    - `applyTagSSE`, `removeTagSSE`
    - `markReadSSE` — mark read on thread open
    - `linkCustomerSSE`
    - `syncSSE` — trigger manual sync
    - `downloadAttachment` — stream attachment bytes

14. [ ] Routes: register in `internal/infrastructure/http/router.go`

---

## Phase 7 — Integration + Wiring
*status: open*

15. [ ] Wire in `internal/app/app.go`:
    - Run migration
    - Create repositories, command/query handlers
    - Start sync worker
    - Register HTTP handlers

16. [ ] Customer detail: add linked email threads section
    - `internal/modules/customer/adapters/http/templates.templ` — `EmailThreadsSection`
    - `internal/modules/customer/adapters/http/handlers.go` — inject `ListEmailThreadsForCustomer` query

---

## Verification

- [ ] `go test ./internal/modules/email/... -v -race` passes
- [ ] `templ generate && go build ./...` clean
- [ ] Configure IMAP account → status active
- [ ] Trigger manual sync → threads appear (SSE live)
- [ ] Non-UTF8 email (ISO-8859-1) displays correctly
- [ ] Reply chain → single thread in list
- [ ] PDF attachment downloads
- [ ] Custom tag apply → badge appears live
- [ ] Mark read → unread badge clears
- [ ] Customer detail → linked threads visible

---

## Progress Log

| Date | Note |
|------|------|
| 2026-04-16 | Plan created from user request; spec created at [[spec - email - imap readonly fetch threading and tagging]] |

---

## Adjustments

*(none yet)*
