package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/ticket/domain"
)

type GetTicketQuery struct {
	ID string
}

type GetTicketHandler struct {
	repo     domain.TicketRepository
	resolver customerNameResolver
}

func NewGetTicketHandler(repo domain.TicketRepository, resolver customerNameResolver) *GetTicketHandler {
	return &GetTicketHandler{repo: repo, resolver: resolver}
}

func (h *GetTicketHandler) Handle(ctx context.Context, q GetTicketQuery) (*TicketReadModel, error) {
	t, err := h.repo.FindByID(ctx, q.ID)
	if err != nil {
		return nil, err
	}
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
	return &TicketReadModel{
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
	}, nil
}
