package domain

import (
	"context"
	"time"
)

// JobRepository is the persistence port for cron jobs.
type JobRepository interface {
	Save(ctx context.Context, j *Job) error
	FindByID(ctx context.Context, id string) (*Job, error)
	FindByName(ctx context.Context, name string) (*Job, error)
	ListDue(ctx context.Context, before time.Time) ([]*Job, error)
	ListAll(ctx context.Context) ([]*Job, error)
}
