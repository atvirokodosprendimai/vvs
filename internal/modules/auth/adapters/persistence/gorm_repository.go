package persistence

import (
	"context"
	"errors"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	"gorm.io/gorm"
)

// --- UserRepository ---

type GormUserRepository struct {
	db *gormsqlite.DB
}

func NewGormUserRepository(db *gormsqlite.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Save(ctx context.Context, u *domain.User) error {
	m := userToModel(u)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(m).Error
	})
}

func (r *GormUserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var m UserModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return userToDomain(&m), nil
}

func (r *GormUserRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var m UserModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("username = ?", username).First(&m).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return userToDomain(&m), nil
}

func (r *GormUserRepository) ListAll(ctx context.Context) ([]*domain.User, error) {
	var users []*domain.User
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []UserModel
		if err := tx.Order("created_at ASC").Find(&models).Error; err != nil {
			return err
		}
		users = make([]*domain.User, len(models))
		for i, m := range models {
			users[i] = userToDomain(&m)
		}
		return nil
	})
	return users, err
}

func (r *GormUserRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Delete(&UserModel{}, "id = ?", id).Error
	})
}

// --- SessionRepository ---

type GormSessionRepository struct {
	db *gormsqlite.DB
}

func NewGormSessionRepository(db *gormsqlite.DB) *GormSessionRepository {
	return &GormSessionRepository{db: db}
}

func (r *GormSessionRepository) Save(ctx context.Context, s *domain.Session) error {
	m := sessionToModel(s)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(m).Error
	})
}

func (r *GormSessionRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	var m SessionModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("token_hash = ?", tokenHash).First(&m).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return sessionToDomain(&m), nil
}

func (r *GormSessionRepository) DeleteByID(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Delete(&SessionModel{}, "id = ?", id).Error
	})
}

func (r *GormSessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Delete(&SessionModel{}, "user_id = ?", userID).Error
	})
}

func (r *GormSessionRepository) PruneExpired(ctx context.Context) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("expires_at < datetime('now')").Delete(&SessionModel{}).Error
	})
}
