package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type DeleteAccountHandler struct {
	repo      domain.EmailAccountRepository
	publisher events.EventPublisher
}

func NewDeleteAccountHandler(repo domain.EmailAccountRepository, pub events.EventPublisher) *DeleteAccountHandler {
	return &DeleteAccountHandler{repo: repo, publisher: pub}
}

func (h *DeleteAccountHandler) Handle(ctx context.Context, id string) error {
	if err := h.repo.Delete(ctx, id); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.EmailAccountDeleted.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.account_deleted", AggregateID: id,
	})
	return nil
}
