package persistence

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormEmailFolderRepository struct{ db *gormsqlite.DB }

func NewGormEmailFolderRepository(db *gormsqlite.DB) *GormEmailFolderRepository {
	return &GormEmailFolderRepository{db: db}
}

func (r *GormEmailFolderRepository) Save(ctx context.Context, f *domain.EmailFolder) error {
	m := folderModel{
		ID:        f.ID,
		AccountID: f.AccountID,
		Name:      f.Name,
		LastUID:   f.LastUID,
		Enabled:   f.Enabled,
		CreatedAt: f.CreatedAt,
	}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "account_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"last_uid", "enabled"}),
		}).Create(&m).Error
	})
}

func (r *GormEmailFolderRepository) ListForAccount(ctx context.Context, accountID string) ([]*domain.EmailFolder, error) {
	var models []folderModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("account_id = ?", accountID).Order("name ASC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EmailFolder, len(models))
	for i := range models {
		out[i] = models[i].toDomain()
	}
	return out, nil
}

func (r *GormEmailFolderRepository) FindByID(ctx context.Context, id string) (*domain.EmailFolder, error) {
	var m folderModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrFolderNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormEmailFolderRepository) FindByAccountAndName(ctx context.Context, accountID, name string) (*domain.EmailFolder, error) {
	var m folderModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("account_id = ? AND name = ?", accountID, name).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrFolderNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormEmailFolderRepository) ListThreadIDsWithFolder(ctx context.Context, accountID, folder string) ([]string, error) {
	var ids []string
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(
			`SELECT DISTINCT thread_id FROM email_messages WHERE account_id = ? AND folder = ?`,
			accountID, folder,
		).Scan(&ids).Error
	})
	return ids, err
}

func (m *folderModel) toDomain() *domain.EmailFolder {
	return &domain.EmailFolder{
		ID:        m.ID,
		AccountID: m.AccountID,
		Name:      m.Name,
		LastUID:   m.LastUID,
		Enabled:   m.Enabled,
		CreatedAt: m.CreatedAt,
	}
}
