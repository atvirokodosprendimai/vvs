package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

type CreateSubscriptionCommand struct {
	CustomerID string
	PackageID  string
	StartsAt   time.Time
}

type CreateSubscriptionHandler struct {
	subRepo domain.SubscriptionRepository
	keyRepo domain.SubscriptionKeyRepository
	pkgRepo domain.PackageRepository
}

func NewCreateSubscriptionHandler(
	subRepo domain.SubscriptionRepository,
	keyRepo domain.SubscriptionKeyRepository,
	pkgRepo domain.PackageRepository,
) *CreateSubscriptionHandler {
	return &CreateSubscriptionHandler{subRepo: subRepo, keyRepo: keyRepo, pkgRepo: pkgRepo}
}

func (h *CreateSubscriptionHandler) Handle(ctx context.Context, cmd CreateSubscriptionCommand) (*domain.Subscription, error) {
	pkg, err := h.pkgRepo.FindByID(ctx, cmd.PackageID)
	if err != nil {
		return nil, err
	}
	sub, err := domain.NewSubscription(
		uuid.Must(uuid.NewV7()).String(),
		cmd.CustomerID, cmd.PackageID, cmd.StartsAt,
	)
	if err != nil {
		return nil, err
	}
	if err := h.subRepo.Save(ctx, sub); err != nil {
		return nil, err
	}
	// Automatically issue a subscription key on creation.
	key, err := domain.NewSubscriptionKey(
		uuid.Must(uuid.NewV7()).String(),
		sub.ID, cmd.CustomerID, pkg.ID,
	)
	if err != nil {
		return nil, err
	}
	if err := h.keyRepo.Save(ctx, key); err != nil {
		return nil, err
	}
	return sub, nil
}

type SuspendSubscriptionCommand struct{ ID string }

type SuspendSubscriptionHandler struct{ repo domain.SubscriptionRepository }

func NewSuspendSubscriptionHandler(repo domain.SubscriptionRepository) *SuspendSubscriptionHandler {
	return &SuspendSubscriptionHandler{repo: repo}
}

func (h *SuspendSubscriptionHandler) Handle(ctx context.Context, cmd SuspendSubscriptionCommand) error {
	sub, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := sub.Suspend(); err != nil {
		return err
	}
	return h.repo.Save(ctx, sub)
}

type ReactivateSubscriptionCommand struct{ ID string }

type ReactivateSubscriptionHandler struct{ repo domain.SubscriptionRepository }

func NewReactivateSubscriptionHandler(repo domain.SubscriptionRepository) *ReactivateSubscriptionHandler {
	return &ReactivateSubscriptionHandler{repo: repo}
}

func (h *ReactivateSubscriptionHandler) Handle(ctx context.Context, cmd ReactivateSubscriptionCommand) error {
	sub, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := sub.Reactivate(); err != nil {
		return err
	}
	return h.repo.Save(ctx, sub)
}

type UpdateSubscriptionCommand struct {
	ID        string
	PackageID string
}

type UpdateSubscriptionHandler struct{ repo domain.SubscriptionRepository }

func NewUpdateSubscriptionHandler(repo domain.SubscriptionRepository) *UpdateSubscriptionHandler {
	return &UpdateSubscriptionHandler{repo: repo}
}

func (h *UpdateSubscriptionHandler) Handle(ctx context.Context, cmd UpdateSubscriptionCommand) error {
	sub, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	sub.PackageID = cmd.PackageID
	sub.UpdatedAt = time.Now().UTC()
	return h.repo.Save(ctx, sub)
}

type CancelSubscriptionCommand struct{ ID string }

type CancelSubscriptionHandler struct{ repo domain.SubscriptionRepository }

func NewCancelSubscriptionHandler(repo domain.SubscriptionRepository) *CancelSubscriptionHandler {
	return &CancelSubscriptionHandler{repo: repo}
}

func (h *CancelSubscriptionHandler) Handle(ctx context.Context, cmd CancelSubscriptionCommand) error {
	sub, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := sub.Cancel(); err != nil {
		return err
	}
	return h.repo.Save(ctx, sub)
}
