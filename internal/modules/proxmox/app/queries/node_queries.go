package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// ListNodesHandler returns all nodes as read models (no token secrets).
type ListNodesHandler struct {
	repo domain.NodeRepository
}

func NewListNodesHandler(repo domain.NodeRepository) *ListNodesHandler {
	return &ListNodesHandler{repo: repo}
}

func (h *ListNodesHandler) Handle(ctx context.Context) ([]NodeReadModel, error) {
	nodes, err := h.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]NodeReadModel, len(nodes))
	for i, n := range nodes {
		result[i] = nodeToReadModel(n)
	}
	return result, nil
}

// GetNodeHandler returns a single node by ID.
type GetNodeHandler struct {
	repo domain.NodeRepository
}

func NewGetNodeHandler(repo domain.NodeRepository) *GetNodeHandler {
	return &GetNodeHandler{repo: repo}
}

func (h *GetNodeHandler) Handle(ctx context.Context, id string) (*NodeReadModel, error) {
	node, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rm := nodeToReadModel(node)
	return &rm, nil
}
