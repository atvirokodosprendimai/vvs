package domain

import (
	"context"
	"io"
)

// ContainerInfo is a read model for a running/stopped container.
type ContainerInfo struct {
	ID     string
	Name   string
	Image  string
	Status string // human string from Docker daemon, e.g. "Up 3 hours"
	State  string // running | stopped | exited | paused | ...
	Ports  []string
}

// DockerClient is the port for interacting with a Docker daemon.
// Implementations live in adapters/dockerclient/.
type DockerClient interface {
	// Ping checks connectivity to the Docker daemon.
	Ping(ctx context.Context) error

	// Deploy creates/starts all services defined in a docker-compose YAML.
	// Networks and volumes are created before containers.
	// Containers are started in dependency order (respects depends_on).
	Deploy(ctx context.Context, projectName, composeYAML string) error

	// ListContainers returns containers belonging to a compose project.
	ListContainers(ctx context.Context, projectName string) ([]ContainerInfo, error)

	// StartContainer starts a stopped container by ID.
	StartContainer(ctx context.Context, containerID string) error

	// StopContainer stops a running container by ID.
	StopContainer(ctx context.Context, containerID string) error

	// RemoveContainers stops and removes all containers for a compose project,
	// along with project-scoped networks and anonymous volumes.
	RemoveContainers(ctx context.Context, projectName string) error

	// StreamLogs returns a ReadCloser that streams stdout+stderr from a container.
	// Caller must close the reader when done.
	StreamLogs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error)
}

// DockerClientFactory builds a DockerClient for a given node.
type DockerClientFactory interface {
	ForNode(node *DockerNode) (DockerClient, error)
}
