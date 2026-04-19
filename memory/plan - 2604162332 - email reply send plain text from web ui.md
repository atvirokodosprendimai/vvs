---
tldr: Add SMTP send capability so users can reply to email threads from the web UI with plain text
status: completed
---

# Plan: Email Reply — Send Plain Text from Web UI

## Context

- Existing email module: `internal/modules/email/` — IMAP receive only, no send capability
- Thread detail page: `GET /emails/threads/{threadID}` (added in prior session)
- `EmailAccount` domain model has IMAP config but no SMTP fields
- Encryption key (`VVS_EMAIL_ENC_KEY`) already used for IMAP passwords — reuse for SMTP
- No SMTP library in go.mod — will use `net/smtp` (stdlib) + `crypto/tls`
- Unique constraint on `email_messages(account_id, uid)` must be handled for sent messages

## Phases

### Phase 1 — SMTP config in account — status: open

1. [ ] Add SMTP fields to `EmailAccount` domain and `AccountReadModel`
   - fields: `SMTPHost string` (empty → use IMAP Host), `SMTPPort int` (default 587), `SMTPTLS string` (default "starttls")
   - add to `domain/account.go`, `persistence/models.go`, `queries/read_model.go`
   - add to `toAccountModel` / `toDomain` mapping functions

2. [ ] Add migration `002_add_smtp_fields.sql`
   - `ALTER TABLE email_accounts ADD COLUMN smtp_host TEXT NOT NULL DEFAULT ''`
   - `ALTER TABLE email_accounts ADD COLUMN smtp_port INTEGER NOT NULL DEFAULT 587`
   - `ALTER TABLE email_accounts ADD COLUMN smtp_tls TEXT NOT NULL DEFAULT 'starttls'`

3. [ ] Add migration `003_add_message_direction.sql` — handle sent message uniqueness
   - `ALTER TABLE email_messages ADD COLUMN direction TEXT NOT NULL DEFAULT 'in'`
   - `DROP INDEX IF EXISTS idx_messages_uid`
   - `CREATE UNIQUE INDEX idx_messages_uid ON email_messages(account_id, uid) WHERE direction = 'in'`
   - Update `messageModel` and domain `EmailMessage` with `Direction string`

4. [ ] Update `configure_account.go` command to accept + persist SMTP fields
   - `ConfigureAccountCommand` gets `SMTPHost`, `SMTPPort`, `SMTPTLS` fields
   - `ConfigureAccountHandler` sets them on the account aggregate

5. [ ] Update settings page SMTP form fields
   - Add smtp_host, smtp_port, smtp_tls inputs to both add and edit modals in `templates.templ`
   - Add SMTP section header in the form to visually separate IMAP / SMTP
   - Update signal init to include `emailSMTPHost`, `emailSMTPPort`, `emailSMTPTLS`
   - Update `emailAccountCard` to show SMTP info line

### Phase 2 — SMTP sender adapter — status: open

1. [ ] Create `adapters/smtp/sender.go`
   - Port interface in domain: `domain/sender.go` — `type EmailSender interface { Send(ctx, account, to, subject, body, inReplyTo, references string) error }`
   - Struct `Sender` with `encKey []byte`
   - `NewSender(encKey []byte) *Sender`
   - Handle three TLS modes using stdlib `net/smtp` + `crypto/tls`:
     - `tls` (implicit, port 465): `tls.Dial` → `smtp.NewClient`
     - `starttls` (port 587): `smtp.Dial` → `client.StartTLS`
     - `none`: plain `smtp.Dial`
   - Auth: `smtp.PlainAuth` using decrypted password
   - Build MIME message with `From`, `To`, `Subject`, `Message-ID`, `In-Reply-To`, `References`, `Date`, `Content-Type: text/plain; charset=utf-8`
   - SMTPHost fallback: if account.SMTPHost == "", use account.Host

### Phase 3 — Send reply command + HTTP handler — status: open

1. [ ] Create `app/commands/send_reply.go`
   - `SendReplyCommand { ThreadID, Body string }`
   - `SendReplyHandler` deps: thread repo, message repo, account repo, tag repo, sender `domain.EmailSender`, publisher
   - Logic:
     1. Load thread → get last received message (highest ReceivedAt, direction='in')
     2. Load account by thread.AccountID
     3. Build To = last message FromAddr; Subject = "Re: " + NormalizeSubject(thread.Subject)
     4. InReplyTo = last message MessageID; References = last message References + " " + last message MessageID
     5. Call `sender.Send(ctx, account, to, subject, body, inReplyTo, references)`
     6. Save sent message: `EmailMessage{Direction: "out", FromAddr: account.Username, ToAddrs: to, TextBody: body, ReceivedAt: now}`
     7. Update thread stats + participant addresses
     8. Publish `isp.email.thread_updated`

2. [ ] Register route `POST /api/email-threads/{threadID}/reply` in handlers.go + routes
   - Read `emailReplyBody` signal
   - On success: patch signal `emailReplyBody = ''` and patch thread detail via SSE refresh

3. [ ] Wire `SendReplyHandler` + `smtp.NewSender` in `app.go`

### Phase 4 — Reply UI on thread page — status: open

1. [ ] Add reply form to `ThreadDetail` template (below messages)
   - Signal `emailReplyBody` (textarea, data-bind)
   - "Send Reply" button: `data-on:click="@post('/api/email-threads/{id}/reply')"`
   - Loading indicator while sending
   - After send: clear textarea (signal patch), show brief "Sent ✓" confirmation
   - Thread list SSE will auto-refresh to show new message (via published event)

## Verification

1. Navigate to a received email thread
2. Type a reply in the textarea, click Send
3. Email is delivered to the original sender
4. Sent message appears immediately in the thread view (direction='out')
5. Account settings page shows SMTP fields (host/port/tls)
6. Works with TLS (port 465) and STARTTLS (port 587) modes

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

<!-- Timestamped entries tracking work done. Updated after every action. -->
