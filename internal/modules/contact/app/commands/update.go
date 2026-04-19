package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/contact/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type UpdateContactCommand struct {
	ID        string
	FirstName string
	LastName  string
	Email     string
	Phone     string
	Role      string
	Notes     string
}

type UpdateContactHandler struct {
	repo      domain.ContactRepository
	publisher events.EventPublisher
}

func NewUpdateContactHandler(repo domain.ContactRepository, pub events.EventPublisher) *UpdateContactHandler {
	return &UpdateContactHandler{repo: repo, publisher: pub}
}

func (h *UpdateContactHandler) Handle(ctx context.Context, cmd UpdateContactCommand) error {
	c, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := c.Update(cmd.FirstName, cmd.LastName, cmd.Email, cmd.Phone, cmd.Role, cmd.Notes); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, c); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.ContactUpdated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "contact.updated",
		AggregateID: c.ID,
		OccurredAt:  c.UpdatedAt,
	})
	return nil
}
