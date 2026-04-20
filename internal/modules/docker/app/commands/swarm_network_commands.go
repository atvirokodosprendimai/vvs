package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/google/uuid"
)

// ── CreateSwarmNetwork ────────────────────────────────────────────────────────

type CreateSwarmNetworkCommand struct {
	ClusterID string
	Name      string
	Driver    domain.SwarmNetworkDriver
	Subnet    string
	Gateway   string
	Parent    string // macvlan only
	Scope     domain.SwarmNetworkScope
}

type CreateSwarmNetworkHandler struct {
	clusterRepo domain.SwarmClusterRepository
	nodeRepo    domain.SwarmNodeRepository
	networkRepo domain.SwarmNetworkRepository
	factory     domain.SwarmClientFactory
	publisher   events.EventPublisher
}

func NewCreateSwarmNetworkHandler(
	clusterRepo domain.SwarmClusterRepository,
	nodeRepo domain.SwarmNodeRepository,
	networkRepo domain.SwarmNetworkRepository,
	factory domain.SwarmClientFactory,
	pub events.EventPublisher,
) *CreateSwarmNetworkHandler {
	return &CreateSwarmNetworkHandler{
		clusterRepo: clusterRepo,
		nodeRepo:    nodeRepo,
		networkRepo: networkRepo,
		factory:     factory,
		publisher:   pub,
	}
}

func (h *CreateSwarmNetworkHandler) Handle(ctx context.Context, cmd CreateSwarmNetworkCommand) (*domain.SwarmNetwork, error) {
	net, err := domain.NewSwarmNetwork(cmd.ClusterID, cmd.Name, cmd.Driver, cmd.Subnet, cmd.Gateway, cmd.Scope)
	if err != nil {
		return nil, err
	}
	net.Parent = cmd.Parent

	// Split subnet — DHCP in lower half, reserved in upper half
	dhcpStart, dhcpEnd, _, _, _ := domain.SplitSubnet(cmd.Subnet)
	net.SetDHCPRange(dhcpStart, dhcpEnd)

	// Compute IPRange CIDR from dhcpStart–dhcpEnd for NetworkCreate
	dhcpCIDR := domain.DHCPRangeCIDR(cmd.Subnet)

	// Find manager node to create network on
	managerNode, err := h.findManagerNode(ctx, cmd.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("find manager node: %w", err)
	}

	client, err := h.factory.ForSwarmNode(managerNode)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	dockerID, err := client.NetworkCreate(ctx, domain.NetworkCreateRequest{
		Name:       cmd.Name,
		Driver:     string(cmd.Driver),
		Subnet:     cmd.Subnet,
		Gateway:    cmd.Gateway,
		IPRange:    dhcpCIDR,
		Parent:     cmd.Parent,
		Attachable: cmd.Driver == domain.SwarmNetworkOverlay,
	})
	if err != nil {
		return nil, fmt.Errorf("docker network create: %w", err)
	}

	net.SetDockerNetworkID(dockerID)
	if err := h.networkRepo.Save(ctx, net); err != nil {
		return nil, fmt.Errorf("save network: %w", err)
	}

	_ = h.publisher.Publish(ctx, events.SwarmNetworkCreated.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmNetworkCreated.String(),
		AggregateID: net.ID, OccurredAt: time.Now().UTC(),
	})
	return net, nil
}

func (h *CreateSwarmNetworkHandler) findManagerNode(ctx context.Context, clusterID string) (*domain.SwarmNode, error) {
	nodes, err := h.nodeRepo.FindByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.Role == domain.SwarmNodeManager && n.VpnIP != "" {
			return n, nil
		}
	}
	return nil, fmt.Errorf("no active manager node found in cluster %s", clusterID)
}

// ── DeleteSwarmNetwork ────────────────────────────────────────────────────────

type DeleteSwarmNetworkCommand struct {
	NetworkID string
}

type DeleteSwarmNetworkHandler struct {
	clusterRepo domain.SwarmClusterRepository
	nodeRepo    domain.SwarmNodeRepository
	networkRepo domain.SwarmNetworkRepository
	factory     domain.SwarmClientFactory
	publisher   events.EventPublisher
}

func NewDeleteSwarmNetworkHandler(
	clusterRepo domain.SwarmClusterRepository,
	nodeRepo domain.SwarmNodeRepository,
	networkRepo domain.SwarmNetworkRepository,
	factory domain.SwarmClientFactory,
	pub events.EventPublisher,
) *DeleteSwarmNetworkHandler {
	return &DeleteSwarmNetworkHandler{
		clusterRepo: clusterRepo,
		nodeRepo:    nodeRepo,
		networkRepo: networkRepo,
		factory:     factory,
		publisher:   pub,
	}
}

func (h *DeleteSwarmNetworkHandler) Handle(ctx context.Context, cmd DeleteSwarmNetworkCommand) error {
	net, err := h.networkRepo.FindByID(ctx, cmd.NetworkID)
	if err != nil {
		return err
	}

	if net.DockerNetworkID != "" && net.ClusterID != "" {
		managerNode, err := h.findManagerNode(ctx, net.ClusterID)
		if err == nil {
			client, err := h.factory.ForSwarmNode(managerNode)
			if err == nil {
				_ = client.NetworkRemove(ctx, net.DockerNetworkID)
			}
		}
	}

	if err := h.networkRepo.Delete(ctx, net.ID); err != nil {
		return fmt.Errorf("delete network record: %w", err)
	}

	_ = h.publisher.Publish(ctx, events.SwarmNetworkDeleted.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmNetworkDeleted.String(),
		AggregateID: net.ID, OccurredAt: time.Now().UTC(),
	})
	return nil
}

func (h *DeleteSwarmNetworkHandler) findManagerNode(ctx context.Context, clusterID string) (*domain.SwarmNode, error) {
	nodes, err := h.nodeRepo.FindByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.Role == domain.SwarmNodeManager && n.VpnIP != "" {
			return n, nil
		}
	}
	return nil, fmt.Errorf("no active manager node found in cluster %s", clusterID)
}

// ── UpdateSwarmNetworkReservedIPs ─────────────────────────────────────────────

type UpdateSwarmNetworkReservedIPsCommand struct {
	NetworkID   string
	ReservedIPs []domain.ReservedIP
}

type UpdateSwarmNetworkReservedIPsHandler struct {
	networkRepo domain.SwarmNetworkRepository
}

func NewUpdateSwarmNetworkReservedIPsHandler(networkRepo domain.SwarmNetworkRepository) *UpdateSwarmNetworkReservedIPsHandler {
	return &UpdateSwarmNetworkReservedIPsHandler{networkRepo: networkRepo}
}

func (h *UpdateSwarmNetworkReservedIPsHandler) Handle(ctx context.Context, cmd UpdateSwarmNetworkReservedIPsCommand) (*domain.SwarmNetwork, error) {
	net, err := h.networkRepo.FindByID(ctx, cmd.NetworkID)
	if err != nil {
		return nil, err
	}
	net.UpdateReservedIPs(cmd.ReservedIPs)
	if err := h.networkRepo.Save(ctx, net); err != nil {
		return nil, fmt.Errorf("save network: %w", err)
	}
	return net, nil
}
