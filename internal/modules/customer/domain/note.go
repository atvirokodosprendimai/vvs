package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrNoteBodyRequired = errors.New("note body required")

type CustomerNote struct {
	ID         string
	CustomerID string
	Body       string
	AuthorID   string
	CreatedAt  time.Time
}

func NewCustomerNote(customerID, body, authorID string) (*CustomerNote, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrNoteBodyRequired
	}
	return &CustomerNote{
		ID:         uuid.Must(uuid.NewV7()).String(),
		CustomerID: customerID,
		Body:       body,
		AuthorID:   authorID,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

type NoteRepository interface {
	SaveNote(ctx context.Context, note *CustomerNote) error
	ListNotes(ctx context.Context, customerID string) ([]*CustomerNote, error)
}
