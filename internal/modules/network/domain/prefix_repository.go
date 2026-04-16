package domain

import "context"

type PrefixRepository interface {
	Save(ctx context.Context, p *NetBoxPrefix) error
	FindByID(ctx context.Context, id string) (*NetBoxPrefix, error)
	ListByLocation(ctx context.Context, location string) ([]*NetBoxPrefix, error) // ordered by priority asc
	ListAll(ctx context.Context) ([]*NetBoxPrefix, error)
	ListLocations(ctx context.Context) ([]string, error) // distinct location values
	Delete(ctx context.Context, id string) error
}
