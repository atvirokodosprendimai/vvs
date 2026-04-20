package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNodeNameRequired = errors.New("docker node name is required")
	ErrHostRequired     = errors.New("docker node host is required")
	ErrNodeNotFound     = errors.New("docker node not found")
	ErrNodeHasServices  = errors.New("docker node has active services — remove them first")
)

// DockerNode stores connection details for a Docker host.
// TLSCert, TLSKey, TLSCA are stored AES-256-GCM encrypted at rest.
type DockerNode struct {
	ID        string
	Name      string // human label, e.g. "prod-docker-01"
	Host      string // unix:///var/run/docker.sock  or  tcp://host:2376
	IsLocal   bool   // true → local socket, no TLS credentials needed
	TLSCert   []byte // client cert (PEM), encrypted at rest
	TLSKey    []byte // client key (PEM), encrypted at rest
	TLSCA     []byte // CA cert (PEM), encrypted at rest
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewDockerNode(name, host string, isLocal bool) (*DockerNode, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNodeNameRequired
	}
	if isLocal {
		host = "unix:///var/run/docker.sock"
	} else {
		host = strings.TrimSpace(host)
		if host == "" {
			return nil, ErrHostRequired
		}
	}
	now := time.Now().UTC()
	return &DockerNode{
		ID:        uuid.Must(uuid.NewV7()).String(),
		Name:      name,
		Host:      host,
		IsLocal:   isLocal,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (n *DockerNode) Update(name, host string, isLocal bool, tlsCert, tlsKey, tlsCA []byte, notes string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNodeNameRequired
	}
	if isLocal {
		host = "unix:///var/run/docker.sock"
	} else {
		host = strings.TrimSpace(host)
		if host == "" {
			return ErrHostRequired
		}
	}
	n.Name = name
	n.Host = host
	n.IsLocal = isLocal
	if len(tlsCert) > 0 {
		n.TLSCert = tlsCert
	}
	if len(tlsKey) > 0 {
		n.TLSKey = tlsKey
	}
	if len(tlsCA) > 0 {
		n.TLSCA = tlsCA
	}
	n.Notes = strings.TrimSpace(notes)
	n.UpdatedAt = time.Now().UTC()
	return nil
}

// DockerNodeRepository is the persistence port for DockerNode.
type DockerNodeRepository interface {
	Save(ctx context.Context, node *DockerNode) error
	FindByID(ctx context.Context, id string) (*DockerNode, error)
	FindAll(ctx context.Context) ([]*DockerNode, error)
	Delete(ctx context.Context, id string) error
}
