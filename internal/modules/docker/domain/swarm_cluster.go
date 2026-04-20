package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrClusterNameRequired  = errors.New("swarm cluster name is required")
	ErrWgmeshKeyTooShort    = errors.New("wgmesh key must be at least 32 characters")
	ErrClusterNotFound      = errors.New("swarm cluster not found")
)

type SwarmClusterStatus string

const (
	SwarmClusterInitializing SwarmClusterStatus = "initializing"
	SwarmClusterActive       SwarmClusterStatus = "active"
	SwarmClusterDegraded     SwarmClusterStatus = "degraded"
	SwarmClusterUnknown      SwarmClusterStatus = "unknown"
)

// SwarmCluster represents a Docker Swarm cluster managed by VVS.
// WgmeshKey, ManagerToken, WorkerToken are stored AES-256-GCM encrypted at rest.
type SwarmCluster struct {
	ID           string
	Name         string
	WgmeshKey    string             // shared 32+ char key for all nodes; encrypted at rest
	ManagerToken string             // docker swarm join-token manager; encrypted at rest
	WorkerToken  string             // docker swarm join-token worker; encrypted at rest
	AdvertiseAddr string            // manager node's vpnIP (wgmesh0), set after SwarmInit
	Notes        string
	Status       SwarmClusterStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewSwarmCluster(name, wgmeshKey, notes string) (*SwarmCluster, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrClusterNameRequired
	}
	if len(strings.TrimSpace(wgmeshKey)) < 32 {
		return nil, ErrWgmeshKeyTooShort
	}
	now := time.Now().UTC()
	return &SwarmCluster{
		ID:        uuid.Must(uuid.NewV7()).String(),
		Name:      name,
		WgmeshKey: strings.TrimSpace(wgmeshKey),
		Notes:     strings.TrimSpace(notes),
		Status:    SwarmClusterInitializing,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (c *SwarmCluster) SetTokens(managerToken, workerToken, advertiseAddr string) {
	c.ManagerToken = managerToken
	c.WorkerToken = workerToken
	c.AdvertiseAddr = advertiseAddr
	c.Status = SwarmClusterActive
	c.UpdatedAt = time.Now().UTC()
}

func (c *SwarmCluster) MarkActive() {
	c.Status = SwarmClusterActive
	c.UpdatedAt = time.Now().UTC()
}

type SwarmClusterRepository interface {
	Save(ctx context.Context, cluster *SwarmCluster) error
	FindByID(ctx context.Context, id string) (*SwarmCluster, error)
	FindAll(ctx context.Context) ([]*SwarmCluster, error)
	Delete(ctx context.Context, id string) error
}
