package itaxtlt

import "context"

// DebtorRecord is the data returned by itax.lt for a single debtor entry.
// itax.lt responds with client code and a boolean indicating if debt exceeds credit budget.
type DebtorRecord struct {
	ClientCode       string // Lithuanian company registration code
	OverCreditBudget bool   // true if the client's debt exceeds their credit budget
}

// DebtorProvider is the port for fetching the debtors list from itax.lt.
type DebtorProvider interface {
	FetchDebtors(ctx context.Context) ([]DebtorRecord, error)
}
