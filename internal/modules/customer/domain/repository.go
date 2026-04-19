package domain

import (
	"context"

	shareddomain "github.com/atvirokodosprendimai/vvs/internal/shared/domain"
)

type CustomerFilter struct {
	Search string
	Status string
}

type CustomerRepository interface {
	NextCode(ctx context.Context) (shareddomain.CompanyCode, error)
	Save(ctx context.Context, customer *Customer) error
	FindByID(ctx context.Context, id string) (*Customer, error)
	FindByCode(ctx context.Context, code string) (*Customer, error)
	FindAll(ctx context.Context, filter CustomerFilter, page shareddomain.Pagination) ([]*Customer, int64, error)
	Delete(ctx context.Context, id string) error
}
