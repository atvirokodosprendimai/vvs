package domain

import "context"

type DebtRepository interface {
	// Upsert inserts or updates the debt status for a customer (keyed by CustomerID).
	Upsert(ctx context.Context, status *DebtStatus) error
	ListAll(ctx context.Context) ([]*DebtStatus, error)
}
