package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/network/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type DeleteRouterHandler struct {
	repo      domain.RouterRepository
	publisher events.EventPublisher
}

func NewDeleteRouterHandler(repo domain.RouterRepository, pub events.EventPublisher) *DeleteRouterHandler {
	return &DeleteRouterHandler{repo: repo, publisher: pub}
}

func (h *DeleteRouterHandler) Handle(ctx context.Context, id string) error {
	router, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := h.repo.Delete(ctx, id); err != nil {
		return err
	}

	data, _ := json.Marshal(map[string]string{"id": router.ID, "name": router.Name})
	h.publisher.Publish(ctx, "isp.network.router.deleted", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "network.router.deleted",
		AggregateID: router.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return nil
}
