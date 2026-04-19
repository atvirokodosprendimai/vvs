package domain

import "context"

// Module identifies a navigable feature area of the system.
type Module string

const (
	ModuleCustomers Module = "customers"
	ModuleTickets   Module = "tickets"
	ModuleDeals     Module = "deals"
	ModuleTasks     Module = "tasks"
	ModuleContacts  Module = "contacts"
	ModuleInvoices  Module = "invoices"
	ModuleProducts  Module = "products"
	ModulePayments  Module = "payments"
	ModuleNetwork   Module = "network"
	ModuleEmail     Module = "email"
	ModuleCron      Module = "cron"
	ModuleAuditLog  Module = "audit_log"
	ModuleUsers     Module = "users"
)

// AllModules lists every module in a stable order (used for seeding and UI rendering).
var AllModules = []Module{
	ModuleCustomers, ModuleTickets, ModuleDeals, ModuleTasks, ModuleContacts,
	ModuleInvoices, ModuleProducts, ModulePayments,
	ModuleNetwork, ModuleEmail, ModuleCron, ModuleAuditLog, ModuleUsers,
}

// RoleModulePermission is a single (role, module) access row.
type RoleModulePermission struct {
	Role    Role
	Module  Module
	CanView bool
	CanEdit bool
}

// PermissionSet is the resolved access map for one role, keyed by Module.
type PermissionSet map[Module]*RoleModulePermission

// CanView returns true if the module is present in the set with view access granted.
func (ps PermissionSet) CanView(m Module) bool {
	p, ok := ps[m]
	return ok && p.CanView
}

// CanEdit returns true if the module is present in the set with edit access granted.
func (ps PermissionSet) CanEdit(m Module) bool {
	p, ok := ps[m]
	return ok && p.CanEdit
}

// AdminPermissionSet returns a full-access PermissionSet for the admin role.
// Admin access is hardcoded — never read from DB.
func AdminPermissionSet() PermissionSet {
	ps := make(PermissionSet, len(AllModules))
	for _, m := range AllModules {
		ps[m] = &RoleModulePermission{Role: RoleAdmin, Module: m, CanView: true, CanEdit: true}
	}
	return ps
}

// DefaultPermissions returns the seed permissions for a role.
// Operator: all modules view + edit. Viewer: all modules view-only.
func DefaultPermissions(role Role) PermissionSet {
	ps := make(PermissionSet, len(AllModules))
	for _, m := range AllModules {
		canEdit := role == RoleOperator
		ps[m] = &RoleModulePermission{Role: role, Module: m, CanView: true, CanEdit: canEdit}
	}
	return ps
}

// RolePermissionsRepository is the port for module permission persistence.
type RolePermissionsRepository interface {
	FindByRole(ctx context.Context, role Role) (PermissionSet, error)
	Save(ctx context.Context, p *RoleModulePermission) error
}
