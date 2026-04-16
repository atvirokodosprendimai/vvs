package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/ticket/domain"
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
		out[i] = TicketReadModel{
			ID:         t.ID,
			CustomerID: t.CustomerID,
			Subject:    t.Subject,
			Body:       t.Body,
			Status:     t.Status,
			Priority:   t.Priority,
			AssigneeID: t.AssigneeID,
			CreatedAt:  t.CreatedAt,
			UpdatedAt:  t.UpdatedAt,
		}
	}
	return out, nil
}
