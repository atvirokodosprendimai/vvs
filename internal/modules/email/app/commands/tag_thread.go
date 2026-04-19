package commands

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"
)

type ApplyTagCommand struct {
	ThreadID string
	TagID    string
}

type RemoveTagCommand struct {
	ThreadID string
	TagID    string
}

type ApplyTagHandler struct {
	threads   domain.EmailThreadRepository
	tags      domain.EmailTagRepository
	publisher events.EventPublisher
}

func NewApplyTagHandler(threads domain.EmailThreadRepository, tags domain.EmailTagRepository, pub events.EventPublisher) *ApplyTagHandler {
	return &ApplyTagHandler{threads: threads, tags: tags, publisher: pub}
}

func (h *ApplyTagHandler) Handle(ctx context.Context, cmd ApplyTagCommand) error {
	if _, err := h.threads.FindByID(ctx, cmd.ThreadID); err != nil {
		return err
	}
	if _, err := h.tags.FindByID(ctx, cmd.TagID); err != nil {
		return err
	}
	if err := h.tags.ApplyToThread(ctx, domain.EmailThreadTag{ThreadID: cmd.ThreadID, TagID: cmd.TagID}); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.EmailThreadTagged.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.thread_tagged",
		AggregateID: cmd.ThreadID, OccurredAt: time.Now().UTC(),
	})
	return nil
}

type RemoveTagHandler struct {
	tags      domain.EmailTagRepository
	publisher events.EventPublisher
}

func NewRemoveTagHandler(tags domain.EmailTagRepository, pub events.EventPublisher) *RemoveTagHandler {
	return &RemoveTagHandler{tags: tags, publisher: pub}
}

func (h *RemoveTagHandler) Handle(ctx context.Context, cmd RemoveTagCommand) error {
	if err := h.tags.RemoveFromThread(ctx, cmd.ThreadID, cmd.TagID); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.EmailThreadUntagged.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.thread_untagged",
		AggregateID: cmd.ThreadID, OccurredAt: time.Now().UTC(),
	})
	return nil
}

// ToggleStarHandler toggles the "starred" system tag on a thread.
type ToggleStarHandler struct {
	tags      domain.EmailTagRepository
	publisher events.EventPublisher
}

func NewToggleStarHandler(tags domain.EmailTagRepository, pub events.EventPublisher) *ToggleStarHandler {
	return &ToggleStarHandler{tags: tags, publisher: pub}
}

func (h *ToggleStarHandler) Handle(ctx context.Context, threadID string) error {
	starTag, err := h.tags.FindSystemTag(ctx, domain.TagStarred)
	if err != nil {
		return err
	}
	current, err := h.tags.ListForThread(ctx, threadID)
	if err != nil {
		return err
	}
	for _, t := range current {
		if t.ID == starTag.ID {
			// already starred — remove
			if err := h.tags.RemoveFromThread(ctx, threadID, starTag.ID); err != nil {
				return err
			}
			h.publisher.Publish(ctx, events.EmailThreadUntagged.String(), events.DomainEvent{
				ID: uuid.Must(uuid.NewV7()).String(), Type: "email.thread_untagged",
				AggregateID: threadID, OccurredAt: time.Now().UTC(),
			})
			return nil
		}
	}
	// not starred — apply
	if err := h.tags.ApplyToThread(ctx, domain.EmailThreadTag{ThreadID: threadID, TagID: starTag.ID}); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.EmailThreadTagged.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.thread_tagged",
		AggregateID: threadID, OccurredAt: time.Now().UTC(),
	})
	return nil
}
