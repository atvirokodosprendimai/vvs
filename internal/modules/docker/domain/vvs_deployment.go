package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDeploymentNodeRequired      = errors.New("deployment target node is required")
	ErrDeploymentComponentRequired = errors.New("deployment component is required")
	ErrDeploymentSourceRequired    = errors.New("deployment source (image or git) is required")
	ErrDeploymentNATSRequired      = errors.New("NATS URL is required for deployment")
	ErrDeploymentNotFound          = errors.New("VVS deployment not found")
)

type VVSComponentType string

const (
	VVSComponentPortal VVSComponentType = "portal"
	VVSComponentSTB    VVSComponentType = "stb"
)

type VVSDeploySource string

const (
	VVSDeployImage VVSDeploySource = "image" // pull from registry (or public)
	VVSDeployGit   VVSDeploySource = "git"   // clone + build from Dockerfile
)

type VVSDeploymentStatus string

const (
	VVSDeploymentPending  VVSDeploymentStatus = "pending"
	VVSDeploymentRunning  VVSDeploymentStatus = "running"
	VVSDeploymentError    VVSDeploymentStatus = "error"
	VVSDeploymentStopped  VVSDeploymentStatus = "stopped"
)

// VVSDeployment represents a deployed vvs-portal or vvs-stb container on a swarm node.
type VVSDeployment struct {
	ID         string
	ClusterID  string
	NodeID     string           // specific swarm node to run on
	Component  VVSComponentType // portal | stb
	Source     VVSDeploySource  // image | git

	// image source
	ImageURL   string // e.g. registry.example.com/vvs-portal:latest or docker.io/org/vvs-portal:latest
	RegistryID string // optional: ID of ContainerRegistry for docker login

	// git source
	GitURL string // git clone URL
	GitRef string // branch or tag (default: main)

	// runtime config
	NATSUrl string            // core NATS URL, reachable via VPN
	Port    int               // host port to bind → container port 8080
	EnvVars map[string]string // additional environment variables

	// state
	Status         VVSDeploymentStatus
	ErrorMsg       string
	LastDeployedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewVVSDeployment(
	clusterID, nodeID string,
	component VVSComponentType,
	source VVSDeploySource,
	natsURL string,
	port int,
) (*VVSDeployment, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, ErrDeploymentNodeRequired
	}
	if component == "" {
		return nil, ErrDeploymentComponentRequired
	}
	if source == "" {
		return nil, ErrDeploymentSourceRequired
	}
	natsURL = strings.TrimSpace(natsURL)
	if natsURL == "" {
		return nil, ErrDeploymentNATSRequired
	}
	if port == 0 {
		switch component {
		case VVSComponentPortal:
			port = 8080
		case VVSComponentSTB:
			port = 8090
		}
	}
	now := time.Now().UTC()
	return &VVSDeployment{
		ID:        uuid.Must(uuid.NewV7()).String(),
		ClusterID: clusterID,
		NodeID:    nodeID,
		Component: component,
		Source:    source,
		NATSUrl:   natsURL,
		Port:      port,
		EnvVars:   make(map[string]string),
		Status:    VVSDeploymentPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (d *VVSDeployment) MarkRunning() {
	now := time.Now().UTC()
	d.Status = VVSDeploymentRunning
	d.ErrorMsg = ""
	d.LastDeployedAt = &now
	d.UpdatedAt = now
}

func (d *VVSDeployment) MarkError(msg string) {
	d.Status = VVSDeploymentError
	d.ErrorMsg = msg
	d.UpdatedAt = time.Now().UTC()
}

func (d *VVSDeployment) MarkStopped() {
	d.Status = VVSDeploymentStopped
	d.UpdatedAt = time.Now().UTC()
}

// ServiceName returns the Docker compose service name for this deployment.
func (d *VVSDeployment) ServiceName() string {
	return "vvs-" + string(d.Component)
}

// ComposePath returns the path where compose file is stored on the target node.
func (d *VVSDeployment) ComposePath() string {
	return "/opt/vvs/components/" + string(d.Component) + "/" + d.ID + "/docker-compose.yml"
}

// ComposeDir returns the directory for this deployment on the target node.
func (d *VVSDeployment) ComposeDir() string {
	return "/opt/vvs/components/" + string(d.Component) + "/" + d.ID
}

type VVSDeploymentRepository interface {
	Save(ctx context.Context, d *VVSDeployment) error
	FindByID(ctx context.Context, id string) (*VVSDeployment, error)
	FindAll(ctx context.Context) ([]*VVSDeployment, error)
	Delete(ctx context.Context, id string) error
}
