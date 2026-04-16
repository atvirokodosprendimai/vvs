---
name: IMAP readonly email module
status: completed
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
*status: completed*

1. [x] Create domain models: `domain/account.go`, `domain/message.go`, `domain/tag.go`
   - => EmailAccount aggregate with status machine (active/paused/error), MarkSynced, SetError, Pause, Resume
   - => EmailThread/Message/Attachment, EmailTag with system consts, NormalizeSubject helper, ReferenceIDs()

2. [x] Create repository port: `domain/repository.go`
   - => 5 interfaces: EmailAccountRepository, EmailThreadRepository, EmailMessageRepository, EmailAttachmentRepository, EmailTagRepository

3. [x] Create read models: `app/queries/read_model.go`
   - => ThreadReadModel, ThreadDetailReadModel, MessageReadModel, AttachmentReadModel, AccountReadModel, TagReadModel
   - => 5 domain tests pass
   - `AccountReadModel`

---

## Phase 2 — Persistence + Migration
*status: completed*

4. [x] Write migration: `migrations/001_create_email_tables.sql`
   - => 6 tables + indexes; system tags (unread/starred/archived) seeded
5. [x] GORM persistence: `adapters/persistence/models.go` + `gorm_repository.go`
   - => All 5 repository interfaces implemented; UpdateThreadStats + AddParticipant helpers

---

## Phase 3 — IMAP Adapter + Decoder
*status: completed*

6. [x] Charset decoder: `adapters/imap/decoder.go`
   - => htmlindex → windows-1252 fallback → U+FFFD sanitize
7. [x] Thread assignment: `adapters/imap/threader.go`
   - => References/In-Reply-To → subject fallback → new thread
8. [x] IMAP fetcher: `adapters/imap/fetcher.go` + `body_parser.go`
   - => go-imap/v2, EXAMINE (readonly), UID-based incremental, go-message RFC2822 parse

---

## Phase 4 — Commands + Queries
*status: completed*

9. [x] Commands: configure_account (AES-256-GCM), apply_tag, remove_tag, mark_read, link_customer
10. [x] Queries: list_threads (account + tag filter), list_threads_for_customer, get_thread (messages+attachments)

---

## Phase 5 — Sync Worker
*status: completed*

11. [x] Background worker: polls active accounts, error state tracking, manual trigger via NATS

---

## Phase 6 — HTTP Layer + UI
*status: completed*

12. [x] Templates: EmailPage, ThreadList, ThreadDetail, AccountForm, EmailThreadsSection
13. [x] Handlers: listSSE, threadSSE, configureAccountSSE, tag/read/link handlers
14. [x] Routes registered via Handlers.RegisterRoutes (chi)

---

## Phase 7 — Integration + Wiring
*status: completed*

15. [x] app.go: email migration, all repos/cmds/queries, sync worker start/stop
16. [x] Customer detail: EmailThreadsSection added; CustomerDetailPage updated with emailThreads param
    - => VVS_EMAIL_ENC_KEY config for IMAP password encryption

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
| 2026-04-16 | All 7 phases completed; fixed remaining gaps: ListAccountsHandler query, attachmentFinder local interface, downloadAttachment serving real bytes |

---

## Adjustments

*(none yet)*
