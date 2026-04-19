package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/contact/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type AddContactCommand struct {
	CustomerID string
	FirstName  string
	LastName   string
	Email      string
	Phone      string
	Role       string
}

type AddContactHandler struct {
	repo      domain.ContactRepository
	publisher events.EventPublisher
}

func NewAddContactHandler(repo domain.ContactRepository, pub events.EventPublisher) *AddContactHandler {
	return &AddContactHandler{repo: repo, publisher: pub}
}

func (h *AddContactHandler) Handle(ctx context.Context, cmd AddContactCommand) (*domain.Contact, error) {
	c, err := domain.NewContact(
		uuid.Must(uuid.NewV7()).String(),
		cmd.CustomerID, cmd.FirstName, cmd.LastName, cmd.Email, cmd.Phone, cmd.Role,
	)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, c); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, events.ContactAdded.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "contact.added",
		AggregateID: c.ID,
		OccurredAt:  c.CreatedAt,
	})
	return c, nil
}
