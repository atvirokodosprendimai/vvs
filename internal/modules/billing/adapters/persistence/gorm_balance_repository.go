package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/modules/billing/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ── Models ─────────────────────────────────────────────────────────────────────

type customerBalanceModel struct {
	CustomerID   string    `gorm:"primaryKey;type:text"`
	BalanceCents int64     `gorm:"column:balance_cents;not null;default:0"`
	UpdatedAt    time.Time
}

func (customerBalanceModel) TableName() string { return "customer_balance" }

type ledgerEntryModel struct {
	ID              string    `gorm:"primaryKey;type:text"`
	CustomerID      string    `gorm:"column:customer_id;type:text;not null"`
	Type            string    `gorm:"column:type;type:text;not null"`
	AmountCents     int64     `gorm:"column:amount_cents;not null"`
	Description     string    `gorm:"column:description;type:text;not null;default:''"`
	StripeSessionID string    `gorm:"column:stripe_session_id;type:text;not null;default:''"`
	CreatedAt       time.Time
}

func (ledgerEntryModel) TableName() string { return "balance_ledger" }

func toLedgerDomain(m *ledgerEntryModel) *domain.BalanceLedgerEntry {
	return &domain.BalanceLedgerEntry{
		ID:              m.ID,
		CustomerID:      m.CustomerID,
		Type:            domain.EntryType(m.Type),
		AmountCents:     m.AmountCents,
		Description:     m.Description,
		StripeSessionID: m.StripeSessionID,
		CreatedAt:       m.CreatedAt,
	}
}

// ── Repository ────────────────────────────────────────────────────────────────

type GormBalanceRepository struct {
	db *gormsqlite.DB
}

func NewGormBalanceRepository(db *gormsqlite.DB) *GormBalanceRepository {
	return &GormBalanceRepository{db: db}
}

func (r *GormBalanceRepository) GetBalance(ctx context.Context, customerID string) (int64, error) {
	var m customerBalanceModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).First(&m).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return m.BalanceCents, nil
}

func (r *GormBalanceRepository) Credit(ctx context.Context, customerID string, amountCents int64, entryType domain.EntryType, description, stripeSessionID string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		// Insert ledger entry (unique index on stripe_session_id enforces idempotency)
		entry := ledgerEntryModel{
			ID:              uuid.Must(uuid.NewV7()).String(),
			CustomerID:      customerID,
			Type:            string(entryType),
			AmountCents:     amountCents,
			Description:     description,
			StripeSessionID: stripeSessionID,
			CreatedAt:       time.Now().UTC(),
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		// Upsert balance
		bal := customerBalanceModel{
			CustomerID:   customerID,
			BalanceCents: amountCents,
			UpdatedAt:    time.Now().UTC(),
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "customer_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"balance_cents": gorm.Expr("balance_cents + ?", amountCents),
				"updated_at":    time.Now().UTC(),
			}),
		}).Create(&bal).Error
	})
}

func (r *GormBalanceRepository) Deduct(ctx context.Context, customerID string, amountCents int64, entryType domain.EntryType, description string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		// Read current balance inside transaction
		var m customerBalanceModel
		err := tx.Where("customer_id = ?", customerID).First(&m).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInsufficientBalance
			}
			return err
		}
		if m.BalanceCents < amountCents {
			return domain.ErrInsufficientBalance
		}
		// Insert debit ledger entry
		entry := ledgerEntryModel{
			ID:          uuid.Must(uuid.NewV7()).String(),
			CustomerID:  customerID,
			Type:        string(entryType),
			AmountCents: -amountCents,
			Description: description,
			CreatedAt:   time.Now().UTC(),
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		// Update balance
		return tx.Model(&customerBalanceModel{}).
			Where("customer_id = ?", customerID).
			Updates(map[string]any{
				"balance_cents": m.BalanceCents - amountCents,
				"updated_at":    time.Now().UTC(),
			}).Error
	})
}

func (r *GormBalanceRepository) GetLedger(ctx context.Context, customerID string) ([]*domain.BalanceLedgerEntry, error) {
	var entries []*domain.BalanceLedgerEntry
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		var models []ledgerEntryModel
		if err := tx.Where("customer_id = ?", customerID).Order("created_at DESC").Find(&models).Error; err != nil {
			return err
		}
		entries = make([]*domain.BalanceLedgerEntry, len(models))
		for i := range models {
			entries[i] = toLedgerDomain(&models[i])
		}
		return nil
	})
	return entries, err
}
