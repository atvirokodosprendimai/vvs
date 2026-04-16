package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/deal/domain"
)

type ListDealsForCustomerQuery struct {
	CustomerID string
}

type ListDealsForCustomerHandler struct {
	repo domain.DealRepository
}

func NewListDealsForCustomerHandler(repo domain.DealRepository) *ListDealsForCustomerHandler {
	return &ListDealsForCustomerHandler{repo: repo}
}

func (h *ListDealsForCustomerHandler) Handle(ctx context.Context, q ListDealsForCustomerQuery) ([]DealReadModel, error) {
	deals, err := h.repo.ListForCustomer(ctx, q.CustomerID)
	if err != nil {
		return nil, err
	}
	out := make([]DealReadModel, len(deals))
	for i, d := range deals {
		out[i] = DealReadModel{
			ID:         d.ID,
			CustomerID: d.CustomerID,
			Title:      d.Title,
			Value:      d.Value,
			Currency:   d.Currency,
			Stage:      d.Stage,
			Notes:      d.Notes,
			CreatedAt:  d.CreatedAt,
			UpdatedAt:  d.UpdatedAt,
		}
	}
	return out, nil
}
