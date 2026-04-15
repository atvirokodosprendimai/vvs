package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/service/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type AssignServiceCommand struct {
	CustomerID  string
	ProductID   string
	ProductName string
	PriceAmount int64
	Currency    string
	StartDate   time.Time
}

type AssignServiceHandler struct {
	repo      domain.ServiceRepository
	publisher events.EventPublisher
}

func NewAssignServiceHandler(repo domain.ServiceRepository, pub events.EventPublisher) *AssignServiceHandler {
	return &AssignServiceHandler{repo: repo, publisher: pub}
}

func (h *AssignServiceHandler) Handle(ctx context.Context, cmd AssignServiceCommand) (*domain.Service, error) {
	svc, err := domain.NewService(
		uuid.Must(uuid.NewV7()).String(),
		cmd.CustomerID, cmd.ProductID, cmd.ProductName,
		cmd.PriceAmount, cmd.Currency, cmd.StartDate,
	)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, svc); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, "isp.service.assigned", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "service.assigned",
		AggregateID: svc.ID,
		OccurredAt:  svc.CreatedAt,
	})
	return svc, nil
}
