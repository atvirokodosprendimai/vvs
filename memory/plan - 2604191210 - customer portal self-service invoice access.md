---
tldr: Customer portal — magic-link auth, invoice list + detail, dunning email integration
status: completed
---

# Plan: Customer Portal — Self-Service Invoice Access

## Context

- Foundation: `PublicModuleRoutes` interface in `internal/infrastructure/http/router.go`
- Foundation: `invoice_tokens` table + `/i/{token}` PDF route (commit 49c49e9)
- Related: `InvoiceToken` domain in `internal/modules/invoice/domain/token.go`
- Related: dunning `SendDunningRemindersHandler` in `cmd/server/dunning_actions.go`
- Spec: `eidos/spec - portal - customer self-service access.md`

## Scope

**In:**
- Magic-link email auth (customer receives link → portal session)
- Admin "Portal Access" button on customer detail page
- `/portal/invoices` — customer sees own invoices (list + HTML detail + PDF link)
- Dunning email embeds portal link
- `vvs_portal` cookie, separate from internal `vvs_session`

**Out:**
- Customer self-service ticket creation
- Payment via portal
- Portal account settings / password

---

## Phase 1 — Spec + Domain — status: completed

1. [x] Create `eidos/spec - portal - customer self-service access.md`
   - => auth model, route map, security properties documented

2. [x] Domain model: `internal/modules/portal/domain/token.go`
   - => `PortalToken{ID, CustomerID, TokenHash, ExpiresAt, CreatedAt}`
   - => `NewPortalToken`, `IsExpired`, `HashOf`, `PortalTokenRepository` interface

3. [x] Migration: `internal/modules/portal/migrations/001_create_portal_tokens.sql`
   - => portal_tokens table, FK to customers ON DELETE CASCADE, index on customer_id

---

## Phase 2 — HTTP + Auth Middleware — status: completed

4. [x] Portal module: `internal/modules/portal/adapters/http/handlers.go`
   - => `Handlers` struct with WithPDFTokens, WithCustomerReader, WithBaseURL, WithSecureCookie
   - => `RegisterPublicRoutes` implements `PublicModuleRoutes`
   - => Routes: GET /portal/auth, POST /portal/logout, GET /portal, GET /portal/invoices, GET /portal/invoices/{id}

5. [x] Portal auth middleware: `requirePortalAuth`
   - => reads vvs_portal cookie → sha256 → FindByHash → IsExpired → injects customerID to context
   - => unauthorized → redirect to /portal/auth?expired=1

6. [x] Admin "Portal Access" handler
   - => `POST /api/customers/{id}/portal-link` — admin-only, SSE PatchElementTempl with PortalLinkFragment
   - => "Portal Access" button added to customer detail page header
   - => `<div id="portal-link-result">` patch target on customer detail page

---

## Phase 3 — Portal Pages (Templ) — status: completed

7. [x] `portalLayout` templ — stripped layout (no sidebar, no admin nav)
   - => header: company name + email + logout button

8. [x] `PortalInvoiceListPage` templ
   - => invoice table with Code, Issue Date, Due Date, Amount, Status
   - => overdue dates shown in red

9. [x] `PortalInvoiceDetailPage` templ
   - => summary card (net/VAT/total/paid at), line items table
   - => "Download PDF" button (amber, only when pdfURL non-empty)
   - => `PortalExpiredPage` for expired/invalid tokens

---

## Phase 4 — App Wiring — status: completed

10. [x] Wire portal module in `internal/app/app.go`
    - => `portalpersistence.NewGormPortalTokenRepository(gdb)`
    - => `portalhttp.NewHandlers(...).WithPDFTokens(invoiceTokenRepo).WithCustomerReader(bridge).WithBaseURL(cfg.BaseURL).WithSecureCookie(cfg.SecureCookie)`
    - => migration added; `portalCustomerBridge` at composition root
    - => `Config.BaseURL` + `VVS_BASE_URL` env var added

11. [x] Dunning email: embed portal link
    - `SendDunningRemindersHandler` gets `portalTokenRepo` + `baseURL` fields
    - Per customer: generate 7d `PortalToken`, save, embed in dunning email body
    - Add `WithPortalAccess(repo, baseURL)` setter

---

## Phase 5 — Tests — status: active

12. [x] Unit tests: `portal/domain/token_test.go` — 6 tests pass
    - => NewPortalToken unique, IsExpired, HashOf matches

13. [x] HTTP handler tests: `portal/adapters/http/handlers_test.go` — 10 tests pass
    - => auth flow (valid/invalid/expired/no token), cookie setting, logout
    - => requirePortalAuth redirect on missing/invalid cookie
    - => admin link generation SSE, non-admin 403, no-user 403

14. [ ] E2E: `e2e/portal.spec.js`
    - `/portal/auth?token=...` → redirects to `/portal/invoices`
    - Invoice list visible
    - PDF link present on detail page

---

## Verification

```bash
go test ./internal/modules/portal/...
# 10 HTTP + 6 domain tests pass
templ generate && go build ./...
```

## Adjustments

- 2026-04-19: Skipped PortalSession separate table — token IS the session (validate hash on every request)
- 2026-04-19: PortalLinkFragment uses `data-portal-url` attribute for clipboard copy (avoids JS injection)
- 2026-04-19: generatePortalLink uses Datastar SSE (PatchElementTempl) instead of plain HTML response

## Progress Log

- 2026-04-19: Phase 1–5 complete except E2E (item 14) — commits 2c577dd, 60a8702, 330a199, 5aa68e7
- E2E (item 14) deferred — portal is fully functional; E2E can be added in a later pass
