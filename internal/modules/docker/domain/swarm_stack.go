package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrStackNameRequired    = errors.New("swarm stack name is required")
	ErrStackClusterRequired = errors.New("swarm stack requires a cluster ID")
	ErrStackNotFound        = errors.New("swarm stack not found")
	ErrRouteNotFound        = errors.New("swarm route not found")
)

type SwarmStackStatus string

const (
	SwarmStackDeploying SwarmStackStatus = "deploying"
	SwarmStackRunning   SwarmStackStatus = "running"
	SwarmStackUpdating  SwarmStackStatus = "updating"
	SwarmStackError     SwarmStackStatus = "error"
	SwarmStackRemoving  SwarmStackStatus = "removing"
)

// SwarmStack is a compose stack deployed on a specific swarm node via SSH.
// Overlay networking is handled by the Swarm; the compose project runs directly on TargetNodeID.
type SwarmStack struct {
	ID           string
	ClusterID    string
	TargetNodeID string // which node runs this compose project (SSH deploy target)
	Name         string
	ComposeYAML  string
	RegistryID   string // optional; empty = no registry auth before compose up
	Status       SwarmStackStatus
	ErrorMsg     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewSwarmStack(clusterID, targetNodeID, name, composeYAML string) (*SwarmStack, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return nil, ErrStackClusterRequired
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrStackNameRequired
	}
	now := time.Now().UTC()
	return &SwarmStack{
		ID:           uuid.Must(uuid.NewV7()).String(),
		ClusterID:    clusterID,
		TargetNodeID: strings.TrimSpace(targetNodeID),
		Name:         name,
		ComposeYAML:  composeYAML,
		Status:       SwarmStackDeploying,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *SwarmStack) MarkRunning() {
	s.Status = SwarmStackRunning
	s.ErrorMsg = ""
	s.UpdatedAt = time.Now().UTC()
}

func (s *SwarmStack) MarkError(msg string) {
	s.Status = SwarmStackError
	s.ErrorMsg = msg
	s.UpdatedAt = time.Now().UTC()
}

func (s *SwarmStack) MarkUpdating() {
	s.Status = SwarmStackUpdating
	s.UpdatedAt = time.Now().UTC()
}

func (s *SwarmStack) UpdateYAML(yaml string) {
	s.ComposeYAML = yaml
	s.Status = SwarmStackUpdating
	s.UpdatedAt = time.Now().UTC()
}

// SwarmRoute maps a hostname:port to a stack for Traefik file provider config generation.
type SwarmRoute struct {
	ID          string
	StackID     string
	Hostname    string
	Port        int
	StripPrefix bool
	CreatedAt   time.Time
}

func NewSwarmRoute(stackID, hostname string, port int, stripPrefix bool) (*SwarmRoute, error) {
	if strings.TrimSpace(hostname) == "" {
		return nil, errors.New("route hostname is required")
	}
	return &SwarmRoute{
		ID:          uuid.Must(uuid.NewV7()).String(),
		StackID:     stackID,
		Hostname:    strings.TrimSpace(hostname),
		Port:        port,
		StripPrefix: stripPrefix,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

type SwarmStackRepository interface {
	Save(ctx context.Context, stack *SwarmStack) error
	FindByID(ctx context.Context, id string) (*SwarmStack, error)
	FindByClusterID(ctx context.Context, clusterID string) ([]*SwarmStack, error)
	Delete(ctx context.Context, id string) error

	SaveRoute(ctx context.Context, route *SwarmRoute) error
	FindRoutesByStackID(ctx context.Context, stackID string) ([]*SwarmRoute, error)
	DeleteRoute(ctx context.Context, id string) error
}
