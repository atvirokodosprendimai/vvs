package commands

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/modules/device/domain"
	"github.com/vvs/isp/internal/shared/events"

	"github.com/google/uuid"
)

type UpdateDeviceCommand struct {
	ID             string
	Name           string
	Notes          string
	Location       string
	PurchasedAt    *time.Time
	WarrantyExpiry *time.Time
}

type UpdateDeviceHandler struct {
	repo      domain.DeviceRepository
	publisher events.EventPublisher
}

func NewUpdateDeviceHandler(repo domain.DeviceRepository, pub events.EventPublisher) *UpdateDeviceHandler {
	return &UpdateDeviceHandler{repo: repo, publisher: pub}
}

func (h *UpdateDeviceHandler) Handle(ctx context.Context, cmd UpdateDeviceCommand) (*domain.Device, error) {
	d, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := d.Update(cmd.Name, cmd.Notes, cmd.Location, cmd.PurchasedAt, cmd.WarrantyExpiry); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, d); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, events.DeviceUpdated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "device.updated",
		AggregateID: d.ID,
		OccurredAt:  d.UpdatedAt,
	})
	return d, nil
}
