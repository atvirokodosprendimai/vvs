---
tldr: Customer portal — magic-link auth, invoice list + detail, dunning email integration
status: active
---

# Plan: Customer Portal — Self-Service Invoice Access

## Context

- Foundation: `PublicModuleRoutes` interface in `internal/infrastructure/http/router.go`
- Foundation: `invoice_tokens` table + `/i/{token}` PDF route (commit 49c49e9)
- Related: `InvoiceToken` domain in `internal/modules/invoice/domain/token.go`
- Related: dunning `SendDunningRemindersHandler` in `cmd/server/dunning_actions.go`
- No spec exists yet — Phase 1 creates it.

## Scope

**In:**
- Magic-link email auth (customer receives link → portal session)
- Admin "Send portal access" action on customer detail page
- `/portal/invoices` — customer sees own invoices (list + HTML detail + PDF link)
- Dunning email embeds portal link
- `vvs_portal` cookie, separate from internal `vvs_session`

**Out:**
- Customer self-service ticket creation
- Payment via portal
- Portal account settings / password

---

## Phase 1 — Spec + Domain — status: open

1. [ ] Create `eidos/spec - portal - customer self-service access.md`
   - define PortalToken, PortalSession, auth flow, route map, security model

2. [ ] Domain model: `internal/modules/portal/domain/`
   - `PortalToken{ID, CustomerID, TokenHash, ExpiresAt, CreatedAt}` — same hash pattern as InvoiceToken
   - `NewPortalToken(customerID string, ttl time.Duration) (*PortalToken, string, error)` — 32-byte rand → base64 → sha256 stored
   - `PortalTokenRepository interface{Save, FindByHash, DeleteByCustomerID}`
   - `PortalSession{ID, CustomerID, ExpiresAt}` (simple struct, stored in memory map or DB)

3. [ ] Migration: `internal/modules/portal/migrations/001_create_portal_tokens.sql`
   ```sql
   CREATE TABLE portal_tokens (
     id TEXT PRIMARY KEY,
     customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
     token_hash TEXT NOT NULL UNIQUE,
     expires_at DATETIME NOT NULL,
     created_at DATETIME NOT NULL
   );
   CREATE INDEX idx_portal_tokens_customer_id ON portal_tokens(customer_id);
   ```
   - portal sessions: reuse same table with short TTL (token IS the session — validate on every request, no separate session table)

---

## Phase 2 — HTTP + Auth Middleware — status: open

4. [ ] Portal module: `internal/modules/portal/adapters/http/handlers.go`
   - `Handlers` struct: `tokenRepo`, `getCustomer`, `listInvoices`
   - `RegisterPublicRoutes(r chi.Router)` — implements `PublicModuleRoutes`
   - Routes:
     - `GET /portal/auth` — validate token param, set cookie, redirect to `/portal`
     - `POST /portal/logout` — clear cookie, redirect to `/portal/auth?expired=1`
     - `GET /portal` → redirect to `/portal/invoices`
     - `GET /portal/invoices` — list customer invoices (requires portal auth)
     - `GET /portal/invoices/{id}` — invoice detail (requires portal auth)

5. [ ] Portal auth middleware: `requirePortalAuth(tokenRepo) func(http.Handler) http.Handler`
   - reads `vvs_portal` cookie
   - sha256 → `FindByHash` → `IsExpired` → attach `customerID` to context
   - unauthorized → redirect to `/portal/auth?expired=1`

6. [ ] Admin "Send portal access" handler
   - `POST /api/customers/{id}/portal-link` — admin-only
   - generates 24h `PortalToken`, saves, emails customer the link
   - returns SSE `PatchElement` with success/error message
   - Add "Portal access" button to customer detail page (contacts/actions card)

---

## Phase 3 — Portal Pages (Templ) — status: open

7. [ ] `PortalLayout` templ — stripped-down layout (no sidebar, no admin nav)
   - header: VVS logo + customer name + logout button
   - neutral/amber palette, same CSS base

8. [ ] `PortalInvoiceListPage` templ
   - table: Invoice #, Date, Due Date, Amount, Status, Actions
   - Actions: "View" → `/portal/invoices/{id}`, "PDF" → `/i/{pdf_token}` (generate on-the-fly or pre-generated)
   - Empty state: "No invoices found"

9. [ ] `PortalInvoiceDetailPage` templ
   - full invoice detail (same data as `InvoicePrintPage` but HTML formatted)
   - "Download PDF" button → generates fresh `/i/{token}` link (24h TTL)
   - "Back to invoices" link

---

## Phase 4 — App Wiring + Dunning Integration — status: open

10. [ ] Wire portal module in `internal/app/app.go`
    - `portalTokenRepo := portalpersistence.NewGormPortalTokenRepository(gdb)`
    - `portalRoutes := portalhttp.NewHandlers(portalTokenRepo, getCustomerQuery, listInvoicesForCustomerQuery)`
    - `portalRoutes.WithInvoiceTokenRepo(invoiceTokenRepo)` — for PDF link generation
    - `moduleRoutes = append(moduleRoutes, portalRoutes)`
    - Add migration to migration list

11. [ ] Dunning email: embed portal link
    - `SendDunningRemindersHandler` gets `portalTokenRepo` + `baseURL` fields
    - Per customer: generate 7d `PortalToken`, save, embed `{baseURL}/portal/auth?token={plain}` in email body
    - Add `WithPortalAccess(repo, baseURL)` setter

---

## Phase 5 — Tests — status: open

12. [ ] Unit tests: `portal/domain/portal_token_test.go`
    - NewPortalToken returns unique token each call
    - IsExpired: false before TTL, true after
    - Hash stored ≠ plaintext

13. [ ] Integration test: `cmd/server/portal_actions_test.go` (or `portal_flow_test.go`)
    - generate token for customer → auth with plain → check customerID in context
    - expired token → redirect to auth page
    - invalid token → 401

14. [ ] E2E: `e2e/portal.spec.js`
    - `/portal/auth?token=...` → redirects to `/portal/invoices`
    - Invoice list visible, pagination works
    - PDF link present on detail page

---

## Verification

```bash
# Unit tests
go test ./internal/modules/portal/...

# Build
templ generate && go build ./...

# Manual
# 1. Admin → customer detail → "Send portal access" → check email
# 2. Click link → /portal/invoices shows customer's invoices
# 3. Click invoice → detail page with PDF button
# 4. PDF button → /i/{token} renders invoice PDF
# 5. Logout → redirected to /portal/auth?expired=1
# 6. Old token URL → redirected (expired or used)

# E2E
npx playwright test e2e/portal.spec.js
```

## Adjustments

## Progress Log
