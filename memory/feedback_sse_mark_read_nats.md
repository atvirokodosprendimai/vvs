---
name: After MarkRead, publish a NATS event so live SSE thread lists refresh
description: MarkRead alone doesn't trigger threadsSSE to re-query — need a publish
type: feedback
---

`threadsSSE` only re-queries the DB when a NATS event arrives on `isp.chat.>`. Calling `store.MarkRead` updates the DB but publishes nothing, so the unread badge in the sidebar never clears while the SSE is open.

**Why:** NATS is the only signal `threadsSSE` has that something changed. Silent DB writes are invisible to it.

**How to apply:** After any state change that should update the thread list in real time, publish a NATS event so `threadsSSE` picks it up:

```go
if err := h.store.MarkRead(ctx, threadID, userID); err == nil {
    h.publisher.Publish(ctx, "isp.chat.read."+threadID, events.DomainEvent{
        Type: "chat.read", AggregateID: threadID, ...
    })
}
```

General rule: any mutation visible in a live SSE view must be followed by a NATS publish on a subject that view subscribes to.
