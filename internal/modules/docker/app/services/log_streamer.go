package services

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/dockerclient"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// NATSPublisher is the port for publishing NATS messages.
type NATSPublisher interface {
	Publish(subject string, data []byte) error
}

// LogStreamer tails Docker container logs and publishes each line to NATS.
type LogStreamer struct {
	nats    NATSPublisher
	factory domain.DockerClientFactory
}

func NewLogStreamer(nats NATSPublisher, factory domain.DockerClientFactory) *LogStreamer {
	return &LogStreamer{nats: nats, factory: factory}
}

// Stream starts a goroutine that tails logs from containerID on the given node.
// Each line is published to isp.docker.logs.{containerID}.
// The goroutine stops when ctx is cancelled.
func (s *LogStreamer) Stream(ctx context.Context, node *domain.DockerNode, containerID string) {
	go func() {
		if err := s.stream(ctx, node, containerID); err != nil {
			if ctx.Err() == nil {
				slog.Warn("docker log stream ended", "container", containerID[:min(12, len(containerID))], "err", err)
			}
		}
	}()
}

func (s *LogStreamer) stream(ctx context.Context, node *domain.DockerNode, containerID string) error {
	client, err := s.factory.ForNode(node)
	if err != nil {
		return fmt.Errorf("build docker client: %w", err)
	}

	rc, err := client.StreamLogs(ctx, containerID, true)
	if err != nil {
		return fmt.Errorf("stream logs: %w", err)
	}
	defer rc.Close()

	subject := fmt.Sprintf(events.DockerLogsLine.String(), containerID)
	reader := bufio.NewReader(rc)

	for {
		line, err := dockerclient.ReadMultiplexLine(reader)
		if err != nil {
			return err // io.EOF on stream end, context cancel propagates here
		}
		if line == "" {
			continue
		}
		if pubErr := s.nats.Publish(subject, []byte(line)); pubErr != nil {
			slog.Warn("docker log publish failed", "err", pubErr)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
