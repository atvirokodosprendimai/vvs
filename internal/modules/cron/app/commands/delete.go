package commands

import (
	"context"

	"github.com/vvs/isp/internal/modules/cron/domain"
)

type DeleteJobCommand struct{ ID string }

type DeleteJobHandler struct{ repo domain.JobRepository }

func NewDeleteJobHandler(repo domain.JobRepository) *DeleteJobHandler {
	return &DeleteJobHandler{repo: repo}
}

func (h *DeleteJobHandler) Handle(ctx context.Context, cmd DeleteJobCommand) error {
	job, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := job.Delete(); err != nil {
		return err
	}
	return h.repo.Save(ctx, job)
}
