package queries

import "time"

// TicketReadModel is the flattened read model for the ticket list view.
type TicketReadModel struct {
	ID         string
	CustomerID string
	Subject    string
	Body       string
	Status     string
	Priority   string
	AssigneeID string
	Comments   []CommentReadModel
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CommentReadModel is the flattened read model for a ticket comment.
type CommentReadModel struct {
	ID        string
	TicketID  string
	Body      string
	AuthorID  string
	CreatedAt time.Time
}
