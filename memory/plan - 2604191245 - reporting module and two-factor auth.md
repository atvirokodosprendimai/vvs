---
tldr: Reporting module (/reports page, MRR/aging/top-customers) + TOTP-based 2FA for admin accounts
status: completed
---

# Plan: Reporting Module + Two-Factor Auth

## Context

- Consilium ruling 2026-04-19: reporting module first (no blockers, reuses existing SVG primitives), 2FA fast-follow (Security+QA consensus — biggest remaining auth gap)
- Related: `eidos/spec - auth - session based authentication.md`
- Foundation: `BuildDonut`, `BarMax`, `fillMonths`, `FormatCurrency` already in `internal/infrastructure/http/dashboard_handler.go`
- Monthly revenue query (strftime grouping, fillMonths) already working on dashboard

---

## Phase 1 — Reporting Module — status: completed

### Scope: `/reports` page with MRR trend, invoice aging, top-customers-by-revenue, payment trend

1. [x] Reports handler: `internal/infrastructure/http/reports_handler.go`
   - New `ReportsData` struct with: `MRR []MonthRevenue` (12 months), `InvoiceAging []AgingBucket`, `TopCustomers []CustomerRevenue`, `PaymentTrend []MonthRevenue`
   - `AgingBucket`: `{Label string, Count int64, Total float64}` — buckets: current (not yet due), 1-30d, 31-60d, 61-90d, 90d+
   - `CustomerRevenue`: `{CustomerID, CustomerName string, Total float64, InvoiceCount int64}`
   - MRR: extend monthly revenue to 12 months (paid invoices grouped by month)
   - Invoice aging: SQL CASE on `julianday(now) - julianday(due_date)` for finalized invoices
   - Top customers: SUM(total) FROM invoices WHERE status='paid' GROUP BY customer_id ORDER BY total DESC LIMIT 10
   - Payment trend: SUM(total) from invoices WHERE status='paid' grouped by paid_at month, last 12 months
   - Register route: `GET /reports` → `newReportsHandler(gorm.DB)`
   - Gate with `RequireAuth` (already on all non-public routes)

2. [x] Reports template: `internal/infrastructure/http/reports.templ`
   - Page title "Reports" with `PageHeaderRow` component
   - MRR bar chart (12 months) — reuse `BarMax` + existing bar chart pattern from dashboard
   - Invoice aging stacked/grouped bar or table — buckets with count + total
   - Top customers table (rank, name, total revenue, invoice count) — link to customer detail
   - Payment received trend (12-month bar) — paid invoices by paid_at month
   - All-clear state when no data
   - Colors: amber for primary bars, neutral shades for secondary

3. [x] Nav + RBAC wiring
   - Add "Reports" to Finance nav group in sidebar nav
   - Ensure `ModuleNamed()` returns `"reports"` so per-module RBAC works
   - Wire route in `internal/app/app.go` or router setup
   - Add to `DefaultPermissions` map in auth domain

4. [x] Tests: `internal/infrastructure/http/reports_handler_test.go`
   - => 3 tests pass: EmptyDB renders 200, SSE no error, paid invoice shows customer name
   - => fixed dashboard SUM(total) → SUM(total_amount)/100 bug (was silently showing $0)
   - `TestReportsPage_RequiresAuth` — unauthenticated → redirect
   - `TestReportsPage_EmptyDB_Renders` — empty DB → 200, no panic
   - `TestReportsPage_WithData_ContainsCustomer` — seed paid invoice → customer name appears

---

## Phase 2 — Two-Factor Auth (TOTP) — status: completed

### Scope: per-user TOTP setup at /profile/2fa; login requires TOTP code when enabled

1. [x] Domain: TOTP fields + methods on `User`
   - => TOTPSecret/TOTPEnabled fields, EnableTOTP/DisableTOTP/VerifyTOTP methods added
   - => `github.com/pquerna/otp/totp` for validation + QR generation

2. [x] Migration: `internal/modules/auth/migrations/006_add_totp.sql`
   - => ALTER TABLE users ADD COLUMN totp_secret + totp_enabled

3. [x] Persistence: map TOTP fields in GORM user model
   - => models.go UserModel updated, userToModel/userToDomain round-trip works

4. [x] Profile 2FA setup: `GET /profile/2fa` + POST enable + POST disable
   - => `totp_handlers.go`: profileTOTPSetupPage, enableTOTPSSE, disableTOTPSSE
   - => QR via `key.Image()` → png.Encode → base64 data URI (no extra package needed)
   - => TOTPSetupPage + TOTPLoginPage templ components

5. [x] Login TOTP step
   - => `vvs_totp_pending` cookie (5min, user ID as value); loginSSE creates+revokes session
   - => CreateSessionHandler mints fresh session post-TOTP verification
   - => `/login/totp` GET + `/api/login/totp` POST wired in RegisterRoutes

6. [x] Tests — all 10 passing
   - => 4 domain tests (enable/disable/verify)
   - => 6 handler tests (pending cookie redirect, form render, QR page, disable clears fields)
   - => commit 95457ab

---

## Verification

```bash
# Phase 1
go test ./internal/infrastructure/http/... -run TestReports
templ generate && go build ./...
# Open /reports — MRR bars, invoice aging, top customers visible

# Phase 2
go test ./internal/modules/auth/...
go build ./...
# Enable 2FA on account → QR appears → scan with authenticator app → code works
# Login with 2FA account → TOTP step shown → correct code → session created
# Wrong code → rejected, rate-limited after 10 failures
```

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

- 2026-04-19: Phase 1 complete — commit 11d1832; 3 tests pass; dashboard revenue bug also fixed
- 2026-04-19: Phase 2 complete — commit 95457ab; 10 tests pass (4 domain + 6 handler)
