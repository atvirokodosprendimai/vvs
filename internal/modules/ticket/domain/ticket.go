package domain

import (
	"errors"
	"time"
)

// Status values for a Ticket.
const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusResolved   = "resolved"
	StatusClosed     = "closed"
)

// Priority values for a Ticket.
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

var (
	ErrNotFound          = errors.New("ticket not found")
	ErrSubjectRequired   = errors.New("subject is required")
	ErrAlreadyClosed     = errors.New("ticket is already closed")
	ErrInvalidTransition = errors.New("invalid status transition")
)

// Ticket is the aggregate root.
type Ticket struct {
	ID         string
	CustomerID string
	Subject    string
	Body       string
	Status     string
	Priority   string
	AssigneeID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewTicket creates a new open ticket with the given fields.
func NewTicket(id, customerID, subject, body, priority string) (*Ticket, error) {
	if customerID == "" {
		return nil, errors.New("customer id required")
	}
	if subject == "" {
		return nil, ErrSubjectRequired
	}
	if priority == "" {
		priority = PriorityNormal
	}
	now := time.Now().UTC()
	return &Ticket{
		ID:         id,
		CustomerID: customerID,
		Subject:    subject,
		Body:       body,
		Status:     StatusOpen,
		Priority:   priority,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// StartWork transitions open → in_progress.
func (t *Ticket) StartWork() error {
	if t.Status != StatusOpen {
		return ErrInvalidTransition
	}
	t.Status = StatusInProgress
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// Resolve transitions in_progress → resolved.
func (t *Ticket) Resolve() error {
	if t.Status != StatusInProgress {
		return ErrInvalidTransition
	}
	t.Status = StatusResolved
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// Close transitions any non-closed status → closed.
func (t *Ticket) Close() error {
	if t.Status == StatusClosed {
		return ErrAlreadyClosed
	}
	t.Status = StatusClosed
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// Reopen transitions resolved/closed → open.
func (t *Ticket) Reopen() error {
	if t.Status != StatusResolved && t.Status != StatusClosed {
		return ErrInvalidTransition
	}
	t.Status = StatusOpen
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// TicketComment is a sub-entity (not an aggregate) stored in ticket_comments.
type TicketComment struct {
	ID        string
	TicketID  string
	Body      string
	AuthorID  string
	CreatedAt time.Time
}
