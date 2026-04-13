package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/payment/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type MatchPaymentCommand struct {
	PaymentID  string
	InvoiceID  string
	CustomerID string
}

type MatchPaymentHandler struct {
	repo      domain.PaymentRepository
	publisher events.EventPublisher
}

func NewMatchPaymentHandler(repo domain.PaymentRepository, pub events.EventPublisher) *MatchPaymentHandler {
	return &MatchPaymentHandler{repo: repo, publisher: pub}
}

func (h *MatchPaymentHandler) Handle(ctx context.Context, cmd MatchPaymentCommand) error {
	payment, err := h.repo.FindByID(ctx, cmd.PaymentID)
	if err != nil {
		return err
	}

	if err := payment.Match(cmd.InvoiceID, cmd.CustomerID); err != nil {
		return err
	}

	if err := h.repo.Save(ctx, payment); err != nil {
		return err
	}

	data, _ := json.Marshal(map[string]string{
		"payment_id": payment.ID,
		"invoice_id": cmd.InvoiceID,
		"customer_id": cmd.CustomerID,
	})

	h.publisher.Publish(ctx, "isp.payment.matched", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "payment.matched",
		AggregateID: payment.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
