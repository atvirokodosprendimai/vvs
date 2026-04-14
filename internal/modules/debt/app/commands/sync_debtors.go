package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/infrastructure/itaxtlt"
	"github.com/vvs/isp/internal/modules/debt/domain"
	"github.com/vvs/isp/internal/shared/events"
	"gorm.io/gorm"
)

type SyncDebtorsCommand struct{}

type SyncDebtorsHandler struct {
	provider  itaxtlt.DebtorProvider
	repo      domain.DebtRepository
	reader    *gorm.DB
	publisher events.EventPublisher
}

func NewSyncDebtorsHandler(
	provider itaxtlt.DebtorProvider,
	repo domain.DebtRepository,
	reader *gorm.DB,
	pub events.EventPublisher,
) *SyncDebtorsHandler {
	return &SyncDebtorsHandler{
		provider:  provider,
		repo:      repo,
		reader:    reader,
		publisher: pub,
	}
}

type customerLookup struct {
	ID    string
	TaxID string `gorm:"column:tax_id"`
}

func (h *SyncDebtorsHandler) Handle(ctx context.Context, _ SyncDebtorsCommand) error {
	records, err := h.provider.FetchDebtors(ctx)
	if err != nil {
		return err
	}

	// Build taxID -> customerID index from our customers table.
	var customers []customerLookup
	if err := h.reader.Table("customers").
		Select("id, tax_id").
		Where("tax_id != ''").
		Scan(&customers).Error; err != nil {
		return err
	}

	byTaxID := make(map[string]string, len(customers))
	for _, c := range customers {
		byTaxID[c.TaxID] = c.ID
	}

	for _, rec := range records {
		customerID, ok := byTaxID[rec.ClientCode]
		if !ok {
			continue // itax.lt client not in our system
		}
		status := domain.NewDebtStatus(customerID, rec.ClientCode, rec.OverCreditBudget)
		if err := h.repo.Upsert(ctx, status); err != nil {
			return err
		}
	}

	data, _ := json.Marshal(map[string]any{"synced_count": len(records)})
	h.publisher.Publish(ctx, "isp.debt.synced", events.DomainEvent{
		ID:         uuid.Must(uuid.NewV7()).String(),
		Type:       "debt.synced",
		OccurredAt: time.Now().UTC(),
		Data:       data,
	})

	return nil
}
