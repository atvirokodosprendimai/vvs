package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/deal/domain"
)

type ListAllDealsQuery struct{}

type ListAllDealsHandler struct {
	repo domain.DealRepository
}

func NewListAllDealsHandler(repo domain.DealRepository) *ListAllDealsHandler {
	return &ListAllDealsHandler{repo: repo}
}

func (h *ListAllDealsHandler) Handle(ctx context.Context, _ ListAllDealsQuery) ([]DealReadModel, error) {
	deals, err := h.repo.ListAll(ctx)
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
