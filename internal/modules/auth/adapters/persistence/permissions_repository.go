package persistence

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

type roleModulePermissionModel struct {
	Role    string `gorm:"primaryKey;column:role"`
	Module  string `gorm:"primaryKey;column:module"`
	CanView bool   `gorm:"column:can_view;not null;default:1"`
	CanEdit bool   `gorm:"column:can_edit;not null;default:1"`
}

func (roleModulePermissionModel) TableName() string { return "role_module_permissions" }

// GormRolePermissionsRepository implements domain.RolePermissionsRepository.
type GormRolePermissionsRepository struct {
	db *gormsqlite.DB
}

func NewGormRolePermissionsRepository(db *gormsqlite.DB) *GormRolePermissionsRepository {
	return &GormRolePermissionsRepository{db: db}
}

func (r *GormRolePermissionsRepository) FindByRole(ctx context.Context, role domain.Role) (domain.PermissionSet, error) {
	var rows []roleModulePermissionModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("role = ?", string(role)).Find(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	ps := make(domain.PermissionSet, len(rows))
	for i := range rows {
		m := domain.Module(rows[i].Module)
		ps[m] = &domain.RoleModulePermission{
			Role:    domain.Role(rows[i].Role),
			Module:  m,
			CanView: rows[i].CanView,
			CanEdit: rows[i].CanEdit,
		}
	}
	return ps, nil
}

func (r *GormRolePermissionsRepository) Save(ctx context.Context, p *domain.RoleModulePermission) error {
	m := roleModulePermissionModel{
		Role:    string(p.Role),
		Module:  string(p.Module),
		CanView: p.CanView,
		CanEdit: p.CanEdit,
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}
