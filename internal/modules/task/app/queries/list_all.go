package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/task/domain"
)

type ListAllTasksHandler struct {
	repo domain.TaskRepository
}

func NewListAllTasksHandler(repo domain.TaskRepository) *ListAllTasksHandler {
	return &ListAllTasksHandler{repo: repo}
}

func (h *ListAllTasksHandler) Handle(ctx context.Context) ([]TaskReadModel, error) {
	tasks, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return tasksToReadModels(tasks), nil
}
