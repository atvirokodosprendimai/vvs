package domain

import (
	"context"

	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type ProductFilter struct {
	Search string
	Type   string
	Active *bool
}

type ProductRepository interface {
	Save(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id string) (*Product, error)
	FindAll(ctx context.Context, filter ProductFilter, page shareddomain.Pagination) ([]*Product, int64, error)
	FindActive(ctx context.Context, page shareddomain.Pagination) ([]*Product, int64, error)
	Delete(ctx context.Context, id string) error
}
