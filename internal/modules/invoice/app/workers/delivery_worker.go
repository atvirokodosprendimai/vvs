package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/vvs/isp/internal/modules/invoice/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// Mailer sends a plain email. Implemented by app.go wiring.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// CustomerEmailGetter fetches a customer's email address by ID.
type CustomerEmailGetter interface {
	GetCustomerEmail(ctx context.Context, customerID string) (string, error)
}

// InvoiceDeliveryWorker subscribes to isp.invoice.finalized and sends the invoice by email.
type InvoiceDeliveryWorker struct {
	mailer         Mailer
	customerGetter CustomerEmailGetter
}

func NewInvoiceDeliveryWorker(mailer Mailer, getter CustomerEmailGetter) *InvoiceDeliveryWorker {
	return &InvoiceDeliveryWorker{mailer: mailer, customerGetter: getter}
}

// Run blocks until ctx is cancelled.
func (w *InvoiceDeliveryWorker) Run(ctx context.Context, sub events.EventSubscriber) {
	ch, cancel := sub.ChanSubscription(events.InvoiceFinalized.String())
	defer cancel()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			w.handleFinalized(ctx, event)
		case <-ctx.Done():
			return
		}
	}
}

func (w *InvoiceDeliveryWorker) handleFinalized(ctx context.Context, event events.DomainEvent) {
	var inv domain.Invoice
	if err := json.Unmarshal(event.Data, &inv); err != nil {
		log.Printf("invoice delivery: unmarshal event: %v", err)
		return
	}

	if inv.CustomerID == "" {
		return
	}

	toEmail, err := w.customerGetter.GetCustomerEmail(ctx, inv.CustomerID)
	if err != nil || toEmail == "" {
		log.Printf("invoice delivery: no email for customer %s — skipping", inv.CustomerID)
		return
	}

	subject := fmt.Sprintf("Invoice %s — %s", inv.Code, inv.CustomerName)
	body := buildInvoiceEmailBody(&inv)

	if err := w.mailer.Send(ctx, toEmail, subject, body); err != nil {
		log.Printf("invoice delivery: send to %s: %v", toEmail, err)
		return
	}
	log.Printf("invoice delivery: sent invoice %s to %s", inv.Code, toEmail)
}

func buildInvoiceEmailBody(inv *domain.Invoice) string {
	lines := ""
	for _, li := range inv.LineItems {
		lines += fmt.Sprintf("  - %s x%d: %s %.2f (incl. %d%% VAT)\n",
			li.ProductName, li.Quantity, inv.Currency,
			float64(li.TotalGross)/100, li.VATRate)
	}
	return fmt.Sprintf(`Dear %s,

Please find your invoice details below.

Invoice: %s
Issue date: %s
Due date:   %s

Items:
%s
Subtotal (net): %s %.2f
VAT:            %s %.2f
Total due:      %s %.2f

Please make payment by the due date.

Thank you for your business.
`,
		inv.CustomerName,
		inv.Code,
		inv.IssueDate.Format("2006-01-02"),
		inv.DueDate.Format("2006-01-02"),
		lines,
		inv.Currency, float64(inv.SubTotal)/100,
		inv.Currency, float64(inv.VATTotal)/100,
		inv.Currency, float64(inv.TotalAmount)/100,
	)
}
