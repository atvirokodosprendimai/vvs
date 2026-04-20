package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

var ErrNotFound = errors.New("iptv: not found")

// slugRe strips characters that aren't alphanumeric or hyphens.
var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

// Slugify converts a string to a URL-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	s = regexp.MustCompile(`-{2,}`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// Channel is a single broadcast stream.
type Channel struct {
	ID        string
	Name      string
	Slug      string // URL-safe identifier; used in provider URL templates and /dev/shm/{slug}/
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

// ── ChannelProvider ───────────────────────────────────────────────────────────

// ProviderType classifies whether a provider transcodes internally or proxies externally.
type ProviderType string

const (
	ProviderInternal ProviderType = "internal" // ffmpeg → HLS via /dev/shm
	ProviderExternal ProviderType = "external" // direct URL to third-party
)

// ChannelProvider is a source for a channel.
// URLTemplate may contain {channel} and {token} placeholders substituted at resolution time.
type ChannelProvider struct {
	ID          string
	ChannelID   string
	Name        string
	URLTemplate string // e.g. "http://host/{token}/{channel}/stream.m3u8"
	Token       string // substituted as {token}
	Type        ProviderType
	Priority    int
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ResolveProviderURL substitutes {channel} and {token} placeholders in the template.
func ResolveProviderURL(template, channelSlug, token string) string {
	s := strings.ReplaceAll(template, "{channel}", channelSlug)
	s = strings.ReplaceAll(s, "{token}", token)
	return s
}

// ChannelProviderRepository is the port for channel provider persistence.
type ChannelProviderRepository interface {
	Save(ctx context.Context, p *ChannelProvider) error
	FindByID(ctx context.Context, id string) (*ChannelProvider, error)
	FindByChannelID(ctx context.Context, channelID string) ([]*ChannelProvider, error)
	Delete(ctx context.Context, id string) error
}
