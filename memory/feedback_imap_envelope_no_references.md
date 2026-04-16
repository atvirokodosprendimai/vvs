---
name: IMAP ENVELOPE does not include References header
description: go-imap Envelope struct has InReplyTo but not References; must parse from raw body bytes
type: feedback
---

The IMAP `ENVELOPE` response (and `go-imap/v2`'s `imaplib.Envelope` struct) includes `InReplyTo` but **not** `References`. References is only in the RFC 2822 message headers.

**Why:** processMessage was setting `msg.InReplyTo` from `env.InReplyTo` but leaving `msg.References` empty. Thread assignment calls `msg.ReferenceIDs()` which reads `msg.References` — so reply chains fell back to subject-based grouping.

**Fix:** Parse `References` from the raw body bytes via `mail.CreateReader`:
```go
r, _ := mail.CreateReader(bytes.NewReader(raw))
refs := r.Header.Get("References")
```

**How to apply:** Whenever reading IMAP message headers beyond Date/Subject/From/To/CC/BCC/MessageID/InReplyTo, fetch them from the raw RFC 2822 body, not the Envelope.
