package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// ── Read models ───────────────────────────────────────────────────────────────

type NodeReadModel struct {
	ID      string
	Name    string
	Host    string
	IsLocal bool
	HasTLS  bool // true when TLS cert is configured
	Notes   string
}

type ServiceReadModel struct {
	ID          string
	NodeID      string
	NodeName    string
	Name        string
	Status      string
	ErrorMsg    string
	ComposeYAML string
}

// ── Node queries ──────────────────────────────────────────────────────────────

type ListNodesHandler struct{ repo domain.DockerNodeRepository }

func NewListNodesHandler(repo domain.DockerNodeRepository) *ListNodesHandler {
	return &ListNodesHandler{repo: repo}
}

func (h *ListNodesHandler) Handle(ctx context.Context) ([]NodeReadModel, error) {
	nodes, err := h.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]NodeReadModel, len(nodes))
	for i, n := range nodes {
		out[i] = toNodeRM(n)
	}
	return out, nil
}

type GetNodeHandler struct{ repo domain.DockerNodeRepository }

func NewGetNodeHandler(repo domain.DockerNodeRepository) *GetNodeHandler {
	return &GetNodeHandler{repo: repo}
}

func (h *GetNodeHandler) Handle(ctx context.Context, id string) (*NodeReadModel, error) {
	n, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rm := toNodeRM(n)
	return &rm, nil
}

// ── Service queries ───────────────────────────────────────────────────────────

type ListServicesHandler struct {
	serviceRepo domain.DockerServiceRepository
	nodeRepo    domain.DockerNodeRepository
}

func NewListServicesHandler(serviceRepo domain.DockerServiceRepository, nodeRepo domain.DockerNodeRepository) *ListServicesHandler {
	return &ListServicesHandler{serviceRepo: serviceRepo, nodeRepo: nodeRepo}
}

func (h *ListServicesHandler) Handle(ctx context.Context) ([]ServiceReadModel, error) {
	svcs, err := h.serviceRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	// build node name lookup
	nodes, _ := h.nodeRepo.FindAll(ctx)
	nodeNames := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeNames[n.ID] = n.Name
	}
	out := make([]ServiceReadModel, len(svcs))
	for i, s := range svcs {
		out[i] = toServiceRM(s, nodeNames[s.NodeID])
	}
	return out, nil
}

type GetServiceHandler struct {
	serviceRepo domain.DockerServiceRepository
	nodeRepo    domain.DockerNodeRepository
}

func NewGetServiceHandler(serviceRepo domain.DockerServiceRepository, nodeRepo domain.DockerNodeRepository) *GetServiceHandler {
	return &GetServiceHandler{serviceRepo: serviceRepo, nodeRepo: nodeRepo}
}

func (h *GetServiceHandler) Handle(ctx context.Context, id string) (*ServiceReadModel, error) {
	svc, err := h.serviceRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	nodeName := ""
	if n, err := h.nodeRepo.FindByID(ctx, svc.NodeID); err == nil {
		nodeName = n.Name
	}
	rm := toServiceRM(svc, nodeName)
	return &rm, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func toNodeRM(n *domain.DockerNode) NodeReadModel {
	return NodeReadModel{
		ID:      n.ID,
		Name:    n.Name,
		Host:    n.Host,
		IsLocal: n.IsLocal,
		HasTLS:  len(n.TLSCert) > 0,
		Notes:   n.Notes,
	}
}

func toServiceRM(s *domain.DockerService, nodeName string) ServiceReadModel {
	return ServiceReadModel{
		ID:          s.ID,
		NodeID:      s.NodeID,
		NodeName:    nodeName,
		Name:        s.Name,
		Status:      string(s.Status),
		ErrorMsg:    s.ErrorMsg,
		ComposeYAML: s.ComposeYAML,
	}
}
