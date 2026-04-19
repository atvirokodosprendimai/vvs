package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/network/domain"
)

type GetRouterHandler struct {
	repo domain.RouterRepository
}

func NewGetRouterHandler(repo domain.RouterRepository) *GetRouterHandler {
	return &GetRouterHandler{repo: repo}
}

func (h *GetRouterHandler) Handle(ctx context.Context, id string) (RouterReadModel, error) {
	r, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return RouterReadModel{}, err
	}
	return domainToReadModel(r), nil
}
