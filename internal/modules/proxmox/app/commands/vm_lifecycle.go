// Package commands implements VM lifecycle operations for Proxmox VMs.
// All mutating operations (Suspend, Resume, Restart, Delete) follow the same pattern:
//  1. Load VM from repo, validate preconditions
//  2. Update status to transitional state (optimistic)
//  3. Launch background goroutine → provisioner call → WaitForTask → publish event
//  4. Return immediately (handler is non-blocking)
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// ── Suspend ───────────────────────────────────────────────────────────────────

type SuspendVMCommand struct{ ID string }

type SuspendVMHandler struct {
	nodeRepo    domain.NodeRepository
	vmRepo      domain.VMRepository
	provisioner domain.VMProvisioner
	publisher   events.EventPublisher
}

func NewSuspendVMHandler(nodeRepo domain.NodeRepository, vmRepo domain.VMRepository, provisioner domain.VMProvisioner, pub events.EventPublisher) *SuspendVMHandler {
	return &SuspendVMHandler{nodeRepo: nodeRepo, vmRepo: vmRepo, provisioner: provisioner, publisher: pub}
}

func (h *SuspendVMHandler) Handle(ctx context.Context, cmd SuspendVMCommand) error {
	vm, conn, err := loadVMWithNode(ctx, cmd.ID, h.vmRepo, h.nodeRepo)
	if err != nil {
		return err
	}
	if err := vm.Suspend(); err != nil {
		return err
	}
	if err := h.vmRepo.UpdateStatus(ctx, vm.ID, vm.Status); err != nil {
		return fmt.Errorf("update vm status: %w", err)
	}
	vmid := vm.VMID
	go runAsyncOp(h.vmRepo, h.provisioner, h.publisher, vm.ID, domain.VMStatusPaused, domain.VMStatusRunning,
		events.ProxmoxVMSuspended.String(), "proxmox.vm.suspended",
		func(ctx context.Context) error {
			upid, err := h.provisioner.SuspendVM(ctx, conn, vmid)
			if err != nil {
				return err
			}
			return h.provisioner.WaitForTask(ctx, conn, upid)
		})
	return nil
}

// ── Resume ────────────────────────────────────────────────────────────────────

type ResumeVMCommand struct{ ID string }

type ResumeVMHandler struct {
	nodeRepo    domain.NodeRepository
	vmRepo      domain.VMRepository
	provisioner domain.VMProvisioner
	publisher   events.EventPublisher
}

func NewResumeVMHandler(nodeRepo domain.NodeRepository, vmRepo domain.VMRepository, provisioner domain.VMProvisioner, pub events.EventPublisher) *ResumeVMHandler {
	return &ResumeVMHandler{nodeRepo: nodeRepo, vmRepo: vmRepo, provisioner: provisioner, publisher: pub}
}

func (h *ResumeVMHandler) Handle(ctx context.Context, cmd ResumeVMCommand) error {
	vm, conn, err := loadVMWithNode(ctx, cmd.ID, h.vmRepo, h.nodeRepo)
	if err != nil {
		return err
	}
	if err := vm.Resume(); err != nil {
		return err
	}
	if err := h.vmRepo.UpdateStatus(ctx, vm.ID, vm.Status); err != nil {
		return fmt.Errorf("update vm status: %w", err)
	}
	vmid := vm.VMID
	go runAsyncOp(h.vmRepo, h.provisioner, h.publisher, vm.ID, domain.VMStatusRunning, domain.VMStatusPaused,
		events.ProxmoxVMResumed.String(), "proxmox.vm.resumed",
		func(ctx context.Context) error {
			upid, err := h.provisioner.StartVM(ctx, conn, vmid)
			if err != nil {
				return err
			}
			return h.provisioner.WaitForTask(ctx, conn, upid)
		})
	return nil
}

// ── Restart ───────────────────────────────────────────────────────────────────

type RestartVMCommand struct{ ID string }

type RestartVMHandler struct {
	nodeRepo    domain.NodeRepository
	vmRepo      domain.VMRepository
	provisioner domain.VMProvisioner
	publisher   events.EventPublisher
}

func NewRestartVMHandler(nodeRepo domain.NodeRepository, vmRepo domain.VMRepository, provisioner domain.VMProvisioner, pub events.EventPublisher) *RestartVMHandler {
	return &RestartVMHandler{nodeRepo: nodeRepo, vmRepo: vmRepo, provisioner: provisioner, publisher: pub}
}

func (h *RestartVMHandler) Handle(ctx context.Context, cmd RestartVMCommand) error {
	vm, conn, err := loadVMWithNode(ctx, cmd.ID, h.vmRepo, h.nodeRepo)
	if err != nil {
		return err
	}
	if vm.Status != domain.VMStatusRunning {
		return domain.ErrVMNotRunning
	}
	// Status stays "running" through reboot.
	vmid := vm.VMID
	go runAsyncOp(h.vmRepo, h.provisioner, h.publisher, vm.ID, domain.VMStatusRunning, domain.VMStatusRunning,
		events.ProxmoxVMRestarted.String(), "proxmox.vm.restarted",
		func(ctx context.Context) error {
			upid, err := h.provisioner.RestartVM(ctx, conn, vmid)
			if err != nil {
				return err
			}
			return h.provisioner.WaitForTask(ctx, conn, upid)
		})
	return nil
}

// ── Delete ────────────────────────────────────────────────────────────────────

type DeleteVMCommand struct{ ID string }

type DeleteVMHandler struct {
	nodeRepo    domain.NodeRepository
	vmRepo      domain.VMRepository
	provisioner domain.VMProvisioner
	publisher   events.EventPublisher
}

func NewDeleteVMHandler(nodeRepo domain.NodeRepository, vmRepo domain.VMRepository, provisioner domain.VMProvisioner, pub events.EventPublisher) *DeleteVMHandler {
	return &DeleteVMHandler{nodeRepo: nodeRepo, vmRepo: vmRepo, provisioner: provisioner, publisher: pub}
}

func (h *DeleteVMHandler) Handle(ctx context.Context, cmd DeleteVMCommand) error {
	vm, conn, err := loadVMWithNode(ctx, cmd.ID, h.vmRepo, h.nodeRepo)
	if err != nil {
		return err
	}
	prevStatus := vm.Status
	if err := vm.MarkDeleting(); err != nil {
		return err
	}
	if err := h.vmRepo.UpdateStatus(ctx, vm.ID, domain.VMStatusDeleting); err != nil {
		return fmt.Errorf("update vm status: %w", err)
	}
	vmID := vm.ID
	vmid := vm.VMID
	go h.runDelete(vmID, vmid, prevStatus, conn)
	return nil
}

func (h *DeleteVMHandler) runDelete(vmID string, vmid int, prevStatus domain.VMStatus, conn domain.NodeConn) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Stop VM first if it was running or paused.
	if prevStatus == domain.VMStatusRunning || prevStatus == domain.VMStatusPaused {
		upid, err := h.provisioner.StopVM(ctx, conn, vmid)
		if err != nil {
			slog.Error("proxmox: stop VM before delete failed", "vmID", vmID, "err", err)
			h.vmRepo.UpdateStatus(ctx, vmID, prevStatus) //nolint
			return
		}
		if err := h.provisioner.WaitForTask(ctx, conn, upid); err != nil {
			slog.Error("proxmox: stop VM task failed", "vmID", vmID, "err", err)
			h.vmRepo.UpdateStatus(ctx, vmID, prevStatus) //nolint
			return
		}
	}

	upid, err := h.provisioner.DeleteVM(ctx, conn, vmid)
	if err != nil {
		slog.Error("proxmox: delete VM failed", "vmID", vmID, "err", err)
		h.vmRepo.UpdateStatus(ctx, vmID, prevStatus) //nolint
		return
	}
	if err := h.provisioner.WaitForTask(ctx, conn, upid); err != nil {
		slog.Error("proxmox: delete VM task failed", "vmID", vmID, "err", err)
		h.vmRepo.UpdateStatus(ctx, vmID, prevStatus) //nolint
		return
	}

	h.vmRepo.Delete(ctx, vmID) //nolint
	publishEvent(h.publisher, ctx, events.ProxmoxVMDeleted.String(), "proxmox.vm.deleted", vmID)
}

// ── Assign customer ───────────────────────────────────────────────────────────

type AssignVMCustomerCommand struct {
	VMID       string
	CustomerID string // empty = unassign
}

type AssignVMCustomerHandler struct {
	vmRepo    domain.VMRepository
	publisher events.EventPublisher
}

func NewAssignVMCustomerHandler(vmRepo domain.VMRepository, pub events.EventPublisher) *AssignVMCustomerHandler {
	return &AssignVMCustomerHandler{vmRepo: vmRepo, publisher: pub}
}

func (h *AssignVMCustomerHandler) Handle(ctx context.Context, cmd AssignVMCustomerCommand) error {
	vm, err := h.vmRepo.FindByID(ctx, cmd.VMID)
	if err != nil {
		return err
	}
	if cmd.CustomerID == "" {
		vm.UnassignCustomer()
	} else {
		vm.AssignCustomer(cmd.CustomerID)
	}
	if err := h.vmRepo.Save(ctx, vm); err != nil {
		return err
	}
	publishEvent(h.publisher, ctx, events.ProxmoxVMStatusChanged.String(), "proxmox.vm.status_changed", vm.ID)
	return nil
}

// ── shared helpers ────────────────────────────────────────────────────────────

func loadVMWithNode(ctx context.Context, id string, vmRepo domain.VMRepository, nodeRepo domain.NodeRepository) (*domain.VirtualMachine, domain.NodeConn, error) {
	vm, err := vmRepo.FindByID(ctx, id)
	if err != nil {
		return nil, domain.NodeConn{}, err
	}
	node, err := nodeRepo.FindByID(ctx, vm.NodeID)
	if err != nil {
		return nil, domain.NodeConn{}, fmt.Errorf("load VM node: %w", err)
	}
	return vm, node.ToConn(), nil
}

// runAsyncOp executes an async provisioner operation in a background goroutine.
// op must include both the provisioner call and WaitForTask.
// On success: sets successStatus + publishes event.
// On failure: reverts to revertStatus.
func runAsyncOp(
	vmRepo domain.VMRepository,
	_ domain.VMProvisioner, // reserved for future use (e.g. status polling)
	pub events.EventPublisher,
	vmID string,
	successStatus, revertStatus domain.VMStatus,
	subject, eventType string,
	op func(context.Context) error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := op(ctx); err != nil {
		slog.Error("proxmox: async vm op failed", "vmID", vmID, "event", eventType, "err", err)
		vmRepo.UpdateStatus(ctx, vmID, revertStatus) //nolint
		return
	}
	vmRepo.UpdateStatus(ctx, vmID, successStatus) //nolint
	publishEvent(pub, ctx, subject, eventType, vmID)
}

func publishEvent(pub events.EventPublisher, ctx context.Context, subject, eventType, vmID string) {
	data, _ := json.Marshal(map[string]string{"id": vmID})
	pub.Publish(ctx, subject, events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        eventType,
		AggregateID: vmID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
}
