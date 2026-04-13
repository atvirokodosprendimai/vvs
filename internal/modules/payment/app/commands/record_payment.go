package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/payment/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type RecordPaymentCommand struct {
	AmountCents int64
	Currency    string
	Reference   string
	PayerName   string
	PayerIBAN   string
	BookingDate time.Time
}

type RecordPaymentHandler struct {
	repo      domain.PaymentRepository
	publisher events.EventPublisher
}

func NewRecordPaymentHandler(repo domain.PaymentRepository, pub events.EventPublisher) *RecordPaymentHandler {
	return &RecordPaymentHandler{repo: repo, publisher: pub}
}

func (h *RecordPaymentHandler) Handle(ctx context.Context, cmd RecordPaymentCommand) (*domain.Payment, error) {
	currency := cmd.Currency
	if currency == "" {
		currency = "EUR"
	}

	payment := domain.NewPayment(
		shareddomain.NewMoney(cmd.AmountCents, currency),
		cmd.Reference,
		cmd.PayerName,
		cmd.PayerIBAN,
		cmd.BookingDate,
		"", // manual entry, no batch
	)

	if err := h.repo.Save(ctx, payment); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(map[string]string{
		"id":        payment.ID,
		"reference": payment.Reference,
	})

	h.publisher.Publish(ctx, "isp.payment.recorded", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "payment.recorded",
		AggregateID: payment.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return payment, nil
}
