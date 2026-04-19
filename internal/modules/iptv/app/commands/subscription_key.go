package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

type RevokeSubscriptionKeyCommand struct{ ID string }

type RevokeSubscriptionKeyHandler struct{ repo domain.SubscriptionKeyRepository }

func NewRevokeSubscriptionKeyHandler(repo domain.SubscriptionKeyRepository) *RevokeSubscriptionKeyHandler {
	return &RevokeSubscriptionKeyHandler{repo: repo}
}

func (h *RevokeSubscriptionKeyHandler) Handle(ctx context.Context, cmd RevokeSubscriptionKeyCommand) (*domain.SubscriptionKey, error) {
	key, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	key.Revoke()
	if err := h.repo.Save(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

type ReissueSubscriptionKeyCommand struct {
	SubscriptionID string
	CustomerID     string
	PackageID      string
}

type ReissueSubscriptionKeyHandler struct{ repo domain.SubscriptionKeyRepository }

func NewReissueSubscriptionKeyHandler(repo domain.SubscriptionKeyRepository) *ReissueSubscriptionKeyHandler {
	return &ReissueSubscriptionKeyHandler{repo: repo}
}

func (h *ReissueSubscriptionKeyHandler) Handle(ctx context.Context, cmd ReissueSubscriptionKeyCommand) (*domain.SubscriptionKey, error) {
	// Revoke all existing active keys for this subscription.
	keys, err := h.repo.FindBySubscriptionID(ctx, cmd.SubscriptionID)
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k.IsActive() {
			k.Revoke()
			if err := h.repo.Save(ctx, k); err != nil {
				return nil, err
			}
		}
	}
	// Issue new key.
	newKey, err := domain.NewSubscriptionKey(
		uuid.Must(uuid.NewV7()).String(),
		cmd.SubscriptionID, cmd.CustomerID, cmd.PackageID,
	)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, newKey); err != nil {
		return nil, err
	}
	return newKey, nil
}
