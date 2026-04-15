package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/product/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type UpdateProductCommand struct {
	ID            string
	Name          string
	Description   string
	Type          string
	PriceAmount   int64
	PriceCurrency string
	BillingPeriod string
}

type UpdateProductHandler struct {
	repo      domain.ProductRepository
	publisher events.EventPublisher
}

func NewUpdateProductHandler(repo domain.ProductRepository, pub events.EventPublisher) *UpdateProductHandler {
	return &UpdateProductHandler{repo: repo, publisher: pub}
}

func (h *UpdateProductHandler) Handle(ctx context.Context, cmd UpdateProductCommand) error {
	product, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	if err := product.Update(
		cmd.Name, cmd.Description, cmd.Type,
		cmd.PriceAmount, cmd.PriceCurrency, cmd.BillingPeriod,
	); err != nil {
		return err
	}

	if err := h.repo.Save(ctx, product); err != nil {
		return err
	}

	data, _ := json.Marshal(domainToReadModel(product))

	h.publisher.Publish(ctx, "isp.product.updated", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "product.updated",
		AggregateID: product.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
