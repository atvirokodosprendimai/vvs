package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/contact/domain"
	"github.com/vvs/isp/internal/shared/events"
	"time"
)

type DeleteContactCommand struct {
	ID string
}

type DeleteContactHandler struct {
	repo      domain.ContactRepository
	publisher events.EventPublisher
}

func NewDeleteContactHandler(repo domain.ContactRepository, pub events.EventPublisher) *DeleteContactHandler {
	return &DeleteContactHandler{repo: repo, publisher: pub}
}

func (h *DeleteContactHandler) Handle(ctx context.Context, cmd DeleteContactCommand) error {
	if _, err := h.repo.FindByID(ctx, cmd.ID); err != nil {
		return err
	}
	if err := h.repo.Delete(ctx, cmd.ID); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.ContactDeleted.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "contact.deleted",
		AggregateID: cmd.ID,
		OccurredAt:  time.Now().UTC(),
	})
	return nil
}
