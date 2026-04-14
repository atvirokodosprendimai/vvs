package persistence

import (
	"context"
	"errors"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/modules/auth/domain"
	"gorm.io/gorm"
)

// --- UserRepository ---

type GormUserRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormUserRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormUserRepository {
	return &GormUserRepository{writer: writer, reader: reader}
}

func (r *GormUserRepository) Save(ctx context.Context, u *domain.User) error {
	m := userToModel(u)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Save(m).Error
	})
}

func (r *GormUserRepository) FindByID(_ context.Context, id string) (*domain.User, error) {
	var m UserModel
	if err := r.reader.Where("id = ?", id).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return userToDomain(&m), nil
}

func (r *GormUserRepository) FindByUsername(_ context.Context, username string) (*domain.User, error) {
	var m UserModel
	if err := r.reader.Where("username = ?", username).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return userToDomain(&m), nil
}

func (r *GormUserRepository) ListAll(_ context.Context) ([]*domain.User, error) {
	var models []UserModel
	if err := r.reader.Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	users := make([]*domain.User, len(models))
	for i, m := range models {
		users[i] = userToDomain(&m)
	}
	return users, nil
}

func (r *GormUserRepository) Delete(ctx context.Context, id string) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Delete(&UserModel{}, "id = ?", id).Error
	})
}

// --- SessionRepository ---

type GormSessionRepository struct {
	writer *database.WriteSerializer
	reader *gorm.DB
}

func NewGormSessionRepository(writer *database.WriteSerializer, reader *gorm.DB) *GormSessionRepository {
	return &GormSessionRepository{writer: writer, reader: reader}
}

func (r *GormSessionRepository) Save(ctx context.Context, s *domain.Session) error {
	m := sessionToModel(s)
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Save(m).Error
	})
}

func (r *GormSessionRepository) FindByTokenHash(_ context.Context, tokenHash string) (*domain.Session, error) {
	var m SessionModel
	if err := r.reader.Where("token_hash = ?", tokenHash).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return sessionToDomain(&m), nil
}

func (r *GormSessionRepository) DeleteByID(ctx context.Context, id string) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Delete(&SessionModel{}, "id = ?", id).Error
	})
}

func (r *GormSessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Delete(&SessionModel{}, "user_id = ?", userID).Error
	})
}

func (r *GormSessionRepository) PruneExpired(ctx context.Context) error {
	return r.writer.Execute(ctx, func(tx *gorm.DB) error {
		return tx.Where("expires_at < datetime('now')").Delete(&SessionModel{}).Error
	})
}
