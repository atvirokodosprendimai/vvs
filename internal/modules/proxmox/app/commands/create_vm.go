package commands

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type CreateVMCommand struct {
	NodeID       string
	CustomerID   string
	Name         string
	TemplateVMID int
	Storage      string
	Cores        int
	MemoryMB     int
	DiskGB       int
	FullClone    bool
	Notes        string
}

type CreateVMHandler struct {
	nodeRepo    domain.NodeRepository
	vmRepo      domain.VMRepository
	provisioner domain.VMProvisioner
	publisher   events.EventPublisher
}

func NewCreateVMHandler(
	nodeRepo domain.NodeRepository,
	vmRepo domain.VMRepository,
	provisioner domain.VMProvisioner,
	pub events.EventPublisher,
) *CreateVMHandler {
	return &CreateVMHandler{
		nodeRepo:    nodeRepo,
		vmRepo:      vmRepo,
		provisioner: provisioner,
		publisher:   pub,
	}
}

func (h *CreateVMHandler) Handle(ctx context.Context, cmd CreateVMCommand) (*domain.VirtualMachine, error) {
	node, err := h.nodeRepo.FindByID(ctx, cmd.NodeID)
	if err != nil {
		return nil, err
	}
	conn := node.ToConn()

	// Allocate VMID from Proxmox cluster.
	vmid, err := h.provisioner.NextVMID(ctx, conn)
	if err != nil {
		return nil, err
	}

	// Persist record immediately in "creating" status.
	vm, err := domain.NewVirtualMachine(vmid, cmd.NodeID, cmd.CustomerID, cmd.Name, cmd.Cores, cmd.MemoryMB, cmd.DiskGB, cmd.Notes)
	if err != nil {
		return nil, err
	}
	if err := h.vmRepo.Save(ctx, vm); err != nil {
		return nil, err
	}

	// Publish created event (status=creating) so UI shows the row immediately.
	h.publishStatusChanged(ctx, vm)

	// Clone in background — request context is released after this function returns.
	spec := domain.VMSpec{
		TemplateVMID: cmd.TemplateVMID,
		NewVMID:      vmid,
		Name:         cmd.Name,
		Storage:      cmd.Storage,
		Cores:        cmd.Cores,
		MemoryMB:     cmd.MemoryMB,
		DiskGB:       cmd.DiskGB,
		FullClone:    cmd.FullClone,
	}
	go h.runCreate(vm.ID, conn, spec)

	return vm, nil
}

func (h *CreateVMHandler) runCreate(vmID string, conn domain.NodeConn, spec domain.VMSpec) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	writeStatus := func(status domain.VMStatus) {
		wctx, wcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer wcancel()
		if err := h.vmRepo.UpdateStatus(wctx, vmID, status); err != nil {
			slog.Error("proxmox: update vm status failed", "vmID", vmID, "status", status, "err", err)
		}
	}

	upid, err := h.provisioner.CreateVM(ctx, conn, spec)
	if err != nil {
		slog.Error("proxmox: create VM failed", "vmID", vmID, "err", err)
		writeStatus(domain.VMStatusUnknown)
		return
	}
	if err := h.provisioner.WaitForTask(ctx, conn, upid); err != nil {
		slog.Error("proxmox: create VM task failed", "vmID", vmID, "err", err)
		writeStatus(domain.VMStatusUnknown)
		return
	}

	writeStatus(domain.VMStatusRunning)
	wctx, wcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wcancel()
	h.publishEvent(wctx, events.ProxmoxVMCreated.String(), "proxmox.vm.created", vmID)
}

func (h *CreateVMHandler) publishStatusChanged(ctx context.Context, vm *domain.VirtualMachine) {
	data, _ := json.Marshal(map[string]any{"id": vm.ID, "vmid": vm.VMID, "status": vm.Status})
	h.publisher.Publish(ctx, events.ProxmoxVMStatusChanged.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "proxmox.vm.status_changed",
		AggregateID: vm.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
}

func (h *CreateVMHandler) publishEvent(ctx context.Context, subject, eventType, vmID string) {
	data, _ := json.Marshal(map[string]string{"id": vmID})
	h.publisher.Publish(ctx, subject, events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        eventType,
		AggregateID: vmID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
}
