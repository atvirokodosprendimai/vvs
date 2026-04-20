package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// VMStatus represents the current state of a virtual machine.
type VMStatus string

const (
	VMStatusRunning  VMStatus = "running"
	VMStatusStopped  VMStatus = "stopped"
	VMStatusPaused   VMStatus = "paused"
	VMStatusCreating VMStatus = "creating"
	VMStatusDeleting VMStatus = "deleting"
	VMStatusUnknown  VMStatus = "unknown"
)

var (
	ErrVMNameRequired  = errors.New("VM name is required")
	ErrVMIDRequired    = errors.New("VMID must be positive")
	ErrCoresPositive   = errors.New("cores must be positive")
	ErrMemoryPositive  = errors.New("memory must be positive")
	ErrVMNotFound      = errors.New("VM not found")
	ErrVMNotRunning    = errors.New("VM is not running")
	ErrVMNotPaused     = errors.New("VM is not paused")
	ErrVMNotStopped    = errors.New("VM must be stopped before deletion")
	ErrVMAlreadyDeleting = errors.New("VM deletion already in progress")
)

// VirtualMachine represents a Proxmox QEMU VM tracked in our system.
// CustomerID is optional — empty string means unassigned.
type VirtualMachine struct {
	ID         string
	VMID       int      // Proxmox numeric VMID (100–999999)
	NodeID     string   // FK → ProxmoxNode.ID
	CustomerID string   // FK → Customer.ID; empty = unassigned
	Name       string
	Status     VMStatus
	Cores      int
	MemoryMB   int
	DiskGB     int
	IPAddress  string
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewVirtualMachine(vmid int, nodeID, customerID, name string, cores, memoryMB, diskGB int, notes string) (*VirtualMachine, error) {
	if vmid <= 0 {
		return nil, ErrVMIDRequired
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrVMNameRequired
	}
	if cores <= 0 {
		return nil, ErrCoresPositive
	}
	if memoryMB <= 0 {
		return nil, ErrMemoryPositive
	}
	if diskGB <= 0 {
		diskGB = 10 // sensible default
	}

	now := time.Now().UTC()
	return &VirtualMachine{
		ID:         uuid.Must(uuid.NewV7()).String(),
		VMID:       vmid,
		NodeID:     nodeID,
		CustomerID: strings.TrimSpace(customerID),
		Name:       name,
		Status:     VMStatusCreating,
		Cores:      cores,
		MemoryMB:   memoryMB,
		DiskGB:     diskGB,
		Notes:      strings.TrimSpace(notes),
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (v *VirtualMachine) MarkRunning() {
	v.Status = VMStatusRunning
	v.UpdatedAt = time.Now().UTC()
}

func (v *VirtualMachine) MarkStopped() {
	v.Status = VMStatusStopped
	v.UpdatedAt = time.Now().UTC()
}

func (v *VirtualMachine) Suspend() error {
	if v.Status != VMStatusRunning {
		return ErrVMNotRunning
	}
	v.Status = VMStatusPaused
	v.UpdatedAt = time.Now().UTC()
	return nil
}

func (v *VirtualMachine) Resume() error {
	if v.Status != VMStatusPaused {
		return ErrVMNotPaused
	}
	v.Status = VMStatusRunning
	v.UpdatedAt = time.Now().UTC()
	return nil
}

func (v *VirtualMachine) MarkDeleting() error {
	if v.Status == VMStatusDeleting {
		return ErrVMAlreadyDeleting
	}
	v.Status = VMStatusDeleting
	v.UpdatedAt = time.Now().UTC()
	return nil
}

func (v *VirtualMachine) AssignCustomer(customerID string) {
	v.CustomerID = strings.TrimSpace(customerID)
	v.UpdatedAt = time.Now().UTC()
}

func (v *VirtualMachine) UnassignCustomer() {
	v.CustomerID = ""
	v.UpdatedAt = time.Now().UTC()
}

func (v *VirtualMachine) SetIPAddress(ip string) {
	v.IPAddress = strings.TrimSpace(ip)
	v.UpdatedAt = time.Now().UTC()
}
