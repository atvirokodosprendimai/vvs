package persistence

import (
	"context"
	"errors"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	"gorm.io/gorm"
)

type GormRoleRepository struct {
	db *gormsqlite.DB
}

func NewGormRoleRepository(db *gormsqlite.DB) *GormRoleRepository {
	return &GormRoleRepository{db: db}
}

func (r *GormRoleRepository) List(ctx context.Context) ([]domain.RoleDefinition, error) {
	var roles []domain.RoleDefinition
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []RoleModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		roles = make([]domain.RoleDefinition, len(models))
		for i, m := range models {
			roles[i] = *roleToDomain(&m)
		}
		return nil
	})
	return roles, err
}

func (r *GormRoleRepository) FindByName(ctx context.Context, name domain.Role) (*domain.RoleDefinition, error) {
	var m RoleModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("name = ?", string(name)).First(&m).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, err
	}
	return roleToDomain(&m), nil
}

func (r *GormRoleRepository) Save(ctx context.Context, rd *domain.RoleDefinition) error {
	m := roleToModel(rd)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(m).Error
	})
}

func (r *GormRoleRepository) Delete(ctx context.Context, name domain.Role) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		// Guard: reject builtin roles.
		var m RoleModel
		if err := tx.Where("name = ?", string(name)).First(&m).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRoleNotFound
			}
			return err
		}
		if m.IsBuiltin {
			return domain.ErrRoleBuiltin
		}
		// Guard: reject if any users are assigned this role.
		var count int64
		if err := tx.Table("users").Where("role = ?", string(name)).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return domain.ErrRoleInUse
		}
		return tx.Delete(&RoleModel{}, "name = ?", string(name)).Error
	})
}
