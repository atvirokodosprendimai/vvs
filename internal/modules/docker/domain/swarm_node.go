package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSwarmNodeNameRequired = errors.New("swarm node name is required")
	ErrSwarmNodeHostRequired = errors.New("swarm node ssh host is required")
	ErrSwarmNodeNotFound     = errors.New("swarm node not found")
)

type SwarmNodeRole string

const (
	SwarmNodeManager SwarmNodeRole = "manager"
	SwarmNodeWorker  SwarmNodeRole = "worker"
)

type SwarmNodeStatus string

const (
	SwarmNodeProvisioning SwarmNodeStatus = "provisioning"
	SwarmNodeActive       SwarmNodeStatus = "active"
	SwarmNodeDown         SwarmNodeStatus = "down"
	SwarmNodeUnknown      SwarmNodeStatus = "unknown"
)

// SwarmNode represents a physical host in the swarm cluster.
// SshKey is stored AES-256-GCM encrypted at rest.
// VpnIP is the wgmesh0 WireGuard interface IP — auto-populated after provisioning.
type SwarmNode struct {
	ID               string
	ClusterID        string        // nullable — standalone SSH nodes are valid
	Role             SwarmNodeRole
	Name             string
	SshHost          string // physical/public IP or hostname for SSH provisioning only
	SshUser          string // defaults to "root"
	SshPort          int    // defaults to 22
	SshKey           []byte // PEM private key, encrypted at rest
	VpnIP            string // wgmesh0 IP — set after wgmesh deploy; empty until provisioned
	SwarmNodeID      string // Docker's internal node ID after joining swarm
	HetznerServerID  int    // Hetzner VPS ID; >0 means node was ordered via Hetzner API
	Status           SwarmNodeStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewSwarmNode(clusterID, name, sshHost, sshUser string, sshPort int, role SwarmNodeRole) (*SwarmNode, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrSwarmNodeNameRequired
	}
	sshHost = strings.TrimSpace(sshHost)
	if sshHost == "" {
		return nil, ErrSwarmNodeHostRequired
	}
	if sshUser == "" {
		sshUser = "root"
	}
	if sshPort == 0 {
		sshPort = 22
	}
	now := time.Now().UTC()
	return &SwarmNode{
		ID:        uuid.Must(uuid.NewV7()).String(),
		ClusterID: clusterID,
		Role:      role,
		Name:      name,
		SshHost:   sshHost,
		SshUser:   sshUser,
		SshPort:   sshPort,
		Status:    SwarmNodeProvisioning,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (n *SwarmNode) SetVpnIP(ip string) {
	n.VpnIP = ip
	n.UpdatedAt = time.Now().UTC()
}

func (n *SwarmNode) SetSwarmNodeID(id string) {
	n.SwarmNodeID = id
	n.Status = SwarmNodeActive
	n.UpdatedAt = time.Now().UTC()
}

func (n *SwarmNode) MarkDown() {
	n.Status = SwarmNodeDown
	n.UpdatedAt = time.Now().UTC()
}

type SwarmNodeRepository interface {
	Save(ctx context.Context, node *SwarmNode) error
	FindByID(ctx context.Context, id string) (*SwarmNode, error)
	FindByClusterID(ctx context.Context, clusterID string) ([]*SwarmNode, error)
	FindAll(ctx context.Context) ([]*SwarmNode, error)
	Delete(ctx context.Context, id string) error
}
