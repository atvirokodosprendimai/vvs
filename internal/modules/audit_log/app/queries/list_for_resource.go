package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/audit_log/domain"
)

type ListForResourceQuery struct {
	Resource   string
	ResourceID string
}

type ListForResourceHandler struct {
	repo domain.AuditLogRepository
}

func NewListForResourceHandler(repo domain.AuditLogRepository) *ListForResourceHandler {
	return &ListForResourceHandler{repo: repo}
}

func (h *ListForResourceHandler) Handle(ctx context.Context, q ListForResourceQuery) ([]*AuditLogReadModel, error) {
	entries, err := h.repo.ListForResource(ctx, q.Resource, q.ResourceID)
	if err != nil {
		return nil, err
	}
	out := make([]*AuditLogReadModel, len(entries))
	for i, e := range entries {
		out[i] = &AuditLogReadModel{
			ID:         e.ID,
			ActorID:    e.ActorID,
			ActorName:  e.ActorName,
			Action:     e.Action,
			Resource:   e.Resource,
			ResourceID: e.ResourceID,
			Changes:    e.Changes,
			CreatedAt:  e.CreatedAt,
		}
	}
	return out, nil
}
