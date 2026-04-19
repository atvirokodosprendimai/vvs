package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/service/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// serviceEventPayload is the JSON payload embedded in service transition events.
type serviceEventPayload struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Status     string `json:"status"`
}

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
	data, _ := json.Marshal(serviceEventPayload{ID: svc.ID, CustomerID: svc.CustomerID, Status: svc.Status})
	h.publisher.Publish(ctx, events.ServiceSuspended.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "service.suspended",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(), Data: data,
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
	data, _ := json.Marshal(serviceEventPayload{ID: svc.ID, CustomerID: svc.CustomerID, Status: svc.Status})
	h.publisher.Publish(ctx, events.ServiceReactivated.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "service.reactivated",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(), Data: data,
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
	data, _ := json.Marshal(serviceEventPayload{ID: svc.ID, CustomerID: svc.CustomerID, Status: svc.Status})
	h.publisher.Publish(ctx, events.ServiceCancelled.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "service.cancelled",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(), Data: data,
	})
	return nil
}
