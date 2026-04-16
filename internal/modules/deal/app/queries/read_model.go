package queries

import "time"

// DealReadModel is the flattened read model for the deal list view.
type DealReadModel struct {
	ID         string
	CustomerID string
	Title      string
	Value      int64
	Currency   string
	Stage      string
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
