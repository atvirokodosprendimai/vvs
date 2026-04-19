---
tldr: Reporting module (/reports page, MRR/aging/top-customers) + TOTP-based 2FA for admin accounts
status: active
---

# Plan: Reporting Module + Two-Factor Auth

## Context

- Consilium ruling 2026-04-19: reporting module first (no blockers, reuses existing SVG primitives), 2FA fast-follow (Security+QA consensus — biggest remaining auth gap)
- Related: `eidos/spec - auth - session based authentication.md`
- Foundation: `BuildDonut`, `BarMax`, `fillMonths`, `FormatCurrency` already in `internal/infrastructure/http/dashboard_handler.go`
- Monthly revenue query (strftime grouping, fillMonths) already working on dashboard

---

## Phase 1 — Reporting Module — status: open

### Scope: `/reports` page with MRR trend, invoice aging, top-customers-by-revenue, payment trend

1. [ ] Reports handler: `internal/infrastructure/http/reports_handler.go`
   - New `ReportsData` struct with: `MRR []MonthRevenue` (12 months), `InvoiceAging []AgingBucket`, `TopCustomers []CustomerRevenue`, `PaymentTrend []MonthRevenue`
   - `AgingBucket`: `{Label string, Count int64, Total float64}` — buckets: current (not yet due), 1-30d, 31-60d, 61-90d, 90d+
   - `CustomerRevenue`: `{CustomerID, CustomerName string, Total float64, InvoiceCount int64}`
   - MRR: extend monthly revenue to 12 months (paid invoices grouped by month)
   - Invoice aging: SQL CASE on `julianday(now) - julianday(due_date)` for finalized invoices
   - Top customers: SUM(total) FROM invoices WHERE status='paid' GROUP BY customer_id ORDER BY total DESC LIMIT 10
   - Payment trend: SUM(total) from invoices WHERE status='paid' grouped by paid_at month, last 12 months
   - Register route: `GET /reports` → `newReportsHandler(gorm.DB)`
   - Gate with `RequireAuth` (already on all non-public routes)

2. [ ] Reports template: `internal/infrastructure/http/reports.templ`
   - Page title "Reports" with `PageHeaderRow` component
   - MRR bar chart (12 months) — reuse `BarMax` + existing bar chart pattern from dashboard
   - Invoice aging stacked/grouped bar or table — buckets with count + total
   - Top customers table (rank, name, total revenue, invoice count) — link to customer detail
   - Payment received trend (12-month bar) — paid invoices by paid_at month
   - All-clear state when no data
   - Colors: amber for primary bars, neutral shades for secondary

3. [ ] Nav + RBAC wiring
   - Add "Reports" to Finance nav group in sidebar nav
   - Ensure `ModuleNamed()` returns `"reports"` so per-module RBAC works
   - Wire route in `internal/app/app.go` or router setup
   - Add to `DefaultPermissions` map in auth domain

4. [ ] Tests: `internal/infrastructure/http/reports_handler_test.go`
   - `TestReportsPage_RequiresAuth` — unauthenticated → redirect
   - `TestReportsPage_EmptyDB_Renders` — empty DB → 200, no panic
   - `TestReportsPage_WithData_ContainsCustomer` — seed paid invoice → customer name appears

---

## Phase 2 — Two-Factor Auth (TOTP) — status: open

### Scope: per-user TOTP setup at /profile/2fa; login requires TOTP code when enabled

1. [ ] Domain: TOTP fields + methods on `User`
   - `internal/modules/auth/domain/user.go`: add `TOTPSecret string`, `TOTPEnabled bool`
   - `EnableTOTP(secret string)` — sets secret + flips enabled flag
   - `DisableTOTP()` — clears secret + flips flag
   - `VerifyTOTP(code string) bool` — uses `github.com/pquerna/otp/totp` `Validate()`
   - Add `github.com/pquerna/otp` dependency via `go get`

2. [ ] Migration: `internal/modules/auth/migrations/005_add_totp.sql`
   - `ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT ''`
   - `ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0`

3. [ ] Persistence: map TOTP fields in GORM user model
   - Update `internal/modules/auth/adapters/persistence/repository.go` GORM struct

4. [ ] Profile 2FA setup: `GET /profile/2fa` + `POST /profile/2fa/enable` + `POST /profile/2fa/disable`
   - `GET /profile/2fa`: `totp.Generate(opts)` → QR code as base64 PNG data URI + manual key + confirm-code input
   - `POST /profile/2fa/enable`: verify code against new secret → `user.EnableTOTP(secret)` → save → redirect /profile
   - `POST /profile/2fa/disable`: `user.DisableTOTP()` → save
   - QR: use `github.com/skip2/go-qrcode` — generate PNG bytes → base64 `data:image/png;base64,...`
   - Template: add `TOTPSetupPage` + 2FA section on profile page

5. [ ] Login TOTP step
   - After password passes + `user.TOTPEnabled`: set short-lived signed cookie `vvs_totp_pending` (5min, HMAC-signed user ID) → redirect `/login/totp`
   - `GET /login/totp` → TOTP input page
   - `POST /login/totp`: verify pending cookie → find user → `VerifyTOTP(code)` → clear pending cookie → create full session → redirect `/`
   - Reuse `globalLoginLimiter` for TOTP failure rate limiting
   - Expired/invalid pending → redirect `/login`

6. [ ] Tests
   - Domain: `TestEnableTOTP`, `TestDisableTOTP`, `TestVerifyTOTP_Valid`, `TestVerifyTOTP_Invalid`
   - HTTP: `TestTOTPSetup_ShowsQR`, `TestLogin_WithTOTP_RequiresCode`, `TestLogin_WithTOTP_WrongCode_Rejected`, `TestLogin_WithTOTP_CorrectCode_CreatesSession`

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

<!-- Timestamped entries tracking work done. Updated after every action. -->
