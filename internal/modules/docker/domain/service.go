package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrServiceNameRequired = errors.New("docker service name is required")
	ErrComposeYAMLRequired = errors.New("compose YAML is required")
	ErrNodeIDRequired      = errors.New("docker node ID is required")
	ErrServiceNotFound     = errors.New("docker service not found")
)

type ServiceStatus string

const (
	ServiceStatusDeploying ServiceStatus = "deploying"
	ServiceStatusRunning   ServiceStatus = "running"
	ServiceStatusStopped   ServiceStatus = "stopped"
	ServiceStatusError     ServiceStatus = "error"
	ServiceStatusRemoving  ServiceStatus = "removing"
)

// DockerService represents a deployed docker-compose project on a node.
type DockerService struct {
	ID          string
	NodeID      string
	Name        string        // compose project name
	ComposeYAML string        // raw YAML stored for re-deploy
	Status      ServiceStatus
	ErrorMsg    string // set when Status == error
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewDockerService(nodeID, name, composeYAML string) (*DockerService, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, ErrNodeIDRequired
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrServiceNameRequired
	}
	composeYAML = strings.TrimSpace(composeYAML)
	if composeYAML == "" {
		return nil, ErrComposeYAMLRequired
	}
	now := time.Now().UTC()
	return &DockerService{
		ID:          uuid.Must(uuid.NewV7()).String(),
		NodeID:      nodeID,
		Name:        name,
		ComposeYAML: composeYAML,
		Status:      ServiceStatusDeploying,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *DockerService) MarkRunning() {
	s.Status = ServiceStatusRunning
	s.ErrorMsg = ""
	s.UpdatedAt = time.Now().UTC()
}

func (s *DockerService) MarkStopped() {
	s.Status = ServiceStatusStopped
	s.ErrorMsg = ""
	s.UpdatedAt = time.Now().UTC()
}

func (s *DockerService) MarkError(msg string) {
	s.Status = ServiceStatusError
	s.ErrorMsg = msg
	s.UpdatedAt = time.Now().UTC()
}

func (s *DockerService) MarkRemoving() {
	s.Status = ServiceStatusRemoving
	s.UpdatedAt = time.Now().UTC()
}

func (s *DockerService) UpdateYAML(composeYAML string) error {
	composeYAML = strings.TrimSpace(composeYAML)
	if composeYAML == "" {
		return ErrComposeYAMLRequired
	}
	s.ComposeYAML = composeYAML
	s.Status = ServiceStatusDeploying
	s.ErrorMsg = ""
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// DockerServiceRepository is the persistence port for DockerService.
type DockerServiceRepository interface {
	Save(ctx context.Context, svc *DockerService) error
	FindByID(ctx context.Context, id string) (*DockerService, error)
	FindAll(ctx context.Context) ([]*DockerService, error)
	FindByNodeID(ctx context.Context, nodeID string) ([]*DockerService, error)
	Delete(ctx context.Context, id string) error
}
