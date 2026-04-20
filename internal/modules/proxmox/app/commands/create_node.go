package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type CreateNodeCommand struct {
	Name        string
	NodeName    string
	Host        string
	Port        int
	User        string
	TokenID     string
	TokenSecret string
	InsecureTLS bool
	Notes       string
}

type CreateNodeHandler struct {
	nodeRepo  domain.NodeRepository
	publisher events.EventPublisher
}

func NewCreateNodeHandler(nodeRepo domain.NodeRepository, pub events.EventPublisher) *CreateNodeHandler {
	return &CreateNodeHandler{nodeRepo: nodeRepo, publisher: pub}
}

func (h *CreateNodeHandler) Handle(ctx context.Context, cmd CreateNodeCommand) (*domain.ProxmoxNode, error) {
	node, err := domain.NewProxmoxNode(cmd.Name, cmd.NodeName, cmd.Host, cmd.Port, cmd.User, cmd.TokenID, cmd.TokenSecret, cmd.Notes, cmd.InsecureTLS)
	if err != nil {
		return nil, err
	}
	if err := h.nodeRepo.Save(ctx, node); err != nil {
		return nil, err
	}
	data, _ := json.Marshal(map[string]string{"id": node.ID, "name": node.Name, "host": node.Host})
	h.publisher.Publish(ctx, events.ProxmoxNodeCreated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "proxmox.node.created",
		AggregateID: node.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return node, nil
}
