---
status: completed
created: 2026-04-19
consilium: 2026-04-19 unanimous Today Dashboard + QA dunning gap
---

# Plan: Dunning Integration Test + Next Feature Consilium

## Context

Consilium 2026-04-19 ruled:
- Today Dashboard → ALREADY COMPLETE (94daeb0)
- Audit log CRM tab → ALREADY COMPLETE (64c1d0a)
- Service mutations audit → ALREADY COMPLETE

**Actual gap:** QA correctly identified that dunning has 5 mock-only unit tests but **no integration test** exercising the real DB → cron action → `ReminderSentAt` set path. Risk: paid invoice could still receive reminder if `PaidAt` propagation has a bug.

CEO ruling: Dunning integration test ships first, then Consilium for next big feature.

---

## Phase 1 — Dunning Integration Test

**File:** `cmd/server/dunning_actions_test.go` (package `main`, same pattern as `billing_actions_test.go`)

### Test setup (reuse billing pattern)
```go
func setupDunningTest(t *testing.T) *gormsqlite.DB {
    db := testutil.NewTestDB(t)
    testutil.RunMigrations(t, db, customermigrations.FS, "goose_customer")
    testutil.RunMigrations(t, db, invoicemigrations.FS, "goose_invoice")
    return db
}
```

### Test cases (3)

1. **Happy path — overdue invoice gets reminder, ReminderSentAt set**
   - Create invoice directly via `invoiceRepo.Save` with:
     - `Status = domain.StatusFinalized`
     - `DueDate = time.Now().Add(-48 * time.Hour)` (2 days overdue)
     - `ReminderSentAt = nil`
   - `RegisterDunningActions(db, nil)` with stub SMTP (capture email, not real send)
   - Call `actions["send-dunning-reminders"](ctx)`
   - Assert: invoice from DB has `ReminderSentAt != nil`

2. **Paid invoice is skipped — no reminder sent**
   - Create invoice with `Status = domain.StatusPaid`, `DueDate` past
   - Run action
   - Assert: no email sent, `ReminderSentAt` still nil

3. **Recently reminded invoice is skipped — cooldown respected**
   - Create overdue invoice with `ReminderSentAt = time.Now().Add(-1 * time.Hour)` (1 hour ago)
   - Run action with default 7-day interval
   - Assert: no email sent, `ReminderSentAt` unchanged

### SMTP stub approach
`RegisterDunningActions` takes `emailEncKey []byte`. Need a way to inject a stub mailer for tests.

Options:
- A) Add `RegisterDunningActionsWithMailer(gdb, mailer EmailSender)` variant for tests
- B) Use `nil` encKey → mailer fails gracefully → assert no panic but skip send
- **Use A** — clean, matches billing test pattern, no silent failures

Add to `dunning_actions.go`:
```go
// RegisterDunningActionsWithMailer is the testable variant — accepts a pre-built mailer.
func RegisterDunningActionsWithMailer(gdb *gormsqlite.DB, mailer invoicecommands.EmailSender) {
    invoiceRepo := invoicepersistence.NewInvoiceRepository(gdb)
    customerRepo := customerpersistence.NewGormCustomerRepository(gdb)
    handler := invoicecommands.NewSendDunningRemindersHandler(invoiceRepo, customerRepo, mailer)
    RegisterAction("send-dunning-reminders", func(ctx context.Context) error {
        result, err := handler.Handle(ctx, invoicecommands.SendDunningRemindersCommand{})
        if err != nil { return err }
        for _, e := range result.Errors { log.Printf("dunning: %s", e) }
        return nil
    })
}
```

### Stub mailer for tests
```go
type captureMailer struct { sent []string }
func (m *captureMailer) SendPlain(_ context.Context, to, subject, _ string) error {
    m.sent = append(m.sent, to)
    return nil
}
```

---

## Phase 2 — Next Consilium (After Integration Test Ships)

Top candidates for next Consilium (in order of strategic importance per Security):
1. **Multi-user RBAC** — admin / billing / read-only roles; prerequisite before portal or second operator
2. **Customer portal** — self-service invoice view; requires RBAC to be in place first
3. **Revenue reporting** — monthly revenue vs prior month, outstanding balance by customer

Likely CEO ruling will be: RBAC first if a second operator is imminent; Reporting first if single-operator stays.

---

## Verification

```bash
# Phase 1
go test ./cmd/server/ -run TestDunningAction -v

# Confirm dunning still builds
go build ./cmd/server/
```
