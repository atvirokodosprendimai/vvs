package domain

import "context"

type RouterRepository interface {
	Save(ctx context.Context, r *Router) error
	FindByID(ctx context.Context, id string) (*Router, error)
	FindAll(ctx context.Context) ([]*Router, error)
	Delete(ctx context.Context, id string) error
}
