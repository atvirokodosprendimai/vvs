package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// ── Swarm read models ─────────────────────────────────────────────────────────

type SwarmClusterReadModel struct {
	ID            string
	Name          string
	Status        string
	AdvertiseAddr string
	Notes         string
	NodeCount     int
	HasHetzner    bool   // true when HetznerAPIKey + HetznerSSHKeyID are configured
	SSHPublicKey  string // cluster-level public key (shown in UI for Hetzner registration)
}

type SwarmNodeReadModel struct {
	ID          string
	ClusterID   string
	Name        string
	Role        string
	Status      string
	SshHost     string
	VpnIP       string
	SwarmNodeID string
}

type SwarmNetworkReadModel struct {
	ID              string
	ClusterID       string
	Name            string
	Driver          string
	Subnet          string
	Gateway         string
	DhcpRangeStart  string
	DhcpRangeEnd    string
	DockerNetworkID string
	ReservedIPs     []domain.ReservedIP
}

type SwarmStackReadModel struct {
	ID          string
	ClusterID   string
	Name        string
	Status      string
	ErrorMsg    string
	ComposeYAML string
	Routes      []SwarmRouteReadModel
}

type SwarmRouteReadModel struct {
	ID          string
	Hostname    string
	Port        int
	StripPrefix bool
}

// ── Cluster queries ───────────────────────────────────────────────────────────

type ListSwarmClustersHandler struct {
	clusterRepo domain.SwarmClusterRepository
	nodeRepo    domain.SwarmNodeRepository
}

func NewListSwarmClustersHandler(clusterRepo domain.SwarmClusterRepository, nodeRepo domain.SwarmNodeRepository) *ListSwarmClustersHandler {
	return &ListSwarmClustersHandler{clusterRepo: clusterRepo, nodeRepo: nodeRepo}
}

func (h *ListSwarmClustersHandler) Handle(ctx context.Context) ([]SwarmClusterReadModel, error) {
	clusters, err := h.clusterRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SwarmClusterReadModel, len(clusters))
	for i, c := range clusters {
		nodes, _ := h.nodeRepo.FindByClusterID(ctx, c.ID)
		out[i] = SwarmClusterReadModel{
			ID:            c.ID,
			Name:          c.Name,
			Status:        string(c.Status),
			AdvertiseAddr: c.AdvertiseAddr,
			Notes:         c.Notes,
			NodeCount:     len(nodes),
			HasHetzner:    c.HasHetzner(),
			SSHPublicKey:  c.SSHPublicKey,
		}
	}
	return out, nil
}

type GetSwarmClusterHandler struct {
	clusterRepo domain.SwarmClusterRepository
}

func NewGetSwarmClusterHandler(repo domain.SwarmClusterRepository) *GetSwarmClusterHandler {
	return &GetSwarmClusterHandler{clusterRepo: repo}
}

func (h *GetSwarmClusterHandler) Handle(ctx context.Context, id string) (*SwarmClusterReadModel, error) {
	c, err := h.clusterRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &SwarmClusterReadModel{
		ID:            c.ID,
		Name:          c.Name,
		Status:        string(c.Status),
		AdvertiseAddr: c.AdvertiseAddr,
		Notes:         c.Notes,
		HasHetzner:    c.HasHetzner(),
		SSHPublicKey:  c.SSHPublicKey,
	}, nil
}

// ── Node queries ──────────────────────────────────────────────────────────────

type ListSwarmNodesHandler struct{ nodeRepo domain.SwarmNodeRepository }

func NewListSwarmNodesHandler(repo domain.SwarmNodeRepository) *ListSwarmNodesHandler {
	return &ListSwarmNodesHandler{nodeRepo: repo}
}

func (h *ListSwarmNodesHandler) Handle(ctx context.Context, clusterID string) ([]SwarmNodeReadModel, error) {
	var nodes []*domain.SwarmNode
	var err error
	if clusterID != "" {
		nodes, err = h.nodeRepo.FindByClusterID(ctx, clusterID)
	} else {
		nodes, err = h.nodeRepo.FindAll(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]SwarmNodeReadModel, len(nodes))
	for i, n := range nodes {
		out[i] = SwarmNodeReadModel{
			ID:          n.ID,
			ClusterID:   n.ClusterID,
			Name:        n.Name,
			Role:        string(n.Role),
			Status:      string(n.Status),
			SshHost:     n.SshHost,
			VpnIP:       n.VpnIP,
			SwarmNodeID: n.SwarmNodeID,
		}
	}
	return out, nil
}

type GetSwarmNodeHandler struct{ nodeRepo domain.SwarmNodeRepository }

func NewGetSwarmNodeHandler(repo domain.SwarmNodeRepository) *GetSwarmNodeHandler {
	return &GetSwarmNodeHandler{nodeRepo: repo}
}

func (h *GetSwarmNodeHandler) Handle(ctx context.Context, id string) (*SwarmNodeReadModel, error) {
	n, err := h.nodeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &SwarmNodeReadModel{
		ID:          n.ID,
		ClusterID:   n.ClusterID,
		Name:        n.Name,
		Role:        string(n.Role),
		Status:      string(n.Status),
		SshHost:     n.SshHost,
		VpnIP:       n.VpnIP,
		SwarmNodeID: n.SwarmNodeID,
	}, nil
}

// ── Network queries ───────────────────────────────────────────────────────────

type ListSwarmNetworksHandler struct{ networkRepo domain.SwarmNetworkRepository }

func NewListSwarmNetworksHandler(repo domain.SwarmNetworkRepository) *ListSwarmNetworksHandler {
	return &ListSwarmNetworksHandler{networkRepo: repo}
}

func (h *ListSwarmNetworksHandler) Handle(ctx context.Context, clusterID string) ([]SwarmNetworkReadModel, error) {
	var networks []*domain.SwarmNetwork
	var err error
	if clusterID != "" {
		networks, err = h.networkRepo.FindByClusterID(ctx, clusterID)
	} else {
		networks, err = h.networkRepo.FindAll(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]SwarmNetworkReadModel, len(networks))
	for i, n := range networks {
		out[i] = toNetworkReadModel(n)
	}
	return out, nil
}

type GetSwarmNetworkHandler struct{ networkRepo domain.SwarmNetworkRepository }

func NewGetSwarmNetworkHandler(repo domain.SwarmNetworkRepository) *GetSwarmNetworkHandler {
	return &GetSwarmNetworkHandler{networkRepo: repo}
}

func (h *GetSwarmNetworkHandler) Handle(ctx context.Context, id string) (*SwarmNetworkReadModel, error) {
	n, err := h.networkRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m := toNetworkReadModel(n)
	return &m, nil
}

func toNetworkReadModel(n *domain.SwarmNetwork) SwarmNetworkReadModel {
	return SwarmNetworkReadModel{
		ID:              n.ID,
		ClusterID:       n.ClusterID,
		Name:            n.Name,
		Driver:          string(n.Driver),
		Subnet:          n.Subnet,
		Gateway:         n.Gateway,
		DhcpRangeStart:  n.DhcpRangeStart,
		DhcpRangeEnd:    n.DhcpRangeEnd,
		DockerNetworkID: n.DockerNetworkID,
		ReservedIPs:     n.ReservedIPs,
	}
}

// ── Stack queries ─────────────────────────────────────────────────────────────

type ListSwarmStacksHandler struct{ stackRepo domain.SwarmStackRepository }

func NewListSwarmStacksHandler(repo domain.SwarmStackRepository) *ListSwarmStacksHandler {
	return &ListSwarmStacksHandler{stackRepo: repo}
}

func (h *ListSwarmStacksHandler) Handle(ctx context.Context, clusterID string) ([]SwarmStackReadModel, error) {
	stacks, err := h.stackRepo.FindByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	out := make([]SwarmStackReadModel, len(stacks))
	for i, s := range stacks {
		out[i] = SwarmStackReadModel{
			ID:          s.ID,
			ClusterID:   s.ClusterID,
			Name:        s.Name,
			Status:      string(s.Status),
			ErrorMsg:    s.ErrorMsg,
			ComposeYAML: s.ComposeYAML,
		}
	}
	return out, nil
}

type GetSwarmStackHandler struct{ stackRepo domain.SwarmStackRepository }

func NewGetSwarmStackHandler(repo domain.SwarmStackRepository) *GetSwarmStackHandler {
	return &GetSwarmStackHandler{stackRepo: repo}
}

func (h *GetSwarmStackHandler) Handle(ctx context.Context, id string) (*SwarmStackReadModel, error) {
	s, err := h.stackRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	routes, _ := h.stackRepo.FindRoutesByStackID(ctx, id)
	routeModels := make([]SwarmRouteReadModel, len(routes))
	for i, r := range routes {
		routeModels[i] = SwarmRouteReadModel{
			ID:          r.ID,
			Hostname:    r.Hostname,
			Port:        r.Port,
			StripPrefix: r.StripPrefix,
		}
	}
	return &SwarmStackReadModel{
		ID:          s.ID,
		ClusterID:   s.ClusterID,
		Name:        s.Name,
		Status:      string(s.Status),
		ErrorMsg:    s.ErrorMsg,
		ComposeYAML: s.ComposeYAML,
		Routes:      routeModels,
	}, nil
}
