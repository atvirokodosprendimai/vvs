package domain

import "context"

// NodeRepository persists Proxmox node connection profiles.
type NodeRepository interface {
	Save(ctx context.Context, node *ProxmoxNode) error
	FindByID(ctx context.Context, id string) (*ProxmoxNode, error)
	FindAll(ctx context.Context) ([]*ProxmoxNode, error)
	Delete(ctx context.Context, id string) error
}

// VMRepository persists virtual machine records.
type VMRepository interface {
	Save(ctx context.Context, vm *VirtualMachine) error
	FindByID(ctx context.Context, id string) (*VirtualMachine, error)
	FindByCustomerID(ctx context.Context, customerID string) ([]*VirtualMachine, error)
	FindByNodeID(ctx context.Context, nodeID string) ([]*VirtualMachine, error)
	FindAll(ctx context.Context) ([]*VirtualMachine, error)
	UpdateStatus(ctx context.Context, id string, status VMStatus) error
	Delete(ctx context.Context, id string) error
}
