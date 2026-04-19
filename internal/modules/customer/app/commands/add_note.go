package commands

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
)

type AddNoteCommand struct {
	CustomerID string
	Body       string
	AuthorID   string
}

type AddNoteHandler struct {
	repo domain.NoteRepository
}

func NewAddNoteHandler(repo domain.NoteRepository) *AddNoteHandler {
	return &AddNoteHandler{repo: repo}
}

func (h *AddNoteHandler) Handle(ctx context.Context, cmd AddNoteCommand) (*domain.CustomerNote, error) {
	note, err := domain.NewCustomerNote(cmd.CustomerID, cmd.Body, cmd.AuthorID)
	if err != nil {
		return nil, err
	}
	if err := h.repo.SaveNote(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}
