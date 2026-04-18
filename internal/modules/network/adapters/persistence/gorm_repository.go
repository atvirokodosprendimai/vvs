package persistence

import (
	"context"
	"fmt"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/email/emailcrypto"
	"github.com/vvs/isp/internal/modules/network/domain"
	"gorm.io/gorm"
)

type GormRouterRepository struct {
	db     *gormsqlite.DB
	encKey []byte
}

// NewGormRouterRepository creates a router repository with optional AES-256-GCM encryption.
// Pass a 32-byte encKey to encrypt passwords at rest; omit or pass nil/empty for dev mode (passthrough).
func NewGormRouterRepository(db *gormsqlite.DB, encKey ...[]byte) *GormRouterRepository {
	var key []byte
	if len(encKey) > 0 {
		key = encKey[0]
	}
	return &GormRouterRepository{db: db, encKey: key}
}

func (r *GormRouterRepository) Save(ctx context.Context, router *domain.Router) error {
	enc, err := emailcrypto.EncryptPassword(r.encKey, []byte(router.Password))
	if err != nil {
		return fmt.Errorf("encrypt router password: %w", err)
	}
	model := toModel(router)
	model.PasswordEnc = enc
	model.Password = "" // clear plaintext
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(model).Error
	})
}

func (r *GormRouterRepository) FindByID(ctx context.Context, id string) (*domain.Router, error) {
	var model RouterModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrRouterNotFound
		}
		return nil, err
	}
	router := toDomain(&model)
	if len(model.PasswordEnc) > 0 {
		if plain, err := emailcrypto.DecryptPassword(r.encKey, model.PasswordEnc); err == nil {
			router.Password = string(plain)
		}
	}
	// fallback: if PasswordEnc is empty, model.Password (plaintext legacy) is used via toDomain
	return router, nil
}

func (r *GormRouterRepository) FindAll(ctx context.Context) ([]*domain.Router, error) {
	var routers []*domain.Router
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []RouterModel
		if err := tx.Order("name ASC").Find(&models).Error; err != nil {
			return err
		}
		routers = make([]*domain.Router, len(models))
		for i, m := range models {
			router := toDomain(&m)
			if len(m.PasswordEnc) > 0 {
				if plain, err := emailcrypto.DecryptPassword(r.encKey, m.PasswordEnc); err == nil {
					router.Password = string(plain)
				}
			}
			// fallback: if PasswordEnc is empty, model.Password (plaintext legacy) is used via toDomain
			routers[i] = router
		}
		return nil
	})
	return routers, err
}

func (r *GormRouterRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Delete(&RouterModel{}, "id = ?", id).Error
	})
}
