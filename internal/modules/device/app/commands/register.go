package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/device/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type RegisterDeviceCommand struct {
	Name         string
	DeviceType   string
	SerialNumber string
	Notes        string
}

type RegisterDeviceHandler struct {
	repo      domain.DeviceRepository
	publisher events.EventPublisher
}

func NewRegisterDeviceHandler(repo domain.DeviceRepository, pub events.EventPublisher) *RegisterDeviceHandler {
	return &RegisterDeviceHandler{repo: repo, publisher: pub}
}

func (h *RegisterDeviceHandler) Handle(ctx context.Context, cmd RegisterDeviceCommand) (*domain.Device, error) {
	d, err := domain.NewDevice(uuid.Must(uuid.NewV7()).String(), cmd.Name, cmd.DeviceType, cmd.SerialNumber)
	if err != nil {
		return nil, err
	}
	d.Notes = cmd.Notes
	if err := h.repo.Save(ctx, d); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, events.DeviceRegistered.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "device.registered",
		AggregateID: d.ID,
		OccurredAt:  d.CreatedAt,
	})
	return d, nil
}
