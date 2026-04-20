package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNetworkNameRequired   = errors.New("swarm network name is required")
	ErrNetworkSubnetRequired = errors.New("swarm network subnet is required")
	ErrNetworkNotFound       = errors.New("swarm network not found")
)

type SwarmNetworkDriver string

const (
	SwarmNetworkOverlay SwarmNetworkDriver = "overlay"
	SwarmNetworkMacvlan SwarmNetworkDriver = "macvlan"
	SwarmNetworkBridge  SwarmNetworkDriver = "bridge"
)

type SwarmNetworkScope string

const (
	SwarmNetworkScopeSwarm SwarmNetworkScope = "swarm"
	SwarmNetworkScopeLocal SwarmNetworkScope = "local"
)

// ReservedIP is an IP pre-allocated in VVS (not from Docker IPAM).
// Used for infra components (Traefik, DNS, gateways) in the upper subnet half.
type ReservedIP struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Label    string `json:"label"`
}

// SwarmNetwork represents a Docker network managed through VVS.
// The subnet is split into DHCP pool (lower half, Docker-managed) and
// reserved range (upper half, VVS-managed metadata only).
type SwarmNetwork struct {
	ID             string
	ClusterID      string // nullable — local networks not scoped to a swarm
	Name           string
	Driver         SwarmNetworkDriver
	Subnet         string // CIDR, e.g. "10.100.0.0/17"
	Gateway        string // optional
	DhcpRangeStart string // lower half start, e.g. "10.100.0.1"
	DhcpRangeEnd   string // lower half end, e.g. "10.100.63.254"
	Parent         string // macvlan only — physical interface name, free-text in VVS
	Options        map[string]string
	ReservedIPs    []ReservedIP
	Scope          SwarmNetworkScope
	DockerNetworkID string // Docker's assigned network ID after creation
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewSwarmNetwork(clusterID, name string, driver SwarmNetworkDriver, subnet, gateway string, scope SwarmNetworkScope) (*SwarmNetwork, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNetworkNameRequired
	}
	subnet = strings.TrimSpace(subnet)
	if subnet == "" {
		return nil, ErrNetworkSubnetRequired
	}
	now := time.Now().UTC()
	return &SwarmNetwork{
		ID:          uuid.Must(uuid.NewV7()).String(),
		ClusterID:   clusterID,
		Name:        name,
		Driver:      driver,
		Subnet:      subnet,
		Gateway:     strings.TrimSpace(gateway),
		Scope:       scope,
		Options:     make(map[string]string),
		ReservedIPs: []ReservedIP{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (n *SwarmNetwork) SetDHCPRange(start, end string) {
	n.DhcpRangeStart = start
	n.DhcpRangeEnd = end
	n.UpdatedAt = time.Now().UTC()
}

func (n *SwarmNetwork) SetDockerNetworkID(id string) {
	n.DockerNetworkID = id
	n.UpdatedAt = time.Now().UTC()
}

func (n *SwarmNetwork) UpdateReservedIPs(ips []ReservedIP) {
	n.ReservedIPs = ips
	n.UpdatedAt = time.Now().UTC()
}

type SwarmNetworkRepository interface {
	Save(ctx context.Context, network *SwarmNetwork) error
	FindByID(ctx context.Context, id string) (*SwarmNetwork, error)
	FindByClusterID(ctx context.Context, clusterID string) ([]*SwarmNetwork, error)
	FindAll(ctx context.Context) ([]*SwarmNetwork, error)
	Delete(ctx context.Context, id string) error
}
