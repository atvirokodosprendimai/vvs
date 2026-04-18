package queries

import "time"

// ServiceReadModel is the flattened read model for the service list view.
type ServiceReadModel struct {
	ID              string
	CustomerID      string
	ProductID       string
	ProductName     string
	PriceAmount     int64
	Currency        string
	StartDate       time.Time
	Status          string
	BillingCycle    string
	NextBillingDate *time.Time
	CreatedAt       time.Time
}
