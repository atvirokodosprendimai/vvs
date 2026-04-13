package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/payment/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type PaymentImporter interface {
	Format() string
	Parse(ctx context.Context, reader io.Reader) ([]*domain.Payment, error)
}

type ImportPaymentsCommand struct {
	Format string
	Reader io.Reader
}

type ImportPaymentsHandler struct {
	repo      domain.PaymentRepository
	importers map[string]PaymentImporter
	publisher events.EventPublisher
}

func NewImportPaymentsHandler(repo domain.PaymentRepository, pub events.EventPublisher) *ImportPaymentsHandler {
	return &ImportPaymentsHandler{
		repo:      repo,
		importers: make(map[string]PaymentImporter),
		publisher: pub,
	}
}

func (h *ImportPaymentsHandler) RegisterImporter(i PaymentImporter) {
	h.importers[i.Format()] = i
}

func (h *ImportPaymentsHandler) Handle(ctx context.Context, cmd ImportPaymentsCommand) ([]*domain.Payment, error) {
	importer, ok := h.importers[cmd.Format]
	if !ok {
		return nil, fmt.Errorf("unsupported import format: %s", cmd.Format)
	}

	payments, err := importer.Parse(ctx, cmd.Reader)
	if err != nil {
		return nil, fmt.Errorf("parsing import file: %w", err)
	}

	if len(payments) == 0 {
		return nil, nil
	}

	if err := h.repo.SaveBatch(ctx, payments); err != nil {
		return nil, fmt.Errorf("saving imported payments: %w", err)
	}

	data, _ := json.Marshal(map[string]interface{}{
		"format": cmd.Format,
		"count":  len(payments),
	})

	h.publisher.Publish(ctx, "isp.payment.imported", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "payment.imported",
		AggregateID: payments[0].ImportBatchID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return payments, nil
}
