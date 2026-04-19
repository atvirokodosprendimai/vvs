package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/cron/domain"
)

type AddJobCommand struct {
	Name     string
	Schedule string
	JobType  string
	Payload  string
}

type AddJobHandler struct {
	repo domain.JobRepository
}

func NewAddJobHandler(repo domain.JobRepository) *AddJobHandler {
	return &AddJobHandler{repo: repo}
}

func (h *AddJobHandler) Handle(ctx context.Context, cmd AddJobCommand) (*domain.Job, error) {
	job, err := domain.NewJob(
		uuid.Must(uuid.NewV7()).String(),
		cmd.Name, cmd.Schedule, cmd.JobType, cmd.Payload,
	)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}
