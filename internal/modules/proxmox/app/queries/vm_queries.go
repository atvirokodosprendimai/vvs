package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// nodeNameResolver enriches VM read models with node names.
// Uses an in-memory node cache to avoid N+1 queries.
type nodeNameResolver struct {
	nodeRepo domain.NodeRepository
	cache    map[string]string // nodeID → nodeName
}

func newResolver(nodeRepo domain.NodeRepository) *nodeNameResolver {
	return &nodeNameResolver{nodeRepo: nodeRepo, cache: make(map[string]string)}
}

func (r *nodeNameResolver) resolve(ctx context.Context, vm *domain.VirtualMachine) VMReadModel {
	rm := vmToReadModel(vm)
	if name, ok := r.cache[vm.NodeID]; ok {
		rm.NodeName = name
		return rm
	}
	if node, err := r.nodeRepo.FindByID(ctx, vm.NodeID); err == nil {
		r.cache[vm.NodeID] = node.Name
		rm.NodeName = node.Name
	}
	return rm
}

// ListVMsHandler returns all VMs with node names.
type ListVMsHandler struct {
	vmRepo   domain.VMRepository
	nodeRepo domain.NodeRepository
}

func NewListVMsHandler(vmRepo domain.VMRepository, nodeRepo domain.NodeRepository) *ListVMsHandler {
	return &ListVMsHandler{vmRepo: vmRepo, nodeRepo: nodeRepo}
}

func (h *ListVMsHandler) Handle(ctx context.Context) ([]VMReadModel, error) {
	vms, err := h.vmRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return h.enrich(ctx, vms), nil
}

// ListVMsForCustomerHandler returns VMs assigned to a specific customer.
type ListVMsForCustomerHandler struct {
	vmRepo   domain.VMRepository
	nodeRepo domain.NodeRepository
}

func NewListVMsForCustomerHandler(vmRepo domain.VMRepository, nodeRepo domain.NodeRepository) *ListVMsForCustomerHandler {
	return &ListVMsForCustomerHandler{vmRepo: vmRepo, nodeRepo: nodeRepo}
}

func (h *ListVMsForCustomerHandler) Handle(ctx context.Context, customerID string) ([]VMReadModel, error) {
	vms, err := h.vmRepo.FindByCustomerID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	lister := &ListVMsHandler{vmRepo: h.vmRepo, nodeRepo: h.nodeRepo}
	return lister.enrich(ctx, vms), nil
}

// GetVMHandler returns a single VM by internal ID.
type GetVMHandler struct {
	vmRepo   domain.VMRepository
	nodeRepo domain.NodeRepository
}

func NewGetVMHandler(vmRepo domain.VMRepository, nodeRepo domain.NodeRepository) *GetVMHandler {
	return &GetVMHandler{vmRepo: vmRepo, nodeRepo: nodeRepo}
}

func (h *GetVMHandler) Handle(ctx context.Context, id string) (*VMReadModel, error) {
	vm, err := h.vmRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resolver := newResolver(h.nodeRepo)
	rm := resolver.resolve(ctx, vm)
	return &rm, nil
}

func (h *ListVMsHandler) enrich(ctx context.Context, vms []*domain.VirtualMachine) []VMReadModel {
	resolver := newResolver(h.nodeRepo)
	result := make([]VMReadModel, len(vms))
	for i, vm := range vms {
		result[i] = resolver.resolve(ctx, vm)
	}
	return result
}
