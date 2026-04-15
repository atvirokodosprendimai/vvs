package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/service/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// SuspendServiceHandler

type SuspendServiceCommand struct{ ID string }

type SuspendServiceHandler struct {
	repo      domain.ServiceRepository
	publisher events.EventPublisher
}

func NewSuspendServiceHandler(repo domain.ServiceRepository, pub events.EventPublisher) *SuspendServiceHandler {
	return &SuspendServiceHandler{repo: repo, publisher: pub}
}

func (h *SuspendServiceHandler) Handle(ctx context.Context, cmd SuspendServiceCommand) error {
	svc, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := svc.Suspend(); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, svc); err != nil {
		return err
	}
	h.publisher.Publish(ctx, "isp.service.suspended", events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "service.suspended",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(),
	})
	return nil
}

// ReactivateServiceHandler

type ReactivateServiceCommand struct{ ID string }

type ReactivateServiceHandler struct {
	repo      domain.ServiceRepository
	publisher events.EventPublisher
}

func NewReactivateServiceHandler(repo domain.ServiceRepository, pub events.EventPublisher) *ReactivateServiceHandler {
	return &ReactivateServiceHandler{repo: repo, publisher: pub}
}

func (h *ReactivateServiceHandler) Handle(ctx context.Context, cmd ReactivateServiceCommand) error {
	svc, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := svc.Reactivate(); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, svc); err != nil {
		return err
	}
	h.publisher.Publish(ctx, "isp.service.reactivated", events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "service.reactivated",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(),
	})
	return nil
}

// CancelServiceHandler

type CancelServiceCommand struct{ ID string }

type CancelServiceHandler struct {
	repo      domain.ServiceRepository
	publisher events.EventPublisher
}

func NewCancelServiceHandler(repo domain.ServiceRepository, pub events.EventPublisher) *CancelServiceHandler {
	return &CancelServiceHandler{repo: repo, publisher: pub}
}

func (h *CancelServiceHandler) Handle(ctx context.Context, cmd CancelServiceCommand) error {
	svc, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := svc.Cancel(); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, svc); err != nil {
		return err
	}
	h.publisher.Publish(ctx, "isp.service.cancelled", events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "service.cancelled",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(),
	})
	return nil
}
