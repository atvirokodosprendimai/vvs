package domain

import (
	"context"
	"errors"
)

// ErrTaskFailed is returned by WaitForTask when Proxmox reports a non-OK exit status.
type ErrTaskFailed struct {
	ExitStatus string
}

func (e ErrTaskFailed) Error() string {
	return "proxmox task failed: " + e.ExitStatus
}

var ErrTaskFailed_ = errors.New("proxmox task failed") // sentinel for errors.Is checks

// VMSpec describes the parameters for creating a new VM by cloning a template.
type VMSpec struct {
	TemplateVMID int    // source template VMID on Proxmox
	NewVMID      int    // target VMID (0 = auto-allocate via NextVMID)
	Name         string
	Storage      string // storage pool, e.g. "local-lvm"
	Cores        int
	MemoryMB     int
	DiskGB       int
	FullClone    bool // full clone vs linked clone
}

// VMInfo is the read model returned from Proxmox for a running VM.
type VMInfo struct {
	VMID     int
	Name     string
	Status   VMStatus
	Cores    int
	MemoryMB int
}

// VMProvisioner is the vendor-agnostic port for Proxmox VM lifecycle operations.
// All mutating operations return a Proxmox UPID task handle.
// Use WaitForTask to poll for completion.
type VMProvisioner interface {
	// NextVMID returns the next available VMID from the Proxmox cluster.
	NextVMID(ctx context.Context, conn NodeConn) (int, error)

	// CreateVM clones a template into a new VM. Returns UPID task handle.
	CreateVM(ctx context.Context, conn NodeConn, spec VMSpec) (upid string, err error)

	// SuspendVM pauses a running VM. Returns UPID task handle.
	SuspendVM(ctx context.Context, conn NodeConn, vmid int) (upid string, err error)

	// StartVM starts a stopped or paused VM. Returns UPID task handle.
	StartVM(ctx context.Context, conn NodeConn, vmid int) (upid string, err error)

	// RestartVM reboots a running VM. Returns UPID task handle.
	RestartVM(ctx context.Context, conn NodeConn, vmid int) (upid string, err error)

	// StopVM performs a hard stop of a VM. Returns UPID task handle.
	StopVM(ctx context.Context, conn NodeConn, vmid int) (upid string, err error)

	// DeleteVM deletes a stopped VM and purges its disks. Returns UPID task handle.
	// The VM must be stopped before calling DeleteVM.
	DeleteVM(ctx context.Context, conn NodeConn, vmid int) (upid string, err error)

	// GetVMInfo returns the current status and config of a VM.
	GetVMInfo(ctx context.Context, conn NodeConn, vmid int) (*VMInfo, error)

	// WaitForTask polls a Proxmox UPID task until completion or context cancellation.
	// Returns ErrTaskFailed if the task exits with a non-OK status.
	WaitForTask(ctx context.Context, conn NodeConn, upid string) error
}
