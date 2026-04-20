package queries

import (
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// NodeReadModel is the query-side view of a Proxmox node.
// Never includes TokenSecret.
type NodeReadModel struct {
	ID          string
	Name        string
	NodeName    string
	Host        string
	Port        int
	User        string
	TokenID     string
	InsecureTLS bool
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func nodeToReadModel(n *domain.ProxmoxNode) NodeReadModel {
	return NodeReadModel{
		ID:          n.ID,
		Name:        n.Name,
		NodeName:    n.NodeName,
		Host:        n.Host,
		Port:        n.Port,
		User:        n.User,
		TokenID:     n.TokenID,
		InsecureTLS: n.InsecureTLS,
		Notes:       n.Notes,
		CreatedAt:   n.CreatedAt,
		UpdatedAt:   n.UpdatedAt,
	}
}

// VMReadModel is the query-side view of a virtual machine.
type VMReadModel struct {
	ID         string
	VMID       int
	NodeID     string
	NodeName   string // joined from node
	CustomerID string
	Name       string
	Status     domain.VMStatus
	Cores      int
	MemoryMB   int
	DiskGB     int
	IPAddress  string
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// VMPlanReadModel is the query-side view of a VM plan.
type VMPlanReadModel struct {
	ID                    string
	Name                  string
	Description           string
	Cores                 int
	MemoryMB              int
	DiskGB                int
	Storage               string
	TemplateVMID          int
	NodeID                string
	PriceMonthlyEuroCents int64
	Enabled               bool
	Notes                 string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func vmPlanToReadModel(p *domain.VMPlan) VMPlanReadModel {
	return VMPlanReadModel{
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

func vmToReadModel(vm *domain.VirtualMachine) VMReadModel {
	return VMReadModel{
		ID:         vm.ID,
		VMID:       vm.VMID,
		NodeID:     vm.NodeID,
		CustomerID: vm.CustomerID,
		Name:       vm.Name,
		Status:     vm.Status,
		Cores:      vm.Cores,
		MemoryMB:   vm.MemoryMB,
		DiskGB:     vm.DiskGB,
		IPAddress:  vm.IPAddress,
		Notes:      vm.Notes,
		CreatedAt:  vm.CreatedAt,
		UpdatedAt:  vm.UpdatedAt,
	}
}
