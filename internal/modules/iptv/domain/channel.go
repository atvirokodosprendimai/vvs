package domain

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("iptv: not found")

// Channel is a single broadcast stream.
type Channel struct {
	ID        string
	Name      string
	LogoURL   string
	StreamURL string
	DVRUrl    string // optional DVR base URL
	Category  string
	EPGSource string // tvg-id / XMLTV source ID
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChannelRepository is the port for channel persistence.
type ChannelRepository interface {
	Save(ctx context.Context, ch *Channel) error
	FindByID(ctx context.Context, id string) (*Channel, error)
	FindAll(ctx context.Context) ([]*Channel, error)
	FindByPackage(ctx context.Context, packageID string) ([]*Channel, error)
	Delete(ctx context.Context, id string) error
}
