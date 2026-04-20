package domain_test

import (
	"testing"

	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVirtualMachine_Valid(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "cust-1", "web-server", 2, 2048, 20, "notes")
	require.NoError(t, err)
	assert.NotEmpty(t, vm.ID)
	assert.Equal(t, 101, vm.VMID)
	assert.Equal(t, "node-1", vm.NodeID)
	assert.Equal(t, "cust-1", vm.CustomerID)
	assert.Equal(t, "web-server", vm.Name)
	assert.Equal(t, domain.VMStatusCreating, vm.Status)
	assert.Equal(t, 2, vm.Cores)
	assert.Equal(t, 2048, vm.MemoryMB)
	assert.Equal(t, 20, vm.DiskGB)
}

func TestNewVirtualMachine_DefaultDisk(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "", "vm", 1, 512, 0, "")
	require.NoError(t, err)
	assert.Equal(t, 10, vm.DiskGB)
}

func TestNewVirtualMachine_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		vmid    int
		vmName  string
		cores   int
		memory  int
		wantErr error
	}{
		{"zero vmid", 0, "vm", 1, 512, domain.ErrVMIDRequired},
		{"empty name", 101, "", 1, 512, domain.ErrVMNameRequired},
		{"zero cores", 101, "vm", 0, 512, domain.ErrCoresPositive},
		{"zero memory", 101, "vm", 1, 0, domain.ErrMemoryPositive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewVirtualMachine(tt.vmid, "node-1", "", tt.vmName, tt.cores, tt.memory, 10, "")
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestVirtualMachine_Suspend_RunningToPaused(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "", "vm", 1, 512, 10, "")
	require.NoError(t, err)
	vm.MarkRunning()

	err = vm.Suspend()
	require.NoError(t, err)
	assert.Equal(t, domain.VMStatusPaused, vm.Status)
}

func TestVirtualMachine_Suspend_NotRunning_Errors(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "", "vm", 1, 512, 10, "")
	require.NoError(t, err)
	// Status is "creating" — not running

	err = vm.Suspend()
	assert.ErrorIs(t, err, domain.ErrVMNotRunning)
}

func TestVirtualMachine_Resume_PausedToRunning(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "", "vm", 1, 512, 10, "")
	require.NoError(t, err)
	vm.MarkRunning()
	require.NoError(t, vm.Suspend())

	err = vm.Resume()
	require.NoError(t, err)
	assert.Equal(t, domain.VMStatusRunning, vm.Status)
}

func TestVirtualMachine_Resume_NotPaused_Errors(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "", "vm", 1, 512, 10, "")
	require.NoError(t, err)
	vm.MarkRunning()

	err = vm.Resume()
	assert.ErrorIs(t, err, domain.ErrVMNotPaused)
}

func TestVirtualMachine_MarkDeleting(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "", "vm", 1, 512, 10, "")
	require.NoError(t, err)

	err = vm.MarkDeleting()
	require.NoError(t, err)
	assert.Equal(t, domain.VMStatusDeleting, vm.Status)
}

func TestVirtualMachine_MarkDeleting_AlreadyDeleting(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "", "vm", 1, 512, 10, "")
	require.NoError(t, err)
	require.NoError(t, vm.MarkDeleting())

	err = vm.MarkDeleting()
	assert.ErrorIs(t, err, domain.ErrVMAlreadyDeleting)
}

func TestVirtualMachine_AssignUnassignCustomer(t *testing.T) {
	vm, err := domain.NewVirtualMachine(101, "node-1", "", "vm", 1, 512, 10, "")
	require.NoError(t, err)
	assert.Empty(t, vm.CustomerID)

	vm.AssignCustomer("cust-99")
	assert.Equal(t, "cust-99", vm.CustomerID)

	vm.UnassignCustomer()
	assert.Empty(t, vm.CustomerID)
}
