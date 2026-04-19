package domain

import (
	"context"
	"time"
)

// STB is an optional Set-Top Box device record (inventory only).
type STB struct {
	ID         string
	MAC        string // normalised uppercase hex, e.g. 00:1A:2B:3C:4D:5E
	Model      string
	Firmware   string
	Serial     string
	CustomerID string
	AssignedAt time.Time
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// STBRepository is the port for STB inventory persistence.
type STBRepository interface {
	Save(ctx context.Context, stb *STB) error
	FindByID(ctx context.Context, id string) (*STB, error)
	FindByMAC(ctx context.Context, mac string) (*STB, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*STB, error)
	ListAll(ctx context.Context) ([]*STB, error)
	Delete(ctx context.Context, id string) error
}
