package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/cron/domain"
)

type GetJobHandler struct{ repo domain.JobRepository }

func NewGetJobHandler(repo domain.JobRepository) *GetJobHandler {
	return &GetJobHandler{repo: repo}
}

func (h *GetJobHandler) Handle(ctx context.Context, id string) (*JobReadModel, error) {
	j, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m := toReadModel(j)
	return &m, nil
}
