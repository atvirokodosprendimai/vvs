package domain

import (
	"errors"
	"time"
)

// Status values for a Task.
const (
	StatusTodo       = "todo"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusCancelled  = "cancelled"
)

// Priority values for a Task.
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
)

var (
	ErrNotFound          = errors.New("task not found")
	ErrTitleRequired     = errors.New("task title is required")
	ErrAlreadyDone       = errors.New("task is already done")
	ErrInvalidTransition = errors.New("invalid status transition")
)

// Task is the aggregate root for the task CRM module.
type Task struct {
	ID          string
	CustomerID  string // optional
	Title       string
	Description string
	Status      string
	Priority    string
	DueDate     *time.Time
	AssigneeID  string // optional
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewTask creates a new Task aggregate.
func NewTask(id, customerID, title, description, priority string, dueDate *time.Time, assigneeID string) (*Task, error) {
	if title == "" {
		return nil, ErrTitleRequired
	}
	if priority == "" {
		priority = PriorityNormal
	}
	now := time.Now().UTC()
	return &Task{
		ID:          id,
		CustomerID:  customerID,
		Title:       title,
		Description: description,
		Status:      StatusTodo,
		Priority:    priority,
		DueDate:     dueDate,
		AssigneeID:  assigneeID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Start transitions a task from todo → in_progress.
func (t *Task) Start() error {
	if t.Status != StatusTodo {
		return ErrInvalidTransition
	}
	t.Status = StatusInProgress
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// Complete transitions a task from todo or in_progress → done.
func (t *Task) Complete() error {
	if t.Status == StatusDone {
		return ErrAlreadyDone
	}
	if t.Status != StatusTodo && t.Status != StatusInProgress {
		return ErrInvalidTransition
	}
	t.Status = StatusDone
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// Cancel transitions a task from todo or in_progress → cancelled.
func (t *Task) Cancel() error {
	if t.Status != StatusTodo && t.Status != StatusInProgress {
		return ErrInvalidTransition
	}
	t.Status = StatusCancelled
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// Reopen transitions a task from done or cancelled → todo.
func (t *Task) Reopen() error {
	if t.Status != StatusDone && t.Status != StatusCancelled {
		return ErrInvalidTransition
	}
	t.Status = StatusTodo
	t.UpdatedAt = time.Now().UTC()
	return nil
}
