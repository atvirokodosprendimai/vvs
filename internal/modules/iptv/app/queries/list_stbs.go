package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

type ListSTBsHandler struct{ repo domain.STBRepository }

func NewListSTBsHandler(repo domain.STBRepository) *ListSTBsHandler {
	return &ListSTBsHandler{repo: repo}
}

func (h *ListSTBsHandler) Handle(ctx context.Context) ([]STBReadModel, error) {
	stbs, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]STBReadModel, len(stbs))
	for i, s := range stbs {
		out[i] = STBReadModel{
			ID:         s.ID,
			MAC:        s.MAC,
			Model:      s.Model,
			CustomerID: s.CustomerID,
			AssignedAt: s.AssignedAt,
			Notes:      s.Notes,
		}
	}
	return out, nil
}
