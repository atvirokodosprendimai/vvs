package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// IPTVStackStatus represents the lifecycle state of a managed IPTV stack.
type IPTVStackStatus string

const (
	IPTVStackPending   IPTVStackStatus = "pending"
	IPTVStackDeploying IPTVStackStatus = "deploying"
	IPTVStackRunning   IPTVStackStatus = "running"
	IPTVStackError     IPTVStackStatus = "error"
)

// IPTVStack is a managed Docker Compose deployment on a swarm node that
// runs N ffmpeg containers (internal providers) and one Caddy file server.
type IPTVStack struct {
	ID               string
	Name             string
	ClusterID        string
	NodeID           string
	WANNetworkID     string
	OverlayNetworkID string
	WANNetworkName   string // Docker network name for compose
	OverlayNetworkName string
	WanIP            string
	Status           IPTVStackStatus
	LastDeployedAt   *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IPTVStackChannel links a channel+provider pair to a stack.
type IPTVStackChannel struct {
	ID         string
	StackID    string
	ChannelID  string
	ProviderID string
}

// IPTVStackChannelDetail is the resolved detail needed for compose generation.
type IPTVStackChannelDetail struct {
	ChannelSlug   string
	ProviderType  ProviderType
	ResolvedURL   string // source URL for ffmpeg (internal) or proxy target (external)
}

// IPTVStackRepository is the port for IPTVStack persistence.
type IPTVStackRepository interface {
	Save(ctx context.Context, s *IPTVStack) error
	FindByID(ctx context.Context, id string) (*IPTVStack, error)
	FindAll(ctx context.Context) ([]*IPTVStack, error)
	Delete(ctx context.Context, id string) error
}

// IPTVStackChannelRepository manages channel assignments within a stack.
type IPTVStackChannelRepository interface {
	Save(ctx context.Context, sc *IPTVStackChannel) error
	FindByStackID(ctx context.Context, stackID string) ([]*IPTVStackChannel, error)
	FindByStackIDAndChannelID(ctx context.Context, stackID, channelID string) (*IPTVStackChannel, error)
	Delete(ctx context.Context, id string) error
	DeleteByStackID(ctx context.Context, stackID string) error
}

// GenerateIPTVCompose produces a docker-compose YAML string for the given stack and channels.
// One ffmpeg service per internal channel, one Caddy file-server, both networks external.
func GenerateIPTVCompose(stack *IPTVStack, channels []IPTVStackChannelDetail) string {
	var sb strings.Builder

	sb.WriteString("# managed by VVS IPTV stack — do not edit manually\n")
	sb.WriteString("services:\n")

	// ffmpeg services for internal providers
	for _, ch := range channels {
		if ch.ProviderType != ProviderInternal {
			continue
		}
		slug := ch.ChannelSlug
		// Escape single quotes in URL for shell
		safeURL := strings.ReplaceAll(ch.ResolvedURL, "'", "'\"'\"'")
		sb.WriteString(fmt.Sprintf(`
  ffmpeg-%s:
    image: jrottenberg/ffmpeg:4.4-alpine
    network_mode: host
    restart: unless-stopped
    volumes:
      - /dev/shm/%s:/data
    entrypoint: >
      sh -c "mkdir -p /data && ffmpeg -re -i '%s'
      -c:v copy -c:a copy -f hls
      -hls_time 2 -hls_list_size 5
      -hls_flags delete_segments+append_list
      -hls_segment_filename /data/seg%%d.ts
      /data/stream.m3u8"
`, slug, slug, safeURL))
	}

	// Caddy file-server
	sb.WriteString(fmt.Sprintf(`
  caddy:
    image: caddy:alpine
    entrypoint: sh -c "caddy file-server --root /data --browse --listen :80"
    restart: unless-stopped
    volumes:
      - /dev/shm:/data
    networks:
      %s:
        ipv4_address: %s
      %s:
`, stack.WANNetworkName, stack.WanIP, stack.OverlayNetworkName))

	// Networks block
	sb.WriteString("\nnetworks:\n")
	sb.WriteString(fmt.Sprintf("  %s:\n    external: true\n", stack.WANNetworkName))
	sb.WriteString(fmt.Sprintf("  %s:\n    external: true\n", stack.OverlayNetworkName))

	return sb.String()
}
