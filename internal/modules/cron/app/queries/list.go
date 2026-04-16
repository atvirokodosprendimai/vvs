package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/cron/domain"
)

type ListJobsHandler struct{ repo domain.JobRepository }

func NewListJobsHandler(repo domain.JobRepository) *ListJobsHandler {
	return &ListJobsHandler{repo: repo}
}

func (h *ListJobsHandler) Handle(ctx context.Context) ([]JobReadModel, error) {
	jobs, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]JobReadModel, len(jobs))
	for i, j := range jobs {
		out[i] = toReadModel(j)
	}
	return out, nil
}

func toReadModel(j *domain.Job) JobReadModel {
	return JobReadModel{
		ID:        j.ID,
		Name:      j.Name,
		Schedule:  j.Schedule,
		JobType:   j.JobType,
		Payload:   j.Payload,
		Status:    j.Status,
		LastRun:   j.LastRun,
		LastError: j.LastError,
		NextRun:   j.NextRun,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}
