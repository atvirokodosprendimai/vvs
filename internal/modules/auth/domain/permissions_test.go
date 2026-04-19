package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vvs/isp/internal/modules/auth/domain"
)

func TestPermissionSet_CanView_PresentAndAllowed(t *testing.T) {
	ps := domain.PermissionSet{
		domain.ModuleCustomers: {Role: domain.RoleOperator, Module: domain.ModuleCustomers, CanView: true, CanEdit: true},
	}
	assert.True(t, ps.CanView(domain.ModuleCustomers))
}

func TestPermissionSet_CanView_PresentButDenied(t *testing.T) {
	ps := domain.PermissionSet{
		domain.ModuleNetwork: {Role: domain.RoleOperator, Module: domain.ModuleNetwork, CanView: false, CanEdit: false},
	}
	assert.False(t, ps.CanView(domain.ModuleNetwork))
}

func TestPermissionSet_CanView_Absent(t *testing.T) {
	ps := domain.PermissionSet{}
	assert.False(t, ps.CanView(domain.ModuleInvoices))
}

func TestPermissionSet_CanEdit_ViewOnlyModule(t *testing.T) {
	ps := domain.PermissionSet{
		domain.ModuleInvoices: {Role: domain.RoleViewer, Module: domain.ModuleInvoices, CanView: true, CanEdit: false},
	}
	assert.True(t, ps.CanView(domain.ModuleInvoices))
	assert.False(t, ps.CanEdit(domain.ModuleInvoices))
}

func TestPermissionSet_CanEdit_Absent(t *testing.T) {
	ps := domain.PermissionSet{}
	assert.False(t, ps.CanEdit(domain.ModuleCustomers))
}

func TestAdminPermissionSet_CanViewAll(t *testing.T) {
	ps := domain.AdminPermissionSet()
	for _, m := range domain.AllModules {
		assert.True(t, ps.CanView(m), "admin must view %s", m)
		assert.True(t, ps.CanEdit(m), "admin must edit %s", m)
	}
}

func TestDefaultOperatorPermissions_ViewAndEdit_ExceptUsers(t *testing.T) {
	perms := domain.DefaultPermissions(domain.RoleOperator)
	for _, m := range domain.AllModules {
		if m == domain.ModuleUsers {
			assert.False(t, perms[m].CanView, "operator must not view users by default")
			assert.False(t, perms[m].CanEdit, "operator must not edit users by default")
		} else {
			assert.True(t, perms[m].CanView, "operator should view %s", m)
			assert.True(t, perms[m].CanEdit, "operator should edit %s", m)
		}
	}
}

func TestDefaultViewerPermissions_ViewOnly_ExceptUsers(t *testing.T) {
	perms := domain.DefaultPermissions(domain.RoleViewer)
	for _, m := range domain.AllModules {
		if m == domain.ModuleUsers {
			assert.False(t, perms[m].CanView, "viewer must not view users by default")
			assert.False(t, perms[m].CanEdit, "viewer must not edit users by default")
		} else {
			assert.True(t, perms[m].CanView, "viewer should view %s", m)
			assert.False(t, perms[m].CanEdit, "viewer must not edit %s", m)
		}
	}
}
