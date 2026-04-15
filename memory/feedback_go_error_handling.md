---
name: Handle errors gracefully in Go — don't silently discard with _
description: Errors discarded with _ hide bugs; log or handle every error path
type: feedback
---

Don't silence errors with `_` unless the error is genuinely impossible or irrelevant. Silent discards hide bugs that are hard to diagnose later.

**Why:** Errors like `store.ListThreadsForUser` returning empty due to a scan failure were invisible because the caller did `threads, _ := ...`. Took multiple debug rounds to find.

**How to apply:**

In HTTP handlers — return an error response:
```go
threads, err := h.store.ListThreadsForUser(r.Context(), user.ID)
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
```

In SSE loops where you can't return an HTTP error — log and continue:
```go
next, err := h.store.ListThreadsForUser(r.Context(), user.ID)
if err != nil {
    log.Printf("threadsSSE: list threads: %v", err)
    continue
}
```

For fire-and-forget side effects (EnsurePublicMembership, MarkRead) — still log:
```go
if err := h.store.EnsurePublicMembership(r.Context(), user.ID); err != nil {
    log.Printf("EnsurePublicMembership: %v", err)
}
```

Only acceptable `_` uses:
- `defer cancel()` style cleanup where error is structurally impossible
- Explicit comment explaining why the error is safe to ignore
