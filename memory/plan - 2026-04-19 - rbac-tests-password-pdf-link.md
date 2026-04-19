# Plan: RBAC Tests + Change Password + PDF Portal Link

## Context

Consilium ruling 2026-04-19. Three phases ordered by security gap → admin friction → customer friction.

---

## Phase 1 — RBAC Middleware Tests

**Why:** `RequireWrite` middleware has zero tests. Viewer could accidentally get write access if middleware is misconfigured or dropped in a refactor. QA flagged as highest regression risk.

### File

`internal/infrastructure/http/auth_middleware_test.go` (new file, package `http_test`)

### Test cases

1. **Viewer POST → 403**
   - Build `httptest.Server` with `RequireWrite` wrapping a handler that writes 200
   - Make request with viewer-role user in context
   - Assert 403

2. **Operator POST → passes through**
   - Same setup, operator-role user
   - Assert handler reached (200)

3. **Admin POST → passes through**
   - Admin-role user
   - Assert 200

4. **GET request → always passes through (read-only)**
   - Viewer-role user, GET request
   - Assert 200 (RequireWrite only blocks mutating methods)

5. **No user in context → passes through (unauthenticated, let RequireAuth handle it)**
   - nil user in context
   - Assert 200 (RequireWrite does not double-gate)

### Key pattern

```go
func makeUser(role authdomain.Role) *authdomain.User {
    u, _ := authdomain.NewUser("test", "pass", role)
    return u
}

func withUser(u *authdomain.User, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := authhttp.WithUser(r.Context(), u)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## Phase 2 — Change Password (Self-Service)

**Why:** Every password reset requires admin delete+recreate. Prerequisite for any customer portal future work.

### Domain change

`internal/modules/auth/domain/user.go` — `ChangePassword` already exists. No change needed.

### HTTP handler

`internal/modules/auth/adapters/http/handlers.go`

Add `ChangeMyPasswordHandler`:

```go
// POST /api/users/me/password
// Signals: {currentPassword: "", newPassword: ""}
func (h *AuthHandlers) ChangeMyPasswordHandler(w http.ResponseWriter, r *http.Request) {
    user := authhttp.UserFromContext(r.Context())
    // parse body: currentPassword, newPassword
    if !user.VerifyPassword(req.CurrentPassword) {
        // render error fragment
        return
    }
    if err := user.ChangePassword(req.NewPassword); err != nil {
        // render error fragment
        return
    }
    if err := h.userRepo.Save(ctx, user); err != nil { ... }
    // render success fragment or redirect
}
```

### Template additions

`internal/modules/auth/adapters/http/templates.templ`

Add `ProfilePage` templ:
- Shows current username (read-only display)
- Change password form: `currentPassword`, `newPassword` signal inputs
- `data-on:click="@post('/api/users/me/password')"` button
- `<div id="change-pw-error"></div>` fragment target
- `<div id="change-pw-success"></div>` fragment target

Add `changePasswordError(msg string)` + `changePasswordSuccess()` fragments.

### Router wiring

`internal/infrastructure/http/router.go`:
```go
r.Post("/api/users/me/password", authHandlers.ChangeMyPasswordHandler)
```

Nav: add "Profile" link in sidebar bottom section (near logout).

### Tests

`internal/modules/auth/adapters/http/handlers_test.go` or `change_password_test.go`:
1. Wrong current password → 422 or error fragment
2. Empty new password → error fragment
3. Valid change → success fragment + DB updated

---

## Phase 3 — PDF Portal Link

**Why:** Dunning emails go out but customers have no way to view their invoice. Dead-end flow.

### Migration

`internal/modules/invoice/migrations/XXX_add_invoice_tokens.sql`:
```sql
CREATE TABLE invoice_tokens (
    id         TEXT PRIMARY KEY,
    invoice_id TEXT NOT NULL REFERENCES invoices(id),
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);
```

### Domain

`internal/modules/invoice/domain/token.go`:
```go
type InvoiceToken struct {
    ID        string
    InvoiceID string
    TokenHash string
    ExpiresAt time.Time
    CreatedAt time.Time
}

func NewInvoiceToken(invoiceID string, ttl time.Duration) (*InvoiceToken, string) {
    raw := make([]byte, 32)
    rand.Read(raw)
    plain := base64.RawURLEncoding.EncodeToString(raw)
    hash := sha256.Sum256([]byte(plain))
    return &InvoiceToken{
        ID:        uuid.New().String(),
        InvoiceID: invoiceID,
        TokenHash: hex.EncodeToString(hash[:]),
        ExpiresAt: time.Now().Add(ttl),
        CreatedAt: time.Now(),
    }, plain
}

func (t *InvoiceToken) IsExpired() bool { return time.Now().After(t.ExpiresAt) }
```

### Repository

`internal/modules/invoice/domain/token_repository.go`:
```go
type InvoiceTokenRepository interface {
    Save(ctx context.Context, t *InvoiceToken) error
    FindByHash(ctx context.Context, hash string) (*InvoiceToken, error)
}
```

`internal/modules/invoice/adapters/persistence/token_repository.go` — GORM impl.

### HTTP handler (public, no auth required)

`internal/modules/invoice/adapters/http/handlers.go` — add `PublicInvoiceByToken`:

```go
// GET /i/{token}
// No auth middleware — public route
func (h *InvoiceHandlers) PublicInvoiceByToken(w http.ResponseWriter, r *http.Request) {
    plain := chi.URLParam(r, "token")
    hash := sha256hex(plain)
    token, err := h.tokenRepo.FindByHash(ctx, hash)
    if err != nil || token.IsExpired() {
        http.Error(w, "Link expired or not found", 404)
        return
    }
    inv, _ := h.invoiceRepo.FindByID(ctx, token.InvoiceID)
    w.Header().Set("Content-Type", "application/pdf")
    w.Header().Set("Cache-Control", "no-store")
    w.Header().Set("Referrer-Policy", "no-referrer")
    renderPDF(w, inv)
}
```

### Router wiring

`internal/infrastructure/http/router.go` — add BEFORE `RequireAuth`:
```go
r.Get("/i/{token}", invoiceHandlers.PublicInvoiceByToken)
```

### Dunning integration

`internal/modules/invoice/app/commands/send_dunning.go` — when sending reminder email, generate token and include link in body:
```go
token, plain := domain.NewInvoiceToken(inv.ID, 48*time.Hour)
h.tokenRepo.Save(ctx, token)
link := fmt.Sprintf("https://%s/i/%s", h.baseURL, plain)
body = fmt.Sprintf("...\n\nView invoice: %s", link)
```

Add `tokenRepo InvoiceTokenRepository` + `baseURL string` to `SendDunningRemindersHandler`.

### Tests

1. Valid token → returns PDF (200, content-type application/pdf)
2. Expired token → 404
3. Unknown token → 404
4. Token generated correctly (32-byte entropy, SHA-256 hash stored)

---

## Execution Order

```
Phase 1: auth_middleware_test.go → go test ./internal/infrastructure/http/...
Phase 2: handlers.go + templates.templ + router → go build ./...
Phase 3: migration + domain + repo + handler → go build ./... + go test ./internal/modules/invoice/...
```

## Verification

```bash
# Phase 1
go test ./internal/infrastructure/http/... -run TestRequireWrite

# Phase 2
go build ./...
# Login as viewer → POST /api/users/me/password → 403 (RequireWrite)
# Login as operator → change password form → works

# Phase 3
go build ./...
# Finalize invoice → send dunning → email contains /i/{token} link
# Click link → PDF served, no login required
# Wait 48h (or set TTL to 1s in test) → 404
```
