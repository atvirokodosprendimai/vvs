package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/service/domain"
)

type ListServicesForCustomerQuery struct {
	CustomerID string
}

type ListServicesForCustomerHandler struct {
	repo domain.ServiceRepository
}

func NewListServicesForCustomerHandler(repo domain.ServiceRepository) *ListServicesForCustomerHandler {
	return &ListServicesForCustomerHandler{repo: repo}
}

func (h *ListServicesForCustomerHandler) Handle(ctx context.Context, q ListServicesForCustomerQuery) ([]ServiceReadModel, error) {
	svcs, err := h.repo.ListForCustomer(ctx, q.CustomerID)
	if err != nil {
		return nil, err
	}
	out := make([]ServiceReadModel, len(svcs))
	for i, s := range svcs {
		out[i] = ServiceReadModel{
			ID:              s.ID,
			CustomerID:      s.CustomerID,
			ProductID:       s.ProductID,
			ProductName:     s.ProductName,
			PriceAmount:     s.PriceAmount,
			Currency:        s.Currency,
			StartDate:       s.StartDate,
			Status:          s.Status,
			BillingCycle:    s.BillingCycle,
			NextBillingDate: s.NextBillingDate,
			CreatedAt:       s.CreatedAt,
		}
	}
	return out, nil
}
