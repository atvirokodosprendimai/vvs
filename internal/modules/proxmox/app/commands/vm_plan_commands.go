package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// ── CreateVMPlan ──────────────────────────────────────────────────────────────

type CreateVMPlanCommand struct {
	Name                  string
	Description           string
	Cores                 int
	MemoryMB              int
	DiskGB                int
	Storage               string
	TemplateVMID          int
	NodeID                string
	PriceMonthlyEuroCents int64
	Notes                 string
}

type CreateVMPlanHandler struct {
	planRepo  domain.VMPlanRepository
	publisher events.EventPublisher
}

func NewCreateVMPlanHandler(planRepo domain.VMPlanRepository, pub events.EventPublisher) *CreateVMPlanHandler {
	return &CreateVMPlanHandler{planRepo: planRepo, publisher: pub}
}

func (h *CreateVMPlanHandler) Handle(ctx context.Context, cmd CreateVMPlanCommand) (*domain.VMPlan, error) {
	plan, err := domain.NewVMPlan(
		cmd.Name, cmd.Description,
		cmd.Cores, cmd.MemoryMB, cmd.DiskGB, cmd.TemplateVMID,
		cmd.Storage, cmd.NodeID,
		cmd.PriceMonthlyEuroCents, cmd.Notes,
	)
	if err != nil {
		return nil, err
	}
	if err := h.planRepo.Save(ctx, plan); err != nil {
		return nil, err
	}
	data, _ := json.Marshal(map[string]any{"id": plan.ID, "name": plan.Name})
	h.publisher.Publish(ctx, events.ProxmoxVMPlanCreated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "proxmox.vmplan.created",
		AggregateID: plan.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return plan, nil
}

// ── UpdateVMPlan ──────────────────────────────────────────────────────────────

type UpdateVMPlanCommand struct {
	ID                    string
	Name                  string
	Description           string
	Cores                 int
	MemoryMB              int
	DiskGB                int
	Storage               string
	TemplateVMID          int
	NodeID                string
	PriceMonthlyEuroCents int64
	Enabled               bool
	Notes                 string
}

type UpdateVMPlanHandler struct {
	planRepo  domain.VMPlanRepository
	publisher events.EventPublisher
}

func NewUpdateVMPlanHandler(planRepo domain.VMPlanRepository, pub events.EventPublisher) *UpdateVMPlanHandler {
	return &UpdateVMPlanHandler{planRepo: planRepo, publisher: pub}
}

func (h *UpdateVMPlanHandler) Handle(ctx context.Context, cmd UpdateVMPlanCommand) (*domain.VMPlan, error) {
	plan, err := h.planRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := plan.Update(
		cmd.Name, cmd.Description,
		cmd.Cores, cmd.MemoryMB, cmd.DiskGB, cmd.TemplateVMID,
		cmd.Storage, cmd.NodeID,
		cmd.PriceMonthlyEuroCents, cmd.Enabled, cmd.Notes,
	); err != nil {
		return nil, err
	}
	if err := h.planRepo.Save(ctx, plan); err != nil {
		return nil, err
	}
	data, _ := json.Marshal(map[string]any{"id": plan.ID, "name": plan.Name})
	h.publisher.Publish(ctx, events.ProxmoxVMPlanUpdated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "proxmox.vmplan.updated",
		AggregateID: plan.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return plan, nil
}

// ── DeleteVMPlan ──────────────────────────────────────────────────────────────

type DeleteVMPlanCommand struct {
	ID string
}

type DeleteVMPlanHandler struct {
	planRepo  domain.VMPlanRepository
	publisher events.EventPublisher
}

func NewDeleteVMPlanHandler(planRepo domain.VMPlanRepository, pub events.EventPublisher) *DeleteVMPlanHandler {
	return &DeleteVMPlanHandler{planRepo: planRepo, publisher: pub}
}

func (h *DeleteVMPlanHandler) Handle(ctx context.Context, cmd DeleteVMPlanCommand) error {
	if _, err := h.planRepo.FindByID(ctx, cmd.ID); err != nil {
		return err
	}
	if err := h.planRepo.Delete(ctx, cmd.ID); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]string{"id": cmd.ID})
	h.publisher.Publish(ctx, events.ProxmoxVMPlanDeleted.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "proxmox.vmplan.deleted",
		AggregateID: cmd.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return nil
}
