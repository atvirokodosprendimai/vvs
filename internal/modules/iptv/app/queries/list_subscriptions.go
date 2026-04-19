package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/iptv/domain"
)

type ListSubscriptionsHandler struct {
	subRepo domain.SubscriptionRepository
	pkgRepo domain.PackageRepository
}

func NewListSubscriptionsHandler(subRepo domain.SubscriptionRepository, pkgRepo domain.PackageRepository) *ListSubscriptionsHandler {
	return &ListSubscriptionsHandler{subRepo: subRepo, pkgRepo: pkgRepo}
}

func (h *ListSubscriptionsHandler) Handle(ctx context.Context) ([]SubscriptionReadModel, error) {
	subs, err := h.subRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	// Build package name lookup.
	pkgNames := make(map[string]string)
	out := make([]SubscriptionReadModel, len(subs))
	for i, s := range subs {
		if _, ok := pkgNames[s.PackageID]; !ok {
			if pkg, err := h.pkgRepo.FindByID(ctx, s.PackageID); err == nil {
				pkgNames[s.PackageID] = pkg.Name
			}
		}
		out[i] = SubscriptionReadModel{
			ID:          s.ID,
			CustomerID:  s.CustomerID,
			PackageID:   s.PackageID,
			PackageName: pkgNames[s.PackageID],
			Status:      s.Status,
			StartsAt:    s.StartsAt,
			EndsAt:      s.EndsAt,
			CreatedAt:   s.CreatedAt,
		}
	}
	return out, nil
}

type ListSubscriptionsForCustomerHandler struct {
	subRepo domain.SubscriptionRepository
	pkgRepo domain.PackageRepository
	keyRepo domain.SubscriptionKeyRepository
}

func NewListSubscriptionsForCustomerHandler(
	subRepo domain.SubscriptionRepository,
	pkgRepo domain.PackageRepository,
	keyRepo domain.SubscriptionKeyRepository,
) *ListSubscriptionsForCustomerHandler {
	return &ListSubscriptionsForCustomerHandler{subRepo: subRepo, pkgRepo: pkgRepo, keyRepo: keyRepo}
}

func (h *ListSubscriptionsForCustomerHandler) Handle(ctx context.Context, customerID string) ([]SubscriptionReadModel, error) {
	subs, err := h.subRepo.ListForCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	pkgNames := make(map[string]string)
	out := make([]SubscriptionReadModel, len(subs))
	for i, s := range subs {
		if _, ok := pkgNames[s.PackageID]; !ok {
			if pkg, err := h.pkgRepo.FindByID(ctx, s.PackageID); err == nil {
				pkgNames[s.PackageID] = pkg.Name
			}
		}
		out[i] = SubscriptionReadModel{
			ID:          s.ID,
			CustomerID:  s.CustomerID,
			PackageID:   s.PackageID,
			PackageName: pkgNames[s.PackageID],
			Status:      s.Status,
			StartsAt:    s.StartsAt,
			EndsAt:      s.EndsAt,
			CreatedAt:   s.CreatedAt,
		}
	}
	return out, nil
}
