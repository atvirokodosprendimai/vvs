package queries

import "time"

// TaskReadModel is the flattened read model for the task list views.
type TaskReadModel struct {
	ID          string
	CustomerID  string
	Title       string
	Description string
	Status      string
	Priority    string
	DueDate     *time.Time
	AssigneeID  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
