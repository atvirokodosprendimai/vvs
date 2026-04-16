package commands

import (
	"context"

	"github.com/vvs/isp/internal/modules/cron/domain"
)

type ResumeJobCommand struct{ ID string }

type ResumeJobHandler struct{ repo domain.JobRepository }

func NewResumeJobHandler(repo domain.JobRepository) *ResumeJobHandler {
	return &ResumeJobHandler{repo: repo}
}

func (h *ResumeJobHandler) Handle(ctx context.Context, cmd ResumeJobCommand) error {
	job, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := job.Resume(); err != nil {
		return err
	}
	return h.repo.Save(ctx, job)
}
