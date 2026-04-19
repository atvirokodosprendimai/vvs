package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/audit_log/domain"
)

type ListAuditLogsQuery struct {
	ActorID  string
	Resource string
}

type ListAuditLogsHandler struct {
	repo domain.AuditLogRepository
}

func NewListAuditLogsHandler(repo domain.AuditLogRepository) *ListAuditLogsHandler {
	return &ListAuditLogsHandler{repo: repo}
}

func (h *ListAuditLogsHandler) Handle(ctx context.Context, q ListAuditLogsQuery) ([]*AuditLogReadModel, error) {
	filter := domain.Filter{
		ActorID:  q.ActorID,
		Resource: q.Resource,
	}
	entries, err := h.repo.ListAll(ctx, filter)
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
