package persistence

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/ticket/domain"
	"gorm.io/gorm"
)

// --- GORM models ---

type TicketModel struct {
	ID         string    `gorm:"primaryKey;type:text"`
	CustomerID string    `gorm:"type:text;not null;column:customer_id"`
	Subject    string    `gorm:"type:text;not null"`
	Body       string    `gorm:"type:text;not null;default:''"`
	Status     string    `gorm:"type:text;not null;default:'open'"`
	Priority   string    `gorm:"type:text;not null;default:'normal'"`
	AssigneeID string    `gorm:"type:text;not null;default:'';column:assignee_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (TicketModel) TableName() string { return "tickets" }

type TicketCommentModel struct {
	ID        string    `gorm:"primaryKey;type:text"`
	TicketID  string    `gorm:"type:text;not null;column:ticket_id"`
	Body      string    `gorm:"type:text;not null"`
	AuthorID  string    `gorm:"type:text;not null;default:'';column:author_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (TicketCommentModel) TableName() string { return "ticket_comments" }

// --- mapping helpers ---

func toTicketModel(t *domain.Ticket) TicketModel {
	return TicketModel{
		ID:         t.ID,
		CustomerID: t.CustomerID,
		Subject:    t.Subject,
		Body:       t.Body,
		Status:     t.Status,
		Priority:   t.Priority,
		AssigneeID: t.AssigneeID,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

func (m *TicketModel) toDomain() *domain.Ticket {
	return &domain.Ticket{
		ID:         m.ID,
		CustomerID: m.CustomerID,
		Subject:    m.Subject,
		Body:       m.Body,
		Status:     m.Status,
		Priority:   m.Priority,
		AssigneeID: m.AssigneeID,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

func toCommentModel(c *domain.TicketComment) TicketCommentModel {
	return TicketCommentModel{
		ID:        c.ID,
		TicketID:  c.TicketID,
		Body:      c.Body,
		AuthorID:  c.AuthorID,
		CreatedAt: c.CreatedAt,
	}
}

func (m *TicketCommentModel) toDomain() *domain.TicketComment {
	return &domain.TicketComment{
		ID:        m.ID,
		TicketID:  m.TicketID,
		Body:      m.Body,
		AuthorID:  m.AuthorID,
		CreatedAt: m.CreatedAt,
	}
}

// --- repository ---

// GormTicketRepository implements domain.TicketRepository using GORM + SQLite.
type GormTicketRepository struct {
	db *gormsqlite.DB
}

func NewGormTicketRepository(db *gormsqlite.DB) *GormTicketRepository {
	return &GormTicketRepository{db: db}
}

func (r *GormTicketRepository) Save(ctx context.Context, t *domain.Ticket) error {
	model := toTicketModel(t)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&model).Error
	})
}

func (r *GormTicketRepository) FindByID(ctx context.Context, id string) (*domain.Ticket, error) {
	var model TicketModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&model).Error
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *GormTicketRepository) ListAll(ctx context.Context) ([]*domain.Ticket, error) {
	var models []TicketModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Ticket, len(models))
	for i := range models {
		result[i] = models[i].toDomain()
	}
	return result, nil
}

func (r *GormTicketRepository) ListForCustomer(ctx context.Context, customerID string) ([]*domain.Ticket, error) {
	var models []TicketModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.Ticket, len(models))
	for i := range models {
		result[i] = models[i].toDomain()
	}
	return result, nil
}

func (r *GormTicketRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&TicketModel{}).Error
	})
}

func (r *GormTicketRepository) SaveComment(ctx context.Context, c *domain.TicketComment) error {
	model := toCommentModel(c)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&model).Error
	})
}

func (r *GormTicketRepository) ListComments(ctx context.Context, ticketID string) ([]*domain.TicketComment, error) {
	var models []TicketCommentModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.TicketComment, len(models))
	for i := range models {
		result[i] = models[i].toDomain()
	}
	return result, nil
}
