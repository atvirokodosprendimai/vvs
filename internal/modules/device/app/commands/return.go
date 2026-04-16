package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/device/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type ReturnDeviceCommand struct {
	ID string
}

type ReturnDeviceHandler struct {
	repo      domain.DeviceRepository
	publisher events.EventPublisher
}

func NewReturnDeviceHandler(repo domain.DeviceRepository, pub events.EventPublisher) *ReturnDeviceHandler {
	return &ReturnDeviceHandler{repo: repo, publisher: pub}
}

func (h *ReturnDeviceHandler) Handle(ctx context.Context, cmd ReturnDeviceCommand) error {
	d, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := d.Return(); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, d); err != nil {
		return err
	}
	h.publisher.Publish(ctx, "isp.device.returned", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "device.returned",
		AggregateID: d.ID,
		OccurredAt:  d.UpdatedAt,
	})
	return nil
}
