package commands

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/cron/domain"
)

type PauseJobCommand struct{ ID string }

type PauseJobHandler struct{ repo domain.JobRepository }

func NewPauseJobHandler(repo domain.JobRepository) *PauseJobHandler {
	return &PauseJobHandler{repo: repo}
}

func (h *PauseJobHandler) Handle(ctx context.Context, cmd PauseJobCommand) error {
	job, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := job.Pause(); err != nil {
		return err
	}
	return h.repo.Save(ctx, job)
}
