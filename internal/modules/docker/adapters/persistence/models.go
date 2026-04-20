package persistence

import (
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// ── DockerNode model ─────────────────────────────────────────────────────────

type DockerNodeModel struct {
	ID        string `gorm:"primaryKey;type:text"`
	Name      string `gorm:"type:text;not null"`
	Host      string `gorm:"type:text;not null"`
	IsLocal   bool   `gorm:"column:is_local;not null;default:false"`
	TLSCert   []byte `gorm:"column:tls_cert"`
	TLSKey    []byte `gorm:"column:tls_key"`
	TLSCA     []byte `gorm:"column:tls_ca"`
	Notes     string `gorm:"type:text;not null;default:''"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (DockerNodeModel) TableName() string { return "docker_nodes" }

func toNodeModel(n *domain.DockerNode) *DockerNodeModel {
	return &DockerNodeModel{
		ID:        n.ID,
		Name:      n.Name,
		Host:      n.Host,
		IsLocal:   n.IsLocal,
		Notes:     n.Notes,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

func toNodeDomain(m *DockerNodeModel) *domain.DockerNode {
	return &domain.DockerNode{
		ID:        m.ID,
		Name:      m.Name,
		Host:      m.Host,
		IsLocal:   m.IsLocal,
		Notes:     m.Notes,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// ── DockerService model ──────────────────────────────────────────────────────

type DockerServiceModel struct {
	ID          string `gorm:"primaryKey;type:text"`
	NodeID      string `gorm:"column:node_id;type:text;not null"`
	Name        string `gorm:"type:text;not null"`
	ComposeYAML string `gorm:"column:compose_yaml;type:text;not null;default:''"`
	Status      string `gorm:"type:text;not null;default:'stopped'"`
	ErrorMsg    string `gorm:"column:error_msg;type:text;not null;default:''"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (DockerServiceModel) TableName() string { return "docker_services" }

func toServiceModel(s *domain.DockerService) *DockerServiceModel {
	return &DockerServiceModel{
		ID:          s.ID,
		NodeID:      s.NodeID,
		Name:        s.Name,
		ComposeYAML: s.ComposeYAML,
		Status:      string(s.Status),
		ErrorMsg:    s.ErrorMsg,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func toServiceDomain(m *DockerServiceModel) *domain.DockerService {
	return &domain.DockerService{
		ID:          m.ID,
		NodeID:      m.NodeID,
		Name:        m.Name,
		ComposeYAML: m.ComposeYAML,
		Status:      domain.ServiceStatus(m.Status),
		ErrorMsg:    m.ErrorMsg,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
