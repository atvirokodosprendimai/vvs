package proxmox

import (
	"context"
	"fmt"
	"net/http"

	proxmoxdomain "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// CreateVM clones a Proxmox template into a new VM.
// If spec.NewVMID is 0, a new VMID is allocated via NextVMID.
// Returns the Proxmox UPID task handle.
func (c *Client) CreateVM(ctx context.Context, conn proxmoxdomain.NodeConn, spec proxmoxdomain.VMSpec) (string, error) {
	newVMID := spec.NewVMID
	if newVMID == 0 {
		id, err := c.NextVMID(ctx, conn)
		if err != nil {
			return "", fmt.Errorf("proxmox: allocate vmid: %w", err)
		}
		newVMID = id
	}

	body := map[string]any{
		"newid": newVMID,
		"name":  spec.Name,
		"full":  boolToInt(spec.FullClone),
	}
	if spec.Storage != "" {
		body["storage"] = spec.Storage
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%d/clone", conn.NodeName, spec.TemplateVMID)
	data, err := c.do(ctx, conn, http.MethodPost, path, body)
	if err != nil {
		return "", err
	}
	return parseUPID(data)
}

// SuspendVM pauses a running VM. Returns the UPID task handle.
func (c *Client) SuspendVM(ctx context.Context, conn proxmoxdomain.NodeConn, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/suspend", conn.NodeName, vmid)
	data, err := c.do(ctx, conn, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	return parseUPID(data)
}

// StartVM starts a stopped or paused VM. Returns the UPID task handle.
func (c *Client) StartVM(ctx context.Context, conn proxmoxdomain.NodeConn, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", conn.NodeName, vmid)
	data, err := c.do(ctx, conn, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	return parseUPID(data)
}

// RestartVM reboots a running VM. Returns the UPID task handle.
func (c *Client) RestartVM(ctx context.Context, conn proxmoxdomain.NodeConn, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/reboot", conn.NodeName, vmid)
	data, err := c.do(ctx, conn, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	return parseUPID(data)
}

// StopVM hard-stops a VM. Returns the UPID task handle.
func (c *Client) StopVM(ctx context.Context, conn proxmoxdomain.NodeConn, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", conn.NodeName, vmid)
	data, err := c.do(ctx, conn, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	return parseUPID(data)
}

// DeleteVM deletes a stopped VM and purges its disks. Returns the UPID task handle.
// The VM must be stopped before calling DeleteVM; use StopVM first if needed.
func (c *Client) DeleteVM(ctx context.Context, conn proxmoxdomain.NodeConn, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d?purge=1&destroy-unreferenced-disks=1", conn.NodeName, vmid)
	data, err := c.do(ctx, conn, http.MethodDelete, path, nil)
	if err != nil {
		return "", err
	}
	return parseUPID(data)
}

// GetVMInfo returns the current status and config of a VM.
func (c *Client) GetVMInfo(ctx context.Context, conn proxmoxdomain.NodeConn, vmid int) (*proxmoxdomain.VMInfo, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", conn.NodeName, vmid)
	data, err := c.do(ctx, conn, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var raw struct {
		VMID   int    `json:"vmid"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Cpus   int    `json:"cpus"`
		MaxMem int64  `json:"maxmem"` // bytes
	}
	if err := parseData(data, &raw); err != nil {
		return nil, fmt.Errorf("proxmox: parse vm info: %w", err)
	}

	return &proxmoxdomain.VMInfo{
		VMID:     raw.VMID,
		Name:     raw.Name,
		Status:   mapPVEStatus(raw.Status),
		Cores:    raw.Cpus,
		MemoryMB: int(raw.MaxMem / (1024 * 1024)),
	}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// mapPVEStatus converts Proxmox status strings to VMStatus constants.
func mapPVEStatus(s string) proxmoxdomain.VMStatus {
	switch s {
	case "running":
		return proxmoxdomain.VMStatusRunning
	case "stopped":
		return proxmoxdomain.VMStatusStopped
	case "paused":
		return proxmoxdomain.VMStatusPaused
	default:
		return proxmoxdomain.VMStatusUnknown
	}
}
