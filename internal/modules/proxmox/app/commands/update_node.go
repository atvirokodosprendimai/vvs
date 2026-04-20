package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type UpdateNodeCommand struct {
	ID          string
	Name        string
	NodeName    string
	Host        string
	Port        int
	User        string
	TokenID     string
	TokenSecret string // empty = preserve existing
	InsecureTLS bool
	Notes       string
}

type UpdateNodeHandler struct {
	nodeRepo  domain.NodeRepository
	publisher events.EventPublisher
}

func NewUpdateNodeHandler(nodeRepo domain.NodeRepository, pub events.EventPublisher) *UpdateNodeHandler {
	return &UpdateNodeHandler{nodeRepo: nodeRepo, publisher: pub}
}

func (h *UpdateNodeHandler) Handle(ctx context.Context, cmd UpdateNodeCommand) (*domain.ProxmoxNode, error) {
	node, err := h.nodeRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := node.Update(cmd.Name, cmd.NodeName, cmd.Host, cmd.Port, cmd.User, cmd.TokenID, cmd.TokenSecret, cmd.Notes, cmd.InsecureTLS); err != nil {
		return nil, err
	}
	if err := h.nodeRepo.Save(ctx, node); err != nil {
		return nil, err
	}
	data, _ := json.Marshal(map[string]string{"id": node.ID, "name": node.Name})
	h.publisher.Publish(ctx, events.ProxmoxNodeUpdated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "proxmox.node.updated",
		AggregateID: node.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return node, nil
}
