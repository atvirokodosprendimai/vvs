package commands

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/cron/domain"
)

type UpdateJobCommand struct {
	ID       string
	Name     string
	Schedule string
	JobType  string
	Payload  string
}

type UpdateJobHandler struct {
	repo domain.JobRepository
}

func NewUpdateJobHandler(repo domain.JobRepository) *UpdateJobHandler {
	return &UpdateJobHandler{repo: repo}
}

func (h *UpdateJobHandler) Handle(ctx context.Context, cmd UpdateJobCommand) error {
	job, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := job.Update(cmd.Name, cmd.Schedule, cmd.JobType, cmd.Payload); err != nil {
		return err
	}
	return h.repo.Save(ctx, job)
}
