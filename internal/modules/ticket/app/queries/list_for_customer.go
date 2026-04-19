package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/ticket/domain"
)

type ListTicketsForCustomerQuery struct {
	CustomerID string
}

type ListTicketsForCustomerHandler struct {
	repo domain.TicketRepository
}

func NewListTicketsForCustomerHandler(repo domain.TicketRepository) *ListTicketsForCustomerHandler {
	return &ListTicketsForCustomerHandler{repo: repo}
}

func (h *ListTicketsForCustomerHandler) Handle(ctx context.Context, q ListTicketsForCustomerQuery) ([]TicketReadModel, error) {
	tickets, err := h.repo.ListForCustomer(ctx, q.CustomerID)
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
		out[i] = TicketReadModel{
			ID:         t.ID,
			CustomerID: t.CustomerID,
			Subject:    t.Subject,
			Body:       t.Body,
			Status:     t.Status,
			Priority:   t.Priority,
			AssigneeID: t.AssigneeID,
			Comments:   cms,
			CreatedAt:  t.CreatedAt,
			UpdatedAt:  t.UpdatedAt,
		}
	}
	return out, nil
}
