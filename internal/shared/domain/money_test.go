package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoney_Add(t *testing.T) {
	a := EUR(1000)
	b := EUR(500)
	result, err := a.Add(b)
	assert.NoError(t, err)
	assert.Equal(t, int64(1500), result.Amount)
	assert.Equal(t, "EUR", result.Currency)
}

func TestMoney_Add_CurrencyMismatch(t *testing.T) {
	a := EUR(1000)
	b := NewMoney(500, "USD")
	_, err := a.Add(b)
	assert.ErrorIs(t, err, ErrCurrencyMismatch)
}

func TestMoney_Subtract(t *testing.T) {
	a := EUR(1000)
	b := EUR(300)
	result, err := a.Subtract(b)
	assert.NoError(t, err)
	assert.Equal(t, int64(700), result.Amount)
}

func TestMoney_Multiply(t *testing.T) {
	m := EUR(500)
	result := m.Multiply(3)
	assert.Equal(t, int64(1500), result.Amount)
}

func TestMoney_Display(t *testing.T) {
	assert.Equal(t, "12.50 EUR", EUR(1250).Display())
	assert.Equal(t, "0.99 EUR", EUR(99).Display())
	assert.Equal(t, "100.00 EUR", EUR(10000).Display())
}

func TestMoney_IsZero(t *testing.T) {
	assert.True(t, EUR(0).IsZero())
	assert.False(t, EUR(1).IsZero())
}

func TestMoney_IsNegative(t *testing.T) {
	assert.True(t, EUR(-100).IsNegative())
	assert.False(t, EUR(100).IsNegative())
}
