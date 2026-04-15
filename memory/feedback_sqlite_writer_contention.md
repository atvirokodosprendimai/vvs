---
name: SQLite single writer — don't call WriteTX inside long-lived SSE goroutines on every event
description: WriteTX in SSE event loops starves the single SQLite writer and blocks HTTP handlers
type: feedback
---

The app uses one SQLite write connection (`SetMaxOpenConns(1)`). If an SSE goroutine calls `WriteTX` on every NATS event (e.g. `EnsurePublicMembership` on every chat message), concurrent SSE connections queue up on the writer and `chatSend` (HTTP handler) blocks waiting — request hangs in browser as "pending".

**Why:** HTTP/1.1 connection held open; GORM's `db.W.Transaction(...)` blocks until the single writer connection is free. With multiple SSE goroutines all writing per-event, `chatSend` can't get the writer.

**How to apply:** In SSE event loops, only call `WriteTX` when the event type actually requires a write. Filter by `event.Type`:

```go
case event, ok := <-ch:
    if event.Type == "chat.thread.created" {
        _ = h.store.EnsurePublicMembership(r.Context(), user.ID) // WriteTX
    }
    // read-only re-query always fine
    next, _ := h.store.ListThreadsForUser(r.Context(), user.ID)
```

Also avoid `MarkRead` (WriteTX) inside the per-message receive loop — call it once on SSE connect instead.
