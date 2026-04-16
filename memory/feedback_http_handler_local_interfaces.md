---
name: HTTP handler packages use local interfaces for DI
description: HTTP adapter packages define minimal local interfaces instead of importing concrete persistence types
type: feedback
---

HTTP handler packages should define a local minimal interface for any repository dependency rather than importing the concrete type from `adapters/persistence`.

**Why:** Keeps the HTTP layer decoupled from persistence concretions; matches hexagonal arch intent. Pattern confirmed when wiring email `downloadAttachment` — defined a local interface instead of importing the concrete repo:

```go
type attachmentFinder interface {
    FindByID(ctx context.Context, id string) (*domain.EmailAttachment, error)
}
```

The concrete `*GormEmailAttachmentRepository` satisfies it automatically. `app.go` passes the concrete value as the interface type.

**How to apply:** Any time an HTTP handler package needs a repo method, declare a local interface with only the methods needed. Never import `adapters/persistence` from `adapters/http`.
