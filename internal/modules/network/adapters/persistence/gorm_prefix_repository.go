package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/network/domain"
)

// ensure gorm is used (for ErrRecordNotFound)
var _ = gorm.ErrRecordNotFound

type PrefixModel struct {
	ID        string    `gorm:"primaryKey;type:text"`
	NetBoxID  int       `gorm:"column:netbox_id;uniqueIndex;not null"`
	CIDR      string    `gorm:"column:cidr;type:text"`
	Location  string    `gorm:"type:text;not null;index"`
	Priority  int       `gorm:"not null;default:0"`
	CreatedAt time.Time
}

func (PrefixModel) TableName() string { return "netbox_prefixes" }

func prefixToModel(p *domain.NetBoxPrefix) *PrefixModel {
	return &PrefixModel{
		ID:        p.ID,
		NetBoxID:  p.NetBoxID,
		CIDR:      p.CIDR,
		Location:  p.Location,
		Priority:  p.Priority,
		CreatedAt: p.CreatedAt,
	}
}

func prefixToDomain(m *PrefixModel) *domain.NetBoxPrefix {
	return &domain.NetBoxPrefix{
		ID:        m.ID,
		NetBoxID:  m.NetBoxID,
		CIDR:      m.CIDR,
		Location:  m.Location,
		Priority:  m.Priority,
		CreatedAt: m.CreatedAt,
	}
}

type GormPrefixRepository struct {
	db *gormsqlite.DB
}

func NewGormPrefixRepository(db *gormsqlite.DB) *GormPrefixRepository {
	return &GormPrefixRepository{db: db}
}

func (r *GormPrefixRepository) Save(ctx context.Context, p *domain.NetBoxPrefix) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(prefixToModel(p)).Error
	})
}

func (r *GormPrefixRepository) FindByID(ctx context.Context, id string) (*domain.NetBoxPrefix, error) {
	var m PrefixModel
	err := r.db.R.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPrefixNotFound
	}
	if err != nil {
		return nil, err
	}
	return prefixToDomain(&m), nil
}

func (r *GormPrefixRepository) ListByLocation(ctx context.Context, location string) ([]*domain.NetBoxPrefix, error) {
	var models []PrefixModel
	err := r.db.R.WithContext(ctx).
		Where("location = ?", location).
		Order("priority asc").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]*domain.NetBoxPrefix, len(models))
	for i := range models {
		out[i] = prefixToDomain(&models[i])
	}
	return out, nil
}

func (r *GormPrefixRepository) ListAll(ctx context.Context) ([]*domain.NetBoxPrefix, error) {
	var models []PrefixModel
	err := r.db.R.WithContext(ctx).Order("location asc, priority asc").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]*domain.NetBoxPrefix, len(models))
	for i := range models {
		out[i] = prefixToDomain(&models[i])
	}
	return out, nil
}

func (r *GormPrefixRepository) ListLocations(ctx context.Context) ([]string, error) {
	var locs []string
	err := r.db.R.WithContext(ctx).
		Model(&PrefixModel{}).
		Distinct("location").
		Order("location asc").
		Pluck("location", &locs).Error
	return locs, err
}

func (r *GormPrefixRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&PrefixModel{}).Error
	})
}
