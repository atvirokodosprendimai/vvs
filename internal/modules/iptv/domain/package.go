package domain

import (
	"context"
	"time"
)

// Package is a bundle of channels available to subscribers.
type Package struct {
	ID          string
	Name        string
	PriceCents  int64 // monthly price in cents
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PackageChannel is a many-to-many join between a package and its channels.
type PackageChannel struct {
	PackageID string
	ChannelID string
}

// PackageRepository is the port for package persistence.
type PackageRepository interface {
	Save(ctx context.Context, p *Package) error
	FindByID(ctx context.Context, id string) (*Package, error)
	FindAll(ctx context.Context) ([]*Package, error)
	Delete(ctx context.Context, id string) error
	// Channel assignments
	AddChannel(ctx context.Context, packageID, channelID string) error
	RemoveChannel(ctx context.Context, packageID, channelID string) error
	ListChannelIDs(ctx context.Context, packageID string) ([]string, error)
	// SetChannelOrder sets per-package channel display order.
	// channelIDs not belonging to the package are silently skipped.
	SetChannelOrder(ctx context.Context, packageID string, channelIDs []string) error
}
