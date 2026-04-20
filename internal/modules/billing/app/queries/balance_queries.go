package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/billing/domain"
)

// BalanceReadModel is the query-side view of a customer's balance.
type BalanceReadModel struct {
	CustomerID   string
	BalanceCents int64
	// BalanceEur is a convenience display string, e.g. "€5.00"
}

// LedgerEntryReadModel is the query-side view of a ledger entry.
type LedgerEntryReadModel struct {
	ID              string
	Type            string
	AmountCents     int64
	Description     string
	StripeSessionID string
	CreatedAt       string // formatted
}

// GetCustomerBalanceHandler returns current balance + recent ledger.
type GetCustomerBalanceHandler struct {
	balanceRepo domain.BalanceRepository
}

func NewGetCustomerBalanceHandler(balanceRepo domain.BalanceRepository) *GetCustomerBalanceHandler {
	return &GetCustomerBalanceHandler{balanceRepo: balanceRepo}
}

func (h *GetCustomerBalanceHandler) Handle(ctx context.Context, customerID string) (*BalanceReadModel, error) {
	cents, err := h.balanceRepo.GetBalance(ctx, customerID)
	if err != nil {
		return nil, err
	}
	return &BalanceReadModel{
		CustomerID:   customerID,
		BalanceCents: cents,
	}, nil
}
