package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrDebtStatusNotFound = errors.New("debt status not found")

// DebtStatus tracks a customer's credit status as reported by itax.lt.
// One record per customer, upserted on every sync.
type DebtStatus struct {
	ID               string
	CustomerID       string
	TaxID            string // Lithuanian company registration code matched against itax.lt
	OverCreditBudget bool   // true if itax.lt reports debt > credit budget
	SyncedAt         time.Time
}

func NewDebtStatus(customerID, taxID string, overCreditBudget bool) *DebtStatus {
	return &DebtStatus{
		ID:               uuid.Must(uuid.NewV7()).String(),
		CustomerID:       customerID,
		TaxID:            taxID,
		OverCreditBudget: overCreditBudget,
		SyncedAt:         time.Now().UTC(),
	}
}

func (d *DebtStatus) Update(overCreditBudget bool) {
	d.OverCreditBudget = overCreditBudget
	d.SyncedAt = time.Now().UTC()
}
