package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/device/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type DecommissionDeviceCommand struct {
	ID string
}

type DecommissionDeviceHandler struct {
	repo      domain.DeviceRepository
	publisher events.EventPublisher
}

func NewDecommissionDeviceHandler(repo domain.DeviceRepository, pub events.EventPublisher) *DecommissionDeviceHandler {
	return &DecommissionDeviceHandler{repo: repo, publisher: pub}
}

func (h *DecommissionDeviceHandler) Handle(ctx context.Context, cmd DecommissionDeviceCommand) error {
	d, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := d.Decommission(); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, d); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.DeviceDecommissioned.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "device.decommissioned",
		AggregateID: d.ID,
		OccurredAt:  d.UpdatedAt,
	})
	return nil
}
