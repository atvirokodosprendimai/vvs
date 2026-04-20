package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrVMPlanNameRequired     = errors.New("VM plan name is required")
	ErrVMPlanCoresPositive    = errors.New("VM plan cores must be positive")
	ErrVMPlanMemoryPositive   = errors.New("VM plan memory must be positive")
	ErrVMPlanDiskPositive     = errors.New("VM plan disk must be positive")
	ErrVMPlanTemplateRequired = errors.New("VM plan templateVMID must be positive")
	ErrVMPlanNotFound         = errors.New("VM plan not found")
)

// VMPlan is a pre-configured VM offering available for self-service purchase.
type VMPlan struct {
	ID                    string
	Name                  string
	Description           string
	Cores                 int
	MemoryMB              int
	DiskGB                int
	Storage               string // e.g. "local-lvm"
	TemplateVMID          int    // Proxmox template VMID to clone from
	NodeID                string // which Proxmox node to deploy on
	PriceMonthlyEuroCents int64  // price in euro cents (e.g. 500 = €5.00)
	Enabled               bool
	Notes                 string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func NewVMPlan(name, description string, cores, memoryMB, diskGB, templateVMID int, storage, nodeID string, priceMonthlyEuroCents int64, notes string) (*VMPlan, error) {
	if name == "" {
		return nil, ErrVMPlanNameRequired
	}
	if cores <= 0 {
		return nil, ErrVMPlanCoresPositive
	}
	if memoryMB <= 0 {
		return nil, ErrVMPlanMemoryPositive
	}
	if diskGB <= 0 {
		return nil, ErrVMPlanDiskPositive
	}
	if templateVMID <= 0 {
		return nil, ErrVMPlanTemplateRequired
	}
	now := time.Now().UTC()
	return &VMPlan{
		ID:                    uuid.Must(uuid.NewV7()).String(),
		Name:                  name,
		Description:           description,
		Cores:                 cores,
		MemoryMB:              memoryMB,
		DiskGB:                diskGB,
		Storage:               storage,
		TemplateVMID:          templateVMID,
		NodeID:                nodeID,
		PriceMonthlyEuroCents: priceMonthlyEuroCents,
		Enabled:               true,
		Notes:                 notes,
		CreatedAt:             now,
		UpdatedAt:             now,
	}, nil
}

func (p *VMPlan) Update(name, description string, cores, memoryMB, diskGB, templateVMID int, storage, nodeID string, priceMonthlyEuroCents int64, enabled bool, notes string) error {
	if name == "" {
		return ErrVMPlanNameRequired
	}
	if cores <= 0 {
		return ErrVMPlanCoresPositive
	}
	if memoryMB <= 0 {
		return ErrVMPlanMemoryPositive
	}
	if diskGB <= 0 {
		return ErrVMPlanDiskPositive
	}
	if templateVMID <= 0 {
		return ErrVMPlanTemplateRequired
	}
	p.Name = name
	p.Description = description
	p.Cores = cores
	p.MemoryMB = memoryMB
	p.DiskGB = diskGB
	p.Storage = storage
	p.TemplateVMID = templateVMID
	p.NodeID = nodeID
	p.PriceMonthlyEuroCents = priceMonthlyEuroCents
	p.Enabled = enabled
	p.Notes = notes
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// VMPlanRepository persists VM plans.
type VMPlanRepository interface {
	Save(ctx context.Context, plan *VMPlan) error
	FindByID(ctx context.Context, id string) (*VMPlan, error)
	FindAll(ctx context.Context) ([]*VMPlan, error)
	FindEnabled(ctx context.Context) ([]*VMPlan, error)
	Delete(ctx context.Context, id string) error
}
