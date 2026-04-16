package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/deal/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type AddDealCommand struct {
	CustomerID string
	Title      string
	Value      int64
	Currency   string
	Notes      string
}

type AddDealHandler struct {
	repo      domain.DealRepository
	publisher events.EventPublisher
}

func NewAddDealHandler(repo domain.DealRepository, pub events.EventPublisher) *AddDealHandler {
	return &AddDealHandler{repo: repo, publisher: pub}
}

func (h *AddDealHandler) Handle(ctx context.Context, cmd AddDealCommand) (*domain.Deal, error) {
	deal, err := domain.NewDeal(
		uuid.Must(uuid.NewV7()).String(),
		cmd.CustomerID, cmd.Title, cmd.Value, cmd.Currency, cmd.Notes,
	)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, deal); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, "isp.deal.added", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "deal.added",
		AggregateID: deal.ID,
		OccurredAt:  deal.CreatedAt,
	})
	return deal, nil
}
