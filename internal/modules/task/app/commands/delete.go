package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/task/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type DeleteTaskCommand struct {
	ID string
}

type DeleteTaskHandler struct {
	repo      domain.TaskRepository
	publisher events.EventPublisher
}

func NewDeleteTaskHandler(repo domain.TaskRepository, pub events.EventPublisher) *DeleteTaskHandler {
	return &DeleteTaskHandler{repo: repo, publisher: pub}
}

func (h *DeleteTaskHandler) Handle(ctx context.Context, cmd DeleteTaskCommand) error {
	if err := h.repo.Delete(ctx, cmd.ID); err != nil {
		return err
	}
	h.publisher.Publish(ctx, "isp.task.deleted", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "task.deleted",
		AggregateID: cmd.ID,
		OccurredAt:  time.Now().UTC(),
	})
	return nil
}
