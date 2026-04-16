package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/task/domain"
)

type ListTasksForCustomerQuery struct {
	CustomerID string
}

type ListTasksForCustomerHandler struct {
	repo domain.TaskRepository
}

func NewListTasksForCustomerHandler(repo domain.TaskRepository) *ListTasksForCustomerHandler {
	return &ListTasksForCustomerHandler{repo: repo}
}

func (h *ListTasksForCustomerHandler) Handle(ctx context.Context, q ListTasksForCustomerQuery) ([]TaskReadModel, error) {
	tasks, err := h.repo.ListForCustomer(ctx, q.CustomerID)
	if err != nil {
		return nil, err
	}
	return tasksToReadModels(tasks), nil
}

func tasksToReadModels(tasks []*domain.Task) []TaskReadModel {
	out := make([]TaskReadModel, len(tasks))
	for i, t := range tasks {
		out[i] = taskToReadModel(t)
	}
	return out
}

func taskToReadModel(t *domain.Task) TaskReadModel {
	return TaskReadModel{
		ID:          t.ID,
		CustomerID:  t.CustomerID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		Priority:    t.Priority,
		DueDate:     t.DueDate,
		AssigneeID:  t.AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
