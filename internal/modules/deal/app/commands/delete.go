package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/deal/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type DeleteDealCommand struct {
	ID string
}

type DeleteDealHandler struct {
	repo      domain.DealRepository
	publisher events.EventPublisher
}

func NewDeleteDealHandler(repo domain.DealRepository, pub events.EventPublisher) *DeleteDealHandler {
	return &DeleteDealHandler{repo: repo, publisher: pub}
}

func (h *DeleteDealHandler) Handle(ctx context.Context, cmd DeleteDealCommand) error {
	if _, err := h.repo.FindByID(ctx, cmd.ID); err != nil {
		return err
	}
	if err := h.repo.Delete(ctx, cmd.ID); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.DealDeleted.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "deal.deleted",
		AggregateID: cmd.ID,
		OccurredAt:  time.Now().UTC(),
	})
	return nil
}
