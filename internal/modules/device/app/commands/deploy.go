package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/device/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type DeployDeviceCommand struct {
	ID         string
	CustomerID string
	Location   string
}

type DeployDeviceHandler struct {
	repo      domain.DeviceRepository
	publisher events.EventPublisher
}

func NewDeployDeviceHandler(repo domain.DeviceRepository, pub events.EventPublisher) *DeployDeviceHandler {
	return &DeployDeviceHandler{repo: repo, publisher: pub}
}

func (h *DeployDeviceHandler) Handle(ctx context.Context, cmd DeployDeviceCommand) (*domain.Device, error) {
	d, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := d.Deploy(cmd.CustomerID, cmd.Location); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, d); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, events.DeviceDeployed.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "device.deployed",
		AggregateID: d.ID,
		OccurredAt:  d.UpdatedAt,
	})
	return d, nil
}
