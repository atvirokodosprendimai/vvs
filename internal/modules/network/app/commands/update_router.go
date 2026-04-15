package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/network/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type UpdateRouterCommand struct {
	ID         string
	Name       string
	RouterType string
	Host       string
	Port       int
	Username   string
	Password   string // empty = keep existing
	Notes      string
}

type UpdateRouterHandler struct {
	repo      domain.RouterRepository
	publisher events.EventPublisher
}

func NewUpdateRouterHandler(repo domain.RouterRepository, pub events.EventPublisher) *UpdateRouterHandler {
	return &UpdateRouterHandler{repo: repo, publisher: pub}
}

func (h *UpdateRouterHandler) Handle(ctx context.Context, cmd UpdateRouterCommand) (*domain.Router, error) {
	router, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if err := router.Update(cmd.Name, cmd.RouterType, cmd.Host, cmd.Port, cmd.Username, cmd.Password, cmd.Notes); err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, router); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(toReadModel(router))
	h.publisher.Publish(ctx, "isp.network.router.updated", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "network.router.updated",
		AggregateID: router.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return router, nil
}
