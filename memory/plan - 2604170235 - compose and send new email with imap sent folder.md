---
tldr: Compose and send new email — SMTP send + save locally + IMAP APPEND to Sent folder
status: active
---

# Plan: Compose and send new email with IMAP Sent folder

## Context

- `domain.EmailSender.Send()` exists and works (SMTP)
- `SendReplyHandler` is the template: SMTP send → save local thread/message → publish event
- `dial(account)` in `imap/fetcher.go` handles IMAP connection reusable for APPEND
- `smtp/sender.go` builds raw RFC 2822 bytes internally — needs to expose them for IMAP APPEND
- No compose UI or command exists yet

## Phases

### Phase 1 - Domain: SentFolder field + migration — status: open

1. [ ] Add `SentFolder string` to `domain.EmailAccount` (default `"Sent"`)
   - fallback logic: if `SentFolder == ""` use `"Sent"`
2. [ ] Migration `007_add_sent_folder.sql` — `ALTER TABLE email_accounts ADD COLUMN sent_folder TEXT NOT NULL DEFAULT 'Sent'`
3. [ ] Add `sent_folder` to `accountModel` in `adapters/persistence/models.go` and mapping functions

### Phase 2 - SMTP sender exposes raw message bytes — status: open

1. [ ] Extract `BuildMessage(account, to, subject, body, inReplyTo, references string) (msgID string, raw []byte)` from `smtp/sender.go`
   - `Send()` calls `BuildMessage()` then sends — no behaviour change
   - Raw bytes needed by IMAP append step

### Phase 3 - IMAP append adapter — status: open

1. [ ] Add `AppendToFolder(ctx context.Context, account *domain.EmailAccount, folder string, raw []byte) error` in `adapters/imap/fetcher.go`
   - Opens IMAP connection via `dial()` + login
   - Calls `c.Append(folder, raw, &imapclient.AppendOptions{Flags: []imap.Flag{imap.FlagSeen}}).Wait()`
   - Returns error; caller treats as best-effort (log + continue)
2. [ ] Add `EmailFolderAppender` domain port interface in `domain/sender.go`:
   ```go
   type EmailFolderAppender interface {
       AppendToFolder(ctx context.Context, account *EmailAccount, folder string, raw []byte) error
   }
   ```

### Phase 4 - ComposeEmailCommand + handler — status: open

1. [ ] Write failing test in `app/commands/compose_test.go`
   - stub sender, stub appender, stub repos
   - assert: thread created, message saved with direction="out", sender called once
2. [ ] Implement `ComposeEmailCommand{AccountID, To, Subject, Body string}` + handler in `app/commands/compose.go`
   - get account by ID
   - `BuildMessage()` → raw bytes + msgID
   - SMTP send via `sender.Send()`
   - create new `EmailThread{Subject, AccountID, MessageCount:1}`; `threads.Save()`
   - save `EmailMessage{direction:"out", ...}` via `messages.Save()`
   - IMAP append via `appender.AppendToFolder(ctx, account, account.SentFolder, raw)` — log error, non-fatal
   - publish `isp.email.thread_created`
3. [ ] Run tests — all pass

### Phase 5 - HTTP handler + UI — status: open

1. [ ] Write failing test in `adapters/http/handlers_test.go`
   - stub repos + stub sender
   - POST `/api/email-compose` with signals `{emailComposeTo, emailComposeSubject, emailComposeBody}`
   - assert: SSE response contains `#email-inbox` patch or clears compose signals
2. [ ] Add `composeSSE` handler in `adapters/http/handlers.go`
   - reads signals: `emailComposeTo`, `emailComposeSubject`, `emailComposeBody`, `emailAccountID`
   - calls `composeCmd.Handle()`
   - on error: `PatchSignals({"emailComposeError": smtpErrMsg(err)})`
   - on success: clear signals + refresh thread list via `PatchSignals({"emailComposeTo":"","emailComposeSubject":"","emailComposeBody":"","emailComposeError":"","emailComposeOpen":false})`
3. [ ] Register route `POST /api/email-compose`
4. [ ] Add compose modal templ in `templates.templ`:
   - Signals: `emailComposeOpen:false, emailComposeTo:'', emailComposeSubject:'', emailComposeBody:'', emailComposeError:''`
   - "New Email" button: `data-on:click="$emailComposeOpen=true"` (in email page header)
   - Modal (`data-show="$emailComposeOpen"`): To, Subject, textarea Body
   - Send button: `data-on:click="@post('/api/email-compose')"`
   - Error div: `data-show="$emailComposeError != ''"` + `data-text="$emailComposeError"`
   - Signal names use kebab-case in data-bind (e.g. `data-bind:email-compose-to`)

### Phase 6 - Wire in app.go — status: open

1. [ ] Inject `AppendToFolder` adapter and `ComposeEmailCommand` handler in `app.go`
   - `WithComposeCmd(composeCmd)` builder on Handlers
   - pass `smtpSender` (already exists) + new IMAP appender

## Verification

1. `go test ./internal/modules/email/... -race` — all pass
2. `go build ./...` — clean
3. Open `/emails?account=<id>` → "New Email" button visible
4. Fill To/Subject/Body → Send → modal closes, new thread appears in list
5. Check IMAP client (Thunderbird/webmail) → message visible in Sent folder
6. Check local DB: `email_messages` row with `direction='out'`, correct `thread_id`

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

<!-- Timestamped entries tracking work done. Updated after every action. -->
