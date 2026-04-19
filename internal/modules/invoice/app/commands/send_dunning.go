package commands

import (
	"context"
	"fmt"
	"time"

	customerdomain "github.com/vvs/isp/internal/modules/customer/domain"
	"github.com/vvs/isp/internal/modules/invoice/domain"
)

// DunningInterval is the minimum time between reminder emails for the same invoice.
const DunningInterval = 7 * 24 * time.Hour

// EmailSender is the port for sending plain-text emails from the dunning command.
type EmailSender interface {
	SendPlain(ctx context.Context, to, subject, body string) error
}

// SendDunningRemindersCommand triggers the dunning run.
type SendDunningRemindersCommand struct {
	// Interval overrides DunningInterval when non-zero (useful for testing).
	Interval time.Duration
}

// SendDunningRemindersResult reports the outcome of a dunning run.
type SendDunningRemindersResult struct {
	Sent   []string // invoice codes that received a reminder
	Errors []string // "INV-XXX: <reason>" entries
}

// SendDunningRemindersHandler finds overdue invoices and sends email reminders.
type SendDunningRemindersHandler struct {
	invoices  domain.InvoiceRepository
	customers customerdomain.CustomerRepository
	mailer    EmailSender
}

func NewSendDunningRemindersHandler(
	invoices domain.InvoiceRepository,
	customers customerdomain.CustomerRepository,
	mailer EmailSender,
) *SendDunningRemindersHandler {
	return &SendDunningRemindersHandler{
		invoices:  invoices,
		customers: customers,
		mailer:    mailer,
	}
}

func (h *SendDunningRemindersHandler) Handle(ctx context.Context, cmd SendDunningRemindersCommand) (SendDunningRemindersResult, error) {
	interval := cmd.Interval
	if interval == 0 {
		interval = DunningInterval
	}

	overdue, err := h.invoices.ListOverdue(ctx)
	if err != nil {
		return SendDunningRemindersResult{}, fmt.Errorf("list overdue: %w", err)
	}

	var result SendDunningRemindersResult
	for _, inv := range overdue {
		if !inv.NeedsReminder(interval) {
			continue
		}

		customer, err := h.customers.FindByID(ctx, inv.CustomerID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: customer lookup: %v", inv.Code, err))
			continue
		}
		if customer.Email == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: customer has no email", inv.Code))
			continue
		}

		subject := fmt.Sprintf("Payment reminder: %s (due %s)", inv.Code, inv.DueDate.Format("2006-01-02"))
		body := buildReminderBody(inv, customer)

		if err := h.mailer.SendPlain(ctx, customer.Email, subject, body); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: send email: %v", inv.Code, err))
			continue
		}

		if err := inv.MarkReminderSent(); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: mark reminder: %v", inv.Code, err))
			continue
		}
		if err := h.invoices.Save(ctx, inv); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: save: %v", inv.Code, err))
			continue
		}

		result.Sent = append(result.Sent, inv.Code)
	}

	return result, nil
}

func buildReminderBody(inv *domain.Invoice, customer *customerdomain.Customer) string {
	overdueDays := int(time.Since(inv.DueDate).Hours() / 24)
	return fmt.Sprintf(`Dear %s,

This is a friendly reminder that invoice %s for %.2f %s was due on %s (%d day(s) ago) and remains unpaid.

Please arrange payment at your earliest convenience.

Invoice details:
  Code:    %s
  Amount:  %.2f %s
  Due:     %s

If you have already made payment, please disregard this message.

Thank you.`,
		customer.ContactName,
		inv.Code,
		float64(inv.TotalAmount)/100, inv.Currency,
		inv.DueDate.Format("2006-01-02"),
		overdueDays,
		inv.Code,
		float64(inv.TotalAmount)/100, inv.Currency,
		inv.DueDate.Format("2006-01-02"),
	)
}
