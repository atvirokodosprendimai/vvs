package domain

import "context"

// TaskRepository is the port for task persistence.
type TaskRepository interface {
	Save(ctx context.Context, task *Task) error
	FindByID(ctx context.Context, id string) (*Task, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*Task, error)
	ListAll(ctx context.Context) ([]*Task, error)
	Delete(ctx context.Context, id string) error
}
