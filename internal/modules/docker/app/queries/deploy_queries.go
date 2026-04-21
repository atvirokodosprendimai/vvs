package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// ── RegistryReadModel ─────────────────────────────────────────────────────────

type RegistryReadModel struct {
	ID       string
	Name     string
	URL      string
	Username string
}

// ── ListRegistriesHandler ─────────────────────────────────────────────────────

type ListRegistriesHandler struct {
	repo domain.ContainerRegistryRepository
}

func NewListRegistriesHandler(repo domain.ContainerRegistryRepository) *ListRegistriesHandler {
	return &ListRegistriesHandler{repo: repo}
}

func (h *ListRegistriesHandler) Handle(ctx context.Context) ([]RegistryReadModel, error) {
	all, err := h.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]RegistryReadModel, len(all))
	for i, r := range all {
		result[i] = RegistryReadModel{
			ID:       r.ID,
			Name:     r.Name,
			URL:      r.URL,
			Username: r.Username,
		}
	}
	return result, nil
}

// ── VVSDeploymentReadModel ────────────────────────────────────────────────────

type VVSDeploymentReadModel struct {
	ID             string
	ClusterID      string
	NodeID         string
	Component      string
	Source         string
	ImageURL       string
	RegistryID     string
	GitURL         string
	GitRef         string
	NATSUrl        string
	Port           int
	Status         string
	ErrorMsg       string
	LastDeployedAt string // formatted or empty
}

// ── ListVVSDeploymentsHandler ─────────────────────────────────────────────────

type ListVVSDeploymentsHandler struct {
	repo domain.VVSDeploymentRepository
}

func NewListVVSDeploymentsHandler(repo domain.VVSDeploymentRepository) *ListVVSDeploymentsHandler {
	return &ListVVSDeploymentsHandler{repo: repo}
}

func (h *ListVVSDeploymentsHandler) Handle(ctx context.Context) ([]VVSDeploymentReadModel, error) {
	all, err := h.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]VVSDeploymentReadModel, len(all))
	for i, d := range all {
		rm := VVSDeploymentReadModel{
			ID:         d.ID,
			ClusterID:  d.ClusterID,
			NodeID:     d.NodeID,
			Component:  string(d.Component),
			Source:     string(d.Source),
			ImageURL:   d.ImageURL,
			RegistryID: d.RegistryID,
			GitURL:     d.GitURL,
			GitRef:     d.GitRef,
			NATSUrl:    d.NATSUrl,
			Port:       d.Port,
			Status:     string(d.Status),
			ErrorMsg:   d.ErrorMsg,
		}
		if d.LastDeployedAt != nil {
			rm.LastDeployedAt = d.LastDeployedAt.Format("2006-01-02 15:04")
		}
		result[i] = rm
	}
	return result, nil
}

// ── GetVVSDeploymentHandler ───────────────────────────────────────────────────

type GetVVSDeploymentHandler struct {
	repo domain.VVSDeploymentRepository
}

func NewGetVVSDeploymentHandler(repo domain.VVSDeploymentRepository) *GetVVSDeploymentHandler {
	return &GetVVSDeploymentHandler{repo: repo}
}

func (h *GetVVSDeploymentHandler) Handle(ctx context.Context, id string) (*VVSDeploymentReadModel, error) {
	d, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rm := &VVSDeploymentReadModel{
		ID:         d.ID,
		ClusterID:  d.ClusterID,
		NodeID:     d.NodeID,
		Component:  string(d.Component),
		Source:     string(d.Source),
		ImageURL:   d.ImageURL,
		RegistryID: d.RegistryID,
		GitURL:     d.GitURL,
		GitRef:     d.GitRef,
		NATSUrl:    d.NATSUrl,
		Port:       d.Port,
		Status:     string(d.Status),
		ErrorMsg:   d.ErrorMsg,
	}
	if d.LastDeployedAt != nil {
		rm.LastDeployedAt = d.LastDeployedAt.Format("2006-01-02 15:04")
	}
	return rm, nil
}
