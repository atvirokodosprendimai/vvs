package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type DeleteNodeCommand struct {
	ID string
}

type DeleteNodeHandler struct {
	nodeRepo  domain.NodeRepository
	vmRepo    domain.VMRepository
	publisher events.EventPublisher
}

func NewDeleteNodeHandler(nodeRepo domain.NodeRepository, vmRepo domain.VMRepository, pub events.EventPublisher) *DeleteNodeHandler {
	return &DeleteNodeHandler{nodeRepo: nodeRepo, vmRepo: vmRepo, publisher: pub}
}

func (h *DeleteNodeHandler) Handle(ctx context.Context, cmd DeleteNodeCommand) error {
	// Guard: refuse if any VMs still exist on this node.
	vms, err := h.vmRepo.FindByNodeID(ctx, cmd.ID)
	if err != nil {
		return fmt.Errorf("check node VMs: %w", err)
	}
	if len(vms) > 0 {
		return domain.ErrNodeHasVMs
	}

	if err := h.nodeRepo.Delete(ctx, cmd.ID); err != nil {
		return err
	}

	data, _ := json.Marshal(map[string]string{"id": cmd.ID})
	h.publisher.Publish(ctx, events.ProxmoxNodeDeleted.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "proxmox.node.deleted",
		AggregateID: cmd.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return nil
}
