package proxmox_test

import (
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/proxmox"
	proxmoxdomain "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// Compile-time interface satisfaction check.
var _ proxmoxdomain.VMProvisioner = (*proxmox.Client)(nil)
