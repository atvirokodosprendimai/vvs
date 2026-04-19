package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type CreateProductCommand struct {
	Name          string
	Description   string
	Type          string
	PriceAmount   int64
	PriceCurrency string
	BillingPeriod string
}

type CreateProductHandler struct {
	repo      domain.ProductRepository
	publisher events.EventPublisher
}

func NewCreateProductHandler(repo domain.ProductRepository, pub events.EventPublisher) *CreateProductHandler {
	return &CreateProductHandler{repo: repo, publisher: pub}
}

func (h *CreateProductHandler) Handle(ctx context.Context, cmd CreateProductCommand) (*domain.Product, error) {
	product, err := domain.NewProduct(cmd.Name, cmd.Description, cmd.Type, cmd.PriceAmount, cmd.PriceCurrency, cmd.BillingPeriod)
	if err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, product); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(domainToReadModel(product))

	h.publisher.Publish(ctx, events.ProductCreated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "product.created",
		AggregateID: product.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return product, nil
}
