package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/task/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// ChangeTaskStatusCommand changes the status of a task.
// Action must be one of: start, complete, cancel, reopen.
type ChangeTaskStatusCommand struct {
	ID     string
	Action string // start | complete | cancel | reopen
}

type ChangeTaskStatusHandler struct {
	repo      domain.TaskRepository
	publisher events.EventPublisher
}

func NewChangeTaskStatusHandler(repo domain.TaskRepository, pub events.EventPublisher) *ChangeTaskStatusHandler {
	return &ChangeTaskStatusHandler{repo: repo, publisher: pub}
}

func (h *ChangeTaskStatusHandler) Handle(ctx context.Context, cmd ChangeTaskStatusCommand) (*domain.Task, error) {
	task, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	switch cmd.Action {
	case "start":
		err = task.Start()
	case "complete":
		err = task.Complete()
	case "cancel":
		err = task.Cancel()
	case "reopen":
		err = task.Reopen()
	default:
		return nil, fmt.Errorf("unknown action: %q", cmd.Action)
	}
	if err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, task); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, "isp.task.status_changed", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "task.status_changed",
		AggregateID: task.ID,
		OccurredAt:  time.Now().UTC(),
	})
	return task, nil
}
