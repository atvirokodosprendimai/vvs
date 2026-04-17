package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/ticket/domain"
)

// customerNameResolver is a local interface for resolving customer names.
type customerNameResolver interface {
	CustomerName(ctx context.Context, id string) string
}

type ListAllTicketsHandler struct {
	repo     domain.TicketRepository
	resolver customerNameResolver
}

func NewListAllTicketsHandler(repo domain.TicketRepository, resolver customerNameResolver) *ListAllTicketsHandler {
	return &ListAllTicketsHandler{repo: repo, resolver: resolver}
}

func (h *ListAllTicketsHandler) Handle(ctx context.Context) ([]TicketReadModel, error) {
	tickets, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TicketReadModel, len(tickets))
	for i, t := range tickets {
		comments, _ := h.repo.ListComments(ctx, t.ID)
		cms := make([]CommentReadModel, len(comments))
		for j, c := range comments {
			cms[j] = CommentReadModel{
				ID:        c.ID,
				TicketID:  c.TicketID,
				Body:      c.Body,
				AuthorID:  c.AuthorID,
				CreatedAt: c.CreatedAt,
			}
		}
		name := ""
		if h.resolver != nil {
			name = h.resolver.CustomerName(ctx, t.CustomerID)
		}
		out[i] = TicketReadModel{
			ID:           t.ID,
			CustomerID:   t.CustomerID,
			CustomerName: name,
			Subject:      t.Subject,
			Body:         t.Body,
			Status:       t.Status,
			Priority:     t.Priority,
			AssigneeID:   t.AssigneeID,
			Comments:     cms,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
		}
	}
	return out, nil
}
