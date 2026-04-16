package commands

import (
	"context"

	"github.com/vvs/isp/internal/modules/email/domain"
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
	threads domain.EmailThreadRepository
	tags    domain.EmailTagRepository
}

func NewApplyTagHandler(threads domain.EmailThreadRepository, tags domain.EmailTagRepository) *ApplyTagHandler {
	return &ApplyTagHandler{threads: threads, tags: tags}
}

func (h *ApplyTagHandler) Handle(ctx context.Context, cmd ApplyTagCommand) error {
	if _, err := h.threads.FindByID(ctx, cmd.ThreadID); err != nil {
		return err
	}
	if _, err := h.tags.FindByID(ctx, cmd.TagID); err != nil {
		return err
	}
	return h.tags.ApplyToThread(ctx, domain.EmailThreadTag{ThreadID: cmd.ThreadID, TagID: cmd.TagID})
}

type RemoveTagHandler struct {
	tags domain.EmailTagRepository
}

func NewRemoveTagHandler(tags domain.EmailTagRepository) *RemoveTagHandler {
	return &RemoveTagHandler{tags: tags}
}

func (h *RemoveTagHandler) Handle(ctx context.Context, cmd RemoveTagCommand) error {
	return h.tags.RemoveFromThread(ctx, cmd.ThreadID, cmd.TagID)
}
