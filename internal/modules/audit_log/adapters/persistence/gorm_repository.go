package persistence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/audit_log/domain"
)

// AuditLogModel is the GORM representation of an audit log row.
type AuditLogModel struct {
	ID         string    `gorm:"primaryKey;type:text"`
	ActorID    string    `gorm:"type:text;not null;default:'';column:actor_id"`
	ActorName  string    `gorm:"type:text;not null;default:'';column:actor_name"`
	Action     string    `gorm:"type:text;not null"`
	Resource   string    `gorm:"type:text;not null"`
	ResourceID string    `gorm:"type:text;not null;column:resource_id"`
	Changes    string    `gorm:"type:text;not null;default:''"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (AuditLogModel) TableName() string { return "audit_logs" }

// --- mapping helpers ---

func toModel(al *domain.AuditLog) AuditLogModel {
	changes := ""
	if al.Changes != nil {
		changes = string(al.Changes)
	}
	return AuditLogModel{
		ID:         al.ID,
		ActorID:    al.ActorID,
		ActorName:  al.ActorName,
		Action:     al.Action,
		Resource:   al.Resource,
		ResourceID: al.ResourceID,
		Changes:    changes,
		CreatedAt:  al.CreatedAt,
	}
}

func (m *AuditLogModel) toDomain() *domain.AuditLog {
	var changes json.RawMessage
	if m.Changes != "" {
		changes = json.RawMessage(m.Changes)
	}
	return &domain.AuditLog{
		ID:         m.ID,
		ActorID:    m.ActorID,
		ActorName:  m.ActorName,
		Action:     m.Action,
		Resource:   m.Resource,
		ResourceID: m.ResourceID,
		Changes:    changes,
		CreatedAt:  m.CreatedAt,
	}
}

// --- repository ---

// GormAuditLogRepository implements domain.AuditLogRepository using GORM + SQLite.
type GormAuditLogRepository struct {
	db *gormsqlite.DB
}

func NewGormAuditLogRepository(db *gormsqlite.DB) *GormAuditLogRepository {
	return &GormAuditLogRepository{db: db}
}

func (r *GormAuditLogRepository) Save(ctx context.Context, al *domain.AuditLog) error {
	model := toModel(al)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Create(&model).Error
	})
}

func (r *GormAuditLogRepository) ListAll(ctx context.Context, filter domain.Filter) ([]*domain.AuditLog, error) {
	var models []AuditLogModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		q := tx.DB // *gorm.DB embedded in Tx

		if filter.ActorID != "" {
			q = q.Where("actor_id = ?", filter.ActorID)
		}
		if filter.Resource != "" {
			q = q.Where("resource = ?", filter.Resource)
		}
		if filter.From != nil {
			q = q.Where("created_at >= ?", filter.From)
		}
		if filter.To != nil {
			q = q.Where("created_at <= ?", filter.To)
		}

		limit := filter.Limit
		if limit <= 0 {
			limit = 100
		}

		return q.Order("created_at DESC").Limit(limit).Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.AuditLog, len(models))
	for i := range models {
		result[i] = models[i].toDomain()
	}
	return result, nil
}

func (r *GormAuditLogRepository) ListForResource(ctx context.Context, resource, resourceID string) ([]*domain.AuditLog, error) {
	var models []AuditLogModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("resource = ? AND resource_id = ?", resource, resourceID).
			Order("created_at DESC").
			Limit(100).
			Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.AuditLog, len(models))
	for i := range models {
		result[i] = models[i].toDomain()
	}
	return result, nil
}
