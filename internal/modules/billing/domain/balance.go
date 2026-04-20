package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// EntryType describes why a ledger entry was created.
type EntryType string

const (
	EntryTypeTopUp      EntryType = "topup"
	EntryTypeVMPurchase EntryType = "vm_purchase"
	EntryTypeRefund     EntryType = "refund"
	EntryTypeAdjustment EntryType = "adjustment"
)

// BalanceLedgerEntry is a single immutable credit or debit event.
type BalanceLedgerEntry struct {
	ID              string
	CustomerID      string
	Type            EntryType
	AmountCents     int64  // positive = credit, negative = debit
	Description     string
	StripeSessionID string // idempotency key for Stripe-originated entries
	CreatedAt       time.Time
}

// CustomerBalance holds the current cached balance for a customer.
type CustomerBalance struct {
	CustomerID  string
	BalanceCents int64
	UpdatedAt   time.Time
}

// BalanceRepository manages customer prepaid balances.
type BalanceRepository interface {
	// GetBalance returns current balance (0 if never set).
	GetBalance(ctx context.Context, customerID string) (int64, error)

	// Credit adds amountCents to the customer's balance atomically.
	// stripeSessionID is stored for idempotency (empty = non-Stripe credit).
	// Returns ErrDuplicateSession if stripeSessionID already credited.
	Credit(ctx context.Context, customerID string, amountCents int64, entryType EntryType, description, stripeSessionID string) error

	// Deduct subtracts amountCents from balance atomically.
	// Returns ErrInsufficientBalance if balance < amountCents.
	Deduct(ctx context.Context, customerID string, amountCents int64, entryType EntryType, description string) error

	// GetLedger returns ledger entries newest-first.
	GetLedger(ctx context.Context, customerID string) ([]*BalanceLedgerEntry, error)
}
