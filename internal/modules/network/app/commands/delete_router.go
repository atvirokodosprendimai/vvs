package commands

import (
	"context"

	"github.com/vvs/isp/internal/modules/network/domain"
)

type DeleteRouterHandler struct {
	repo domain.RouterRepository
}

func NewDeleteRouterHandler(repo domain.RouterRepository) *DeleteRouterHandler {
	return &DeleteRouterHandler{repo: repo}
}

func (h *DeleteRouterHandler) Handle(ctx context.Context, id string) error {
	if _, err := h.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return h.repo.Delete(ctx, id)
}
