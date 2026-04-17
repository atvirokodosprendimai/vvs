package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/task/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type CreateTaskCommand struct {
	CustomerID  string
	Title       string
	Description string
	Priority    string
	DueDate     *time.Time
	AssigneeID  string
}

type CreateTaskHandler struct {
	repo      domain.TaskRepository
	publisher events.EventPublisher
}

func NewCreateTaskHandler(repo domain.TaskRepository, pub events.EventPublisher) *CreateTaskHandler {
	return &CreateTaskHandler{repo: repo, publisher: pub}
}

func (h *CreateTaskHandler) Handle(ctx context.Context, cmd CreateTaskCommand) (*domain.Task, error) {
	task, err := domain.NewTask(
		uuid.Must(uuid.NewV7()).String(),
		cmd.CustomerID,
		cmd.Title,
		cmd.Description,
		cmd.Priority,
		cmd.DueDate,
		cmd.AssigneeID,
	)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, task); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, events.TaskCreated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "task.created",
		AggregateID: task.ID,
		OccurredAt:  task.CreatedAt,
	})
	return task, nil
}
