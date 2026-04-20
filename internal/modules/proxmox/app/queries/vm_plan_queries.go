package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// ListVMPlansHandler returns all VM plans.
type ListVMPlansHandler struct {
	planRepo domain.VMPlanRepository
}

func NewListVMPlansHandler(planRepo domain.VMPlanRepository) *ListVMPlansHandler {
	return &ListVMPlansHandler{planRepo: planRepo}
}

func (h *ListVMPlansHandler) Handle(ctx context.Context) ([]VMPlanReadModel, error) {
	plans, err := h.planRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]VMPlanReadModel, len(plans))
	for i, p := range plans {
		result[i] = vmPlanToReadModel(p)
	}
	return result, nil
}

// GetVMPlanHandler returns a single VM plan by ID.
type GetVMPlanHandler struct {
	planRepo domain.VMPlanRepository
}

func NewGetVMPlanHandler(planRepo domain.VMPlanRepository) *GetVMPlanHandler {
	return &GetVMPlanHandler{planRepo: planRepo}
}

func (h *GetVMPlanHandler) Handle(ctx context.Context, id string) (*VMPlanReadModel, error) {
	plan, err := h.planRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rm := vmPlanToReadModel(plan)
	return &rm, nil
}
