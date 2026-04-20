package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/dockerclient"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/hetzner"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/google/uuid"
)

// ── CreateSwarmNode ───────────────────────────────────────────────────────────

type CreateSwarmNodeCommand struct {
	ClusterID       string
	Name            string
	SshHost         string
	SshUser         string
	SshPort         int
	SshKey          []byte
	Role            domain.SwarmNodeRole
	HetznerServerID int // >0 when node was ordered via Hetzner API
}

type CreateSwarmNodeHandler struct {
	nodeRepo domain.SwarmNodeRepository
}

func NewCreateSwarmNodeHandler(nodeRepo domain.SwarmNodeRepository) *CreateSwarmNodeHandler {
	return &CreateSwarmNodeHandler{nodeRepo: nodeRepo}
}

func (h *CreateSwarmNodeHandler) Handle(ctx context.Context, cmd CreateSwarmNodeCommand) (*domain.SwarmNode, error) {
	node, err := domain.NewSwarmNode(cmd.ClusterID, cmd.Name, cmd.SshHost, cmd.SshUser, cmd.SshPort, cmd.Role)
	if err != nil {
		return nil, err
	}
	node.SshKey = cmd.SshKey
	node.HetznerServerID = cmd.HetznerServerID
	if err := h.nodeRepo.Save(ctx, node); err != nil {
		return nil, fmt.Errorf("save node: %w", err)
	}
	return node, nil
}

// ── ProvisionSwarmNode ────────────────────────────────────────────────────────

// ProvisionSwarmNodeCommand deploys wgmesh on a node and captures its VPN IP.
type ProvisionSwarmNodeCommand struct {
	NodeID string
}

// ProvisionSwarmNodeHandler deploys wgmesh compose to the node via SSH,
// polls wgmesh0 until an IP appears (30 s / 2 s interval), and stores it.
type ProvisionSwarmNodeHandler struct {
	nodeRepo    domain.SwarmNodeRepository
	clusterRepo domain.SwarmClusterRepository
	publisher   events.EventPublisher
	progress    func(msg string) // optional SSE callback — nil is OK
}

func NewProvisionSwarmNodeHandler(
	nodeRepo domain.SwarmNodeRepository,
	clusterRepo domain.SwarmClusterRepository,
	pub events.EventPublisher,
) *ProvisionSwarmNodeHandler {
	return &ProvisionSwarmNodeHandler{
		nodeRepo:    nodeRepo,
		clusterRepo: clusterRepo,
		publisher:   pub,
	}
}

// WithProgress registers an SSE progress callback for this request.
// Returns a shallow copy so the original handler remains reusable.
func (h *ProvisionSwarmNodeHandler) WithProgress(fn func(msg string)) *ProvisionSwarmNodeHandler {
	cp := *h
	cp.progress = fn
	return &cp
}

func (h *ProvisionSwarmNodeHandler) emit(msg string) {
	if h.progress != nil {
		h.progress(msg)
	}
}

func (h *ProvisionSwarmNodeHandler) Handle(ctx context.Context, cmd ProvisionSwarmNodeCommand) (*domain.SwarmNode, error) {
	node, err := h.nodeRepo.FindByID(ctx, cmd.NodeID)
	if err != nil {
		return nil, err
	}

	cluster, err := h.clusterRepo.FindByID(ctx, node.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("load cluster: %w", err)
	}

	h.emit("Connecting to node via SSH…")

	// Write compose file and run docker compose up -d via SSH
	composeYAML := domain.RenderWgmeshCompose(cluster.WgmeshKey, node.Name)

	// Escape single quotes in YAML for shell heredoc
	escapedYAML := strings.ReplaceAll(composeYAML, "'", "'\"'\"'")
	deployCmd := fmt.Sprintf(
		"mkdir -p /opt/wgmesh && printf '%%s' '%s' > /opt/wgmesh/docker-compose.yml && docker compose -f /opt/wgmesh/docker-compose.yml up -d",
		escapedYAML,
	)

	h.emit("Deploying wgmesh…")
	_, err = dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, deployCmd)
	if err != nil {
		return nil, fmt.Errorf("deploy wgmesh: %w", err)
	}

	h.emit("Waiting for wgmesh0 interface…")

	// Poll for wgmesh0 IP — 30 s timeout, 2 s interval
	vpnIP, err := h.pollWgmesh0IP(ctx, node, 30*time.Second, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("wgmesh0 not ready: %w", err)
	}

	h.emit(fmt.Sprintf("VPN IP: %s", vpnIP))

	node.SetVpnIP(vpnIP)
	if err := h.nodeRepo.Save(ctx, node); err != nil {
		return nil, fmt.Errorf("save node: %w", err)
	}

	_ = h.publisher.Publish(ctx, events.SwarmNodeProvisioned.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmNodeProvisioned.String(),
		AggregateID: node.ID, OccurredAt: time.Now().UTC(),
	})

	return node, nil
}

// pollWgmesh0IP polls `ip addr show wgmesh0` via SSH until an IPv4 address appears.
func (h *ProvisionSwarmNodeHandler) pollWgmesh0IP(ctx context.Context, node *domain.SwarmNode, timeout, interval time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	cmd := `ip -4 addr show wgmesh0 2>/dev/null | grep -oP 'inet \K[\d.]+'`
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, cmd)
		if err == nil {
			ip := strings.TrimSpace(out)
			if ip != "" {
				return ip, nil
			}
		}
		time.Sleep(interval)
	}
	return "", fmt.Errorf("timed out after %s", timeout)
}

// ── CreateSwarmCluster ────────────────────────────────────────────────────────

type CreateSwarmClusterCommand struct {
	Name      string
	WgmeshKey string
	Notes     string
}

type CreateSwarmClusterHandler struct {
	clusterRepo domain.SwarmClusterRepository
	publisher   events.EventPublisher
}

func NewCreateSwarmClusterHandler(
	clusterRepo domain.SwarmClusterRepository,
	pub events.EventPublisher,
) *CreateSwarmClusterHandler {
	return &CreateSwarmClusterHandler{clusterRepo: clusterRepo, publisher: pub}
}

func (h *CreateSwarmClusterHandler) Handle(ctx context.Context, cmd CreateSwarmClusterCommand) (*domain.SwarmCluster, error) {
	cluster, err := domain.NewSwarmCluster(cmd.Name, cmd.WgmeshKey, cmd.Notes)
	if err != nil {
		return nil, err
	}
	if err := h.clusterRepo.Save(ctx, cluster); err != nil {
		return nil, fmt.Errorf("save cluster: %w", err)
	}
	_ = h.publisher.Publish(ctx, events.SwarmClusterCreated.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmClusterCreated.String(),
		AggregateID: cluster.ID, OccurredAt: time.Now().UTC(),
	})
	return cluster, nil
}

// ── InitSwarm ─────────────────────────────────────────────────────────────────

// InitSwarmCommand initialises a Docker Swarm on the given manager node.
// The node must already have a VpnIP (provisioned wgmesh0).
type InitSwarmCommand struct {
	ClusterID     string
	ManagerNodeID string
}

type InitSwarmHandler struct {
	clusterRepo domain.SwarmClusterRepository
	nodeRepo    domain.SwarmNodeRepository
	factory     domain.SwarmClientFactory
	publisher   events.EventPublisher
	progress    func(msg string)
}

func NewInitSwarmHandler(
	clusterRepo domain.SwarmClusterRepository,
	nodeRepo domain.SwarmNodeRepository,
	factory domain.SwarmClientFactory,
	pub events.EventPublisher,
) *InitSwarmHandler {
	return &InitSwarmHandler{
		clusterRepo: clusterRepo,
		nodeRepo:    nodeRepo,
		factory:     factory,
		publisher:   pub,
	}
}

func (h *InitSwarmHandler) WithProgress(fn func(msg string)) *InitSwarmHandler {
	cp := *h
	cp.progress = fn
	return &cp
}

func (h *InitSwarmHandler) emit(msg string) {
	if h.progress != nil {
		h.progress(msg)
	}
}

func (h *InitSwarmHandler) Handle(ctx context.Context, cmd InitSwarmCommand) (*domain.SwarmCluster, error) {
	cluster, err := h.clusterRepo.FindByID(ctx, cmd.ClusterID)
	if err != nil {
		return nil, err
	}
	node, err := h.nodeRepo.FindByID(ctx, cmd.ManagerNodeID)
	if err != nil {
		return nil, err
	}
	if node.VpnIP == "" {
		return nil, fmt.Errorf("node %s has no VPN IP — provision wgmesh first", node.Name)
	}

	h.emit("Connecting to manager node…")
	client, err := h.factory.ForSwarmNode(node)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	h.emit(fmt.Sprintf("Initialising swarm on %s (%s)…", node.Name, node.VpnIP))
	managerToken, workerToken, err := client.SwarmInit(ctx, node.VpnIP)
	if err != nil {
		return nil, fmt.Errorf("swarm init: %w", err)
	}

	cluster.SetTokens(managerToken, workerToken, node.VpnIP)
	if err := h.clusterRepo.Save(ctx, cluster); err != nil {
		return nil, fmt.Errorf("save cluster: %w", err)
	}

	h.emit("Swarm active — tokens stored")
	_ = h.publisher.Publish(ctx, events.SwarmClusterInitialised.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmClusterInitialised.String(),
		AggregateID: cluster.ID, OccurredAt: time.Now().UTC(),
	})
	return cluster, nil
}

// ── ImportSwarmCluster ────────────────────────────────────────────────────────

// ImportSwarmClusterCommand imports an existing swarm cluster by pasting its tokens.
// No Docker API call is made — the cluster record is saved with the provided data.
type ImportSwarmClusterCommand struct {
	Name          string
	WgmeshKey     string
	ManagerToken  string
	WorkerToken   string
	AdvertiseAddr string
	Notes         string
}

type ImportSwarmClusterHandler struct {
	clusterRepo domain.SwarmClusterRepository
	publisher   events.EventPublisher
}

func NewImportSwarmClusterHandler(
	clusterRepo domain.SwarmClusterRepository,
	pub events.EventPublisher,
) *ImportSwarmClusterHandler {
	return &ImportSwarmClusterHandler{clusterRepo: clusterRepo, publisher: pub}
}

func (h *ImportSwarmClusterHandler) Handle(ctx context.Context, cmd ImportSwarmClusterCommand) (*domain.SwarmCluster, error) {
	cluster, err := domain.NewSwarmCluster(cmd.Name, cmd.WgmeshKey, cmd.Notes)
	if err != nil {
		return nil, err
	}
	cluster.SetTokens(cmd.ManagerToken, cmd.WorkerToken, cmd.AdvertiseAddr)
	if err := h.clusterRepo.Save(ctx, cluster); err != nil {
		return nil, fmt.Errorf("save cluster: %w", err)
	}
	_ = h.publisher.Publish(ctx, events.SwarmClusterImported.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmClusterImported.String(),
		AggregateID: cluster.ID, OccurredAt: time.Now().UTC(),
	})
	return cluster, nil
}

// ── AddSwarmNode (join) ───────────────────────────────────────────────────────

// AddSwarmNodeCommand provisions wgmesh on a new node and joins it to the swarm.
// Role determines which join token is used (manager vs worker).
type AddSwarmNodeCommand struct {
	ClusterID     string
	NodeID        string // must already be created + provisioned (have VpnIP)
}

type AddSwarmNodeHandler struct {
	clusterRepo domain.SwarmClusterRepository
	nodeRepo    domain.SwarmNodeRepository
	factory     domain.SwarmClientFactory
	publisher   events.EventPublisher
	progress    func(msg string)
}

func NewAddSwarmNodeHandler(
	clusterRepo domain.SwarmClusterRepository,
	nodeRepo domain.SwarmNodeRepository,
	factory domain.SwarmClientFactory,
	pub events.EventPublisher,
) *AddSwarmNodeHandler {
	return &AddSwarmNodeHandler{
		clusterRepo: clusterRepo,
		nodeRepo:    nodeRepo,
		factory:     factory,
		publisher:   pub,
	}
}

func (h *AddSwarmNodeHandler) WithProgress(fn func(msg string)) *AddSwarmNodeHandler {
	cp := *h
	cp.progress = fn
	return &cp
}

func (h *AddSwarmNodeHandler) emit(msg string) {
	if h.progress != nil {
		h.progress(msg)
	}
}

func (h *AddSwarmNodeHandler) Handle(ctx context.Context, cmd AddSwarmNodeCommand) (*domain.SwarmNode, error) {
	cluster, err := h.clusterRepo.FindByID(ctx, cmd.ClusterID)
	if err != nil {
		return nil, err
	}
	node, err := h.nodeRepo.FindByID(ctx, cmd.NodeID)
	if err != nil {
		return nil, err
	}
	if node.VpnIP == "" {
		return nil, fmt.Errorf("node %s has no VPN IP — provision wgmesh first", node.Name)
	}

	joinToken := cluster.WorkerToken
	if node.Role == domain.SwarmNodeManager {
		joinToken = cluster.ManagerToken
	}
	if joinToken == "" {
		return nil, fmt.Errorf("cluster has no %s join token — init swarm first", node.Role)
	}

	h.emit(fmt.Sprintf("Joining node %s to swarm as %s…", node.Name, node.Role))

	client, err := h.factory.ForSwarmNode(node)
	if err != nil {
		return nil, fmt.Errorf("create docker client for node: %w", err)
	}

	if err := client.SwarmJoin(ctx, cluster.AdvertiseAddr, joinToken); err != nil {
		return nil, fmt.Errorf("swarm join: %w", err)
	}

	h.emit("Node joined — fetching Docker node ID…")

	// Get the Docker node ID from the manager node
	managerNodes, err := h.nodeRepo.FindByClusterID(ctx, cmd.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("find cluster nodes: %w", err)
	}
	var managerNode *domain.SwarmNode
	for _, n := range managerNodes {
		if n.Role == domain.SwarmNodeManager && n.SwarmNodeID != "" {
			managerNode = n
			break
		}
	}
	if managerNode != nil {
		managerClient, err := h.factory.ForSwarmNode(managerNode)
		if err == nil {
			nodeList, err := managerClient.SwarmNodeList(ctx)
			if err == nil {
				for _, info := range nodeList {
					if info.Hostname == node.Name {
						node.SetSwarmNodeID(info.ID)
						break
					}
				}
			}
		}
	}
	if node.SwarmNodeID == "" {
		// Still active even without Docker node ID — it will be populated later
		node.Status = domain.SwarmNodeActive
	}

	if err := h.nodeRepo.Save(ctx, node); err != nil {
		return nil, fmt.Errorf("save node: %w", err)
	}

	h.emit("Done — node active in swarm")
	_ = h.publisher.Publish(ctx, events.SwarmNodeJoined.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmNodeJoined.String(),
		AggregateID: node.ID, OccurredAt: time.Now().UTC(),
	})
	return node, nil
}

// ── RemoveSwarmNode ───────────────────────────────────────────────────────────

type RemoveSwarmNodeCommand struct {
	NodeID string
}

type RemoveSwarmNodeHandler struct {
	clusterRepo domain.SwarmClusterRepository
	nodeRepo    domain.SwarmNodeRepository
	factory     domain.SwarmClientFactory
	publisher   events.EventPublisher
}

func NewRemoveSwarmNodeHandler(
	clusterRepo domain.SwarmClusterRepository,
	nodeRepo domain.SwarmNodeRepository,
	factory domain.SwarmClientFactory,
	pub events.EventPublisher,
) *RemoveSwarmNodeHandler {
	return &RemoveSwarmNodeHandler{
		clusterRepo: clusterRepo,
		nodeRepo:    nodeRepo,
		factory:     factory,
		publisher:   pub,
	}
}

func (h *RemoveSwarmNodeHandler) Handle(ctx context.Context, cmd RemoveSwarmNodeCommand) error {
	node, err := h.nodeRepo.FindByID(ctx, cmd.NodeID)
	if err != nil {
		return err
	}

	// Leave the swarm on the node itself
	client, err := h.factory.ForSwarmNode(node)
	if err == nil {
		// Force=false for workers; a manager must be demoted first (best-effort)
		_ = client.SwarmLeave(ctx, node.Role == domain.SwarmNodeWorker)
	}

	// Remove from manager's node list if we have a Docker node ID
	if node.SwarmNodeID != "" && node.ClusterID != "" {
		managerNodes, err := h.nodeRepo.FindByClusterID(ctx, node.ClusterID)
		if err == nil {
			for _, n := range managerNodes {
				if n.Role == domain.SwarmNodeManager && n.ID != node.ID {
					managerClient, err := h.factory.ForSwarmNode(n)
					if err == nil {
						_ = managerClient.SwarmNodeRemove(ctx, node.SwarmNodeID)
					}
					break
				}
			}
		}
	}

	if err := h.nodeRepo.Delete(ctx, node.ID); err != nil {
		return fmt.Errorf("delete node record: %w", err)
	}

	// Delete Hetzner VPS if this node was provisioned via Hetzner API
	if node.HetznerServerID > 0 && node.ClusterID != "" {
		if cluster, err := h.clusterRepo.FindByID(ctx, node.ClusterID); err == nil && cluster.HetznerAPIKey != "" {
			_ = hetzner.DeleteServer(ctx, cluster.HetznerAPIKey, node.HetznerServerID)
		}
	}

	_ = h.publisher.Publish(ctx, events.SwarmNodeRemoved.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmNodeRemoved.String(),
		AggregateID: node.ID, OccurredAt: time.Now().UTC(),
	})
	return nil
}

// ── DeleteSwarmCluster ────────────────────────────────────────────────────────

type DeleteSwarmClusterCommand struct {
	ClusterID string
}

type DeleteSwarmClusterHandler struct {
	clusterRepo domain.SwarmClusterRepository
	publisher   events.EventPublisher
}

func NewDeleteSwarmClusterHandler(
	clusterRepo domain.SwarmClusterRepository,
	pub events.EventPublisher,
) *DeleteSwarmClusterHandler {
	return &DeleteSwarmClusterHandler{clusterRepo: clusterRepo, publisher: pub}
}

func (h *DeleteSwarmClusterHandler) Handle(ctx context.Context, cmd DeleteSwarmClusterCommand) error {
	if err := h.clusterRepo.Delete(ctx, cmd.ClusterID); err != nil {
		return err
	}
	_ = h.publisher.Publish(ctx, events.SwarmClusterDeleted.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmClusterDeleted.String(),
		AggregateID: cmd.ClusterID, OccurredAt: time.Now().UTC(),
	})
	return nil
}
