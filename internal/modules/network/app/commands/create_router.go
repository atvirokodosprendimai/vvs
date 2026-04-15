package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/network/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type CreateRouterCommand struct {
	Name       string
	RouterType string
	Host       string
	Port       int
	Username   string
	Password   string
	Notes      string
}

type CreateRouterHandler struct {
	repo      domain.RouterRepository
	publisher events.EventPublisher
}

func NewCreateRouterHandler(repo domain.RouterRepository, pub events.EventPublisher) *CreateRouterHandler {
	return &CreateRouterHandler{repo: repo, publisher: pub}
}

func (h *CreateRouterHandler) Handle(ctx context.Context, cmd CreateRouterCommand) (*domain.Router, error) {
	router, err := domain.NewRouter(cmd.Name, cmd.RouterType, cmd.Host, cmd.Port, cmd.Username, cmd.Password, cmd.Notes)
	if err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, router); err != nil {
		return nil, err
	}

	data, _ := json.Marshal(toReadModel(router))
	h.publisher.Publish(ctx, "isp.network.router.created", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "network.router.created",
		AggregateID: router.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return router, nil
}
