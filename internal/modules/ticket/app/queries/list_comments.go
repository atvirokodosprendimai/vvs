package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/ticket/domain"
)

type ListCommentsQuery struct {
	TicketID string
}

type ListCommentsHandler struct {
	repo domain.TicketRepository
}

func NewListCommentsHandler(repo domain.TicketRepository) *ListCommentsHandler {
	return &ListCommentsHandler{repo: repo}
}

func (h *ListCommentsHandler) Handle(ctx context.Context, q ListCommentsQuery) ([]CommentReadModel, error) {
	comments, err := h.repo.ListComments(ctx, q.TicketID)
	if err != nil {
		return nil, err
	}
	out := make([]CommentReadModel, len(comments))
	for i, c := range comments {
		out[i] = CommentReadModel{
			ID:        c.ID,
			TicketID:  c.TicketID,
			Body:      c.Body,
			AuthorID:  c.AuthorID,
			CreatedAt: c.CreatedAt,
		}
	}
	return out, nil
}
