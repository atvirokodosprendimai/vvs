package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/task/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type UpdateTaskCommand struct {
	ID          string
	Title       string
	Description string
	Priority    string
	DueDate     *time.Time
	AssigneeID  string
}

type UpdateTaskHandler struct {
	repo      domain.TaskRepository
	publisher events.EventPublisher
}

func NewUpdateTaskHandler(repo domain.TaskRepository, pub events.EventPublisher) *UpdateTaskHandler {
	return &UpdateTaskHandler{repo: repo, publisher: pub}
}

func (h *UpdateTaskHandler) Handle(ctx context.Context, cmd UpdateTaskCommand) (*domain.Task, error) {
	task, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if cmd.Title == "" {
		return nil, domain.ErrTitleRequired
	}

	task.Title = cmd.Title
	task.Description = cmd.Description
	task.Priority = cmd.Priority
	task.DueDate = cmd.DueDate
	task.AssigneeID = cmd.AssigneeID
	task.UpdatedAt = time.Now().UTC()

	if err := h.repo.Save(ctx, task); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, events.TaskUpdated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "task.updated",
		AggregateID: task.ID,
		OccurredAt:  task.UpdatedAt,
	})
	return task, nil
}
