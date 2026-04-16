package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/deal/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type UpdateDealCommand struct {
	ID       string
	Title    string
	Value    int64
	Currency string
	Notes    string
}

type UpdateDealHandler struct {
	repo      domain.DealRepository
	publisher events.EventPublisher
}

func NewUpdateDealHandler(repo domain.DealRepository, pub events.EventPublisher) *UpdateDealHandler {
	return &UpdateDealHandler{repo: repo, publisher: pub}
}

func (h *UpdateDealHandler) Handle(ctx context.Context, cmd UpdateDealCommand) error {
	deal, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := deal.Update(cmd.Title, cmd.Value, cmd.Currency, cmd.Notes); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, deal); err != nil {
		return err
	}
	h.publisher.Publish(ctx, "isp.deal.updated", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "deal.updated",
		AggregateID: deal.ID,
		OccurredAt:  time.Now().UTC(),
	})
	return nil
}
