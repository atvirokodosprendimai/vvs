package itaxtlt

import "context"

// StubDebtorProvider returns an empty list.
// Use this until real itax.lt API credentials are available.
type StubDebtorProvider struct{}

func NewStubDebtorProvider() *StubDebtorProvider {
	return &StubDebtorProvider{}
}

func (s *StubDebtorProvider) FetchDebtors(_ context.Context) ([]DebtorRecord, error) {
	return []DebtorRecord{}, nil
}
