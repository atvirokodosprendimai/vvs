package domain

import (
	"errors"
	"fmt"
)

var (
	ErrCurrencyMismatch = errors.New("currency mismatch")
	ErrNegativeAmount   = errors.New("amount cannot be negative")
)

type Money struct {
	Amount   int64  // cents
	Currency string // ISO 4217
}

func NewMoney(amount int64, currency string) Money {
	return Money{Amount: amount, Currency: currency}
}

func EUR(cents int64) Money {
	return Money{Amount: cents, Currency: "EUR"}
}

func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{Amount: m.Amount - other.Amount, Currency: m.Currency}, nil
}

func (m Money) Multiply(factor int64) Money {
	return Money{Amount: m.Amount * factor, Currency: m.Currency}
}

func (m Money) IsZero() bool {
	return m.Amount == 0
}

func (m Money) IsNegative() bool {
	return m.Amount < 0
}

func (m Money) Display() string {
	whole := m.Amount / 100
	frac := m.Amount % 100
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%02d %s", whole, frac, m.Currency)
}

func (m Money) DisplayAmount() string {
	whole := m.Amount / 100
	frac := m.Amount % 100
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%02d", whole, frac)
}
