package persistence

import (
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// ── Node model ────────────────────────────────────────────────────────────────

type NodeModel struct {
	ID          string `gorm:"primaryKey;type:text"`
	Name        string `gorm:"type:text;not null"`
	NodeName    string `gorm:"type:text;not null"`
	Host        string `gorm:"type:text;not null"`
	Port        int    `gorm:"not null;default:8006"`
	User        string `gorm:"column:user;type:text;not null"`
	TokenID     string `gorm:"type:text;not null"`
	TokenSecret []byte `gorm:"column:token_secret"` // encrypted bytes
	InsecureTLS bool   `gorm:"not null;default:false"`
	Notes       string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (NodeModel) TableName() string { return "proxmox_nodes" }

func toNodeModel(n *domain.ProxmoxNode) *NodeModel {
	return &NodeModel{
		ID:       n.ID,
		Name:     n.Name,
		NodeName: n.NodeName,
		Host:     n.Host,
		Port:     n.Port,
		User:     n.User,
		TokenID:  n.TokenID,
		// TokenSecret set separately after encryption
		InsecureTLS: n.InsecureTLS,
		Notes:       n.Notes,
		CreatedAt:   n.CreatedAt,
		UpdatedAt:   n.UpdatedAt,
	}
}

func toNodeDomain(m *NodeModel) *domain.ProxmoxNode {
	return &domain.ProxmoxNode{
		ID:          m.ID,
		Name:        m.Name,
		NodeName:    m.NodeName,
		Host:        m.Host,
		Port:        m.Port,
		User:        m.User,
		TokenID:     m.TokenID,
		// TokenSecret set after decryption by repository
		InsecureTLS: m.InsecureTLS,
		Notes:       m.Notes,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// ── VM model ──────────────────────────────────────────────────────────────────

type VMModel struct {
	ID         string `gorm:"primaryKey;type:text"`
	VMID       int    `gorm:"column:vmid;not null"`
	NodeID     string `gorm:"column:node_id;type:text;not null"`
	CustomerID string `gorm:"column:customer_id;type:text"`
	Name       string `gorm:"type:text;not null"`
	Status     string `gorm:"type:text;not null;default:unknown"`
	Cores      int    `gorm:"not null;default:1"`
	MemoryMB   int    `gorm:"column:memory_mb;not null;default:1024"`
	DiskGB     int    `gorm:"column:disk_gb;not null;default:10"`
	IPAddress  string `gorm:"column:ip_address;type:text"`
	Notes      string `gorm:"type:text"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (VMModel) TableName() string { return "proxmox_vms" }

func toVMModel(v *domain.VirtualMachine) *VMModel {
	return &VMModel{
		ID:         v.ID,
		VMID:       v.VMID,
		NodeID:     v.NodeID,
		CustomerID: v.CustomerID,
		Name:       v.Name,
		Status:     string(v.Status),
		Cores:      v.Cores,
		MemoryMB:   v.MemoryMB,
		DiskGB:     v.DiskGB,
		IPAddress:  v.IPAddress,
		Notes:      v.Notes,
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  v.UpdatedAt,
	}
}

func toVMDomain(m *VMModel) *domain.VirtualMachine {
	return &domain.VirtualMachine{
		ID:         m.ID,
		VMID:       m.VMID,
		NodeID:     m.NodeID,
		CustomerID: m.CustomerID,
		Name:       m.Name,
		Status:     domain.VMStatus(m.Status),
		Cores:      m.Cores,
		MemoryMB:   m.MemoryMB,
		DiskGB:     m.DiskGB,
		IPAddress:  m.IPAddress,
		Notes:      m.Notes,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

// ── VM Plan model ─────────────────────────────────────────────────────────────

type VMPlanModel struct {
	ID                    string `gorm:"primaryKey;type:text"`
	Name                  string `gorm:"type:text;not null"`
	Description           string `gorm:"type:text;not null;default:''"`
	Cores                 int    `gorm:"not null"`
	MemoryMB              int    `gorm:"column:memory_mb;not null"`
	DiskGB                int    `gorm:"column:disk_gb;not null"`
	Storage               string `gorm:"type:text;not null;default:'local-lvm'"`
	TemplateVMID          int    `gorm:"column:template_vmid;not null"`
	NodeID                string `gorm:"column:node_id;type:text;not null;default:''"`
	PriceMonthlyEuroCents int64  `gorm:"column:price_monthly_euro_cents;not null;default:0"`
	Enabled               bool   `gorm:"not null;default:true"`
	Notes                 string `gorm:"type:text;not null;default:''"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (VMPlanModel) TableName() string { return "vm_plans" }

func toVMPlanModel(p *domain.VMPlan) *VMPlanModel {
	return &VMPlanModel{
		ID:                    p.ID,
		Name:                  p.Name,
		Description:           p.Description,
		Cores:                 p.Cores,
		MemoryMB:              p.MemoryMB,
		DiskGB:                p.DiskGB,
		Storage:               p.Storage,
		TemplateVMID:          p.TemplateVMID,
		NodeID:                p.NodeID,
		PriceMonthlyEuroCents: p.PriceMonthlyEuroCents,
		Enabled:               p.Enabled,
		Notes:                 p.Notes,
		CreatedAt:             p.CreatedAt,
		UpdatedAt:             p.UpdatedAt,
	}
}

func toVMPlanDomain(m *VMPlanModel) *domain.VMPlan {
	return &domain.VMPlan{
		ID:                    m.ID,
		Name:                  m.Name,
		Description:           m.Description,
		Cores:                 m.Cores,
		MemoryMB:              m.MemoryMB,
		DiskGB:                m.DiskGB,
		Storage:               m.Storage,
		TemplateVMID:          m.TemplateVMID,
		NodeID:                m.NodeID,
		PriceMonthlyEuroCents: m.PriceMonthlyEuroCents,
		Enabled:               m.Enabled,
		Notes:                 m.Notes,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}
}
