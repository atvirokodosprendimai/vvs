---
tldr: IMAP readonly email module — fetch, UTF-8 decode, attachment storage, thread organization, tag system
category: feature
---

# Email — IMAP Readonly

## Target

Operators reading and organizing inbound/outbound emails from configured IMAP accounts.
Emails are fetched readonly — no send, no delete on server.
Serves as CRM inbox: link emails to customers, track threads per customer, tag for follow-up.

## Behaviour

### Account configuration

- Operator configures one or more IMAP accounts (host, port, username, password, TLS mode)
- Credentials stored in DB (encrypted at rest via Go's `crypto/aes` with a key derived from config secret)
- Multiple accounts supported; each has a display name and inbox folder to watch (default: INBOX)
- Account status: `active` | `paused` | `error` (error stores last error message)

### Fetch and sync

- Background worker polls each active account on a configurable interval (default: 5 min)
- Worker connects via `github.com/emersion/go-imap/v2` — IMAP4rev1 + IMAP4rev2 compatible
- Readonly: only FETCH and SEARCH commands; no STORE, no EXPUNGE
- Worker tracks last UID per folder in DB; fetches only new messages (UID > last seen)
- Fetch: envelope headers + body structure + full body for messages under 10 MB; body structure only above threshold (large message preview)
- On sync, publishes `isp.email.synced` (batch count) via NATS

### Encoding and content decoding

- All text parts decoded to UTF-8 regardless of declared charset
- Use `golang.org/x/text/encoding/htmlindex` to resolve MIME charset names → decoder
- Fallback: if charset unknown or decoding fails, try windows-1252, then replace invalid bytes with U+FFFD
- Quoted-printable and base64 decoded automatically via `mime/quotedprintable` and `encoding/base64`
- HTML part: extracted as-is (sanitized on display). Plain text preferred for UI; HTML as fallback.
- `text/plain` and `text/html` both stored; UI shows plain, HTML available on request

### Attachments

- Inline and attached parts with `Content-Disposition: attachment` or non-text MIME types extracted
- Stored as blobs in SQLite (acceptable for ISP scale — typical attachments are PDFs, <5 MB)
- Stored with: filename, MIME type, size, message ID
- Max attachment size: 20 MB per file; larger parts recorded as metadata only (not downloaded)

### Thread organization

- Thread detection via standard email headers: `Message-ID`, `References`, `In-Reply-To`
- Algorithm:
  1. On message fetch, check if any existing `EmailThread` contains a message whose `Message-ID` appears in the new message's `References` or matches `In-Reply-To`
  2. If found: add to that thread
  3. If not: check if existing thread has same normalized subject (`Re:`, `Fwd:` stripped)
  4. If still not found: create new `EmailThread`
- Thread subject = subject of first message in thread
- Thread has `last_message_at` updated on new message; used for list ordering
- Thread `participant_addresses` — comma-joined list of unique From/To across all messages

### Tags

- Tags are free-form labels, scoped per account (or global — configurable)
- Applied to threads (not individual messages)
- Predefined system tags: `unread`, `starred`, `archived`
- `unread`: set automatically on new message in thread; cleared when operator opens thread
- `starred`, `archived`: operator-controlled
- Custom tags: operator creates, names, assigns color (hex string)
- Thread can have multiple tags
- Tag filter in UI: click tag → shows only threads with that tag

### Customer linking

- Thread can be linked to a CustomerID (optional)
- Link inferred automatically if From/To address matches a known customer contact email
- Operator can manually link/unlink
- Customer detail page shows linked email threads in a section (read-only list, click to open full thread)

---

## Domain

### Aggregates

**EmailAccount**
```
ID          string    (UUIDv7)
Name        string
Host        string
Port        int
Username    string
PasswordEnc []byte    (AES-encrypted)
TLS         string    (none|starttls|tls)
Folder      string    (default: INBOX)
Status      string    (active|paused|error)
LastError   string
LastSyncAt  time.Time
CreatedAt   time.Time
```

**EmailThread**
```
ID                  string
AccountID           string
Subject             string
ParticipantAddresses string
CustomerID          string    (nullable — linked customer)
LastMessageAt       time.Time
CreatedAt           time.Time
```

**EmailMessage**
```
ID          string    (UUIDv7)
AccountID   string
ThreadID    string
UID         uint32    (IMAP UID)
Folder      string
MessageID   string    (RFC 2822 Message-ID header)
References  string    (space-joined References header)
InReplyTo   string
Subject     string
FromAddr    string
FromName    string
ToAddrs     string    (comma-joined)
TextBody    string
HTMLBody    string
ReceivedAt  time.Time
FetchedAt   time.Time
```

**EmailAttachment**
```
ID          string
MessageID   string
Filename    string
MIMEType    string
Size        int64
Data        []byte    (nil if above threshold)
CreatedAt   time.Time
```

**EmailTag**
```
ID        string
AccountID string    (empty = global)
Name      string
Color     string
System    bool
```

**EmailThreadTag** (join)
```
ThreadID  string
TagID     string
```

---

## Module structure

```
internal/modules/email/
  domain/
    account.go          — EmailAccount aggregate
    message.go          — EmailMessage, EmailThread, EmailAttachment
    tag.go              — EmailTag, system tag consts
    repository.go       — port interfaces
  app/
    commands/
      configure_account.go
      sync_account.go     — trigger manual sync
      link_customer.go
      apply_tag.go
      remove_tag.go
      mark_read.go
    queries/
      list_threads.go
      get_thread.go       — thread + messages + attachments
      list_accounts.go
      read_model.go
  adapters/
    imap/
      fetcher.go          — IMAP connection + fetch loop
      decoder.go          — charset/encoding decode
      threader.go         — thread assignment logic
    persistence/
      gorm_repository.go
      models.go
    http/
      handlers.go
      templates.templ
  migrations/
    001_create_email_tables.sql
    embed.go
  worker/
    sync_worker.go        — background polling goroutine per account
```

---

## Routes

```
GET  /emails                          — thread list (all accounts, filterable)
GET  /emails/{threadID}               — thread detail (messages + attachments)
GET  /sse/emails                      — live thread list SSE
GET  /sse/emails/{threadID}           — live thread detail SSE

GET  /api/email-accounts              — list accounts
POST /api/email-accounts              — create account
PUT  /api/email-accounts/{id}         — update account (credentials, folder, status)
DELETE /api/email-accounts/{id}       — delete account + all messages

POST /api/email-threads/{id}/tags     — apply tag to thread
DELETE /api/email-threads/{id}/tags/{tagID}  — remove tag
POST /api/email-threads/{id}/link     — link to customer
POST /api/email-threads/{id}/read     — mark thread read (clears unread tag)
POST /api/email-sync/{accountID}      — trigger manual sync

GET  /api/email-attachments/{id}      — download attachment
```

---

## Interactions

- Depends on: `internal/shared/events` (NATS publisher)
- Depends on: `golang.org/x/text` (charset decoding)
- Depends on: `github.com/emersion/go-imap/v2` (IMAP client)
- Customer detail page: `internal/modules/customer/adapters/http` imports `emailhttp` read model for linked threads section
- No cross-module domain imports; CustomerID is a plain string

---

## Verification

1. Configure account → status shows `active`
2. Trigger sync → new messages appear in thread list (SSE live update)
3. Non-UTF8 email (ISO-8859-1, windows-1252) displays correctly
4. Reply chain grouped into single thread (References/In-Reply-To)
5. Same-subject grouping fallback works
6. PDF attachment downloads via `/api/email-attachments/{id}`
7. Apply custom tag → thread shows tag badge; tag filter shows only that thread
8. Link thread to customer → customer detail shows thread in email section
9. Mark read → unread badge clears; `isp.email.read` event published
10. `go test ./internal/modules/email/... -v -race` passes
