package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type DeleteProductCommand struct {
	ID string
}

type DeleteProductHandler struct {
	repo      domain.ProductRepository
	publisher events.EventPublisher
}

func NewDeleteProductHandler(repo domain.ProductRepository, pub events.EventPublisher) *DeleteProductHandler {
	return &DeleteProductHandler{repo: repo, publisher: pub}
}

func (h *DeleteProductHandler) Handle(ctx context.Context, cmd DeleteProductCommand) error {
	product, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	if err := h.repo.Delete(ctx, cmd.ID); err != nil {
		return err
	}

	data, _ := json.Marshal(map[string]string{
		"id":   product.ID,
		"name": product.Name,
	})

	h.publisher.Publish(ctx, events.ProductDeleted.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "product.deleted",
		AggregateID: product.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
