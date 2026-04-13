package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProduct_Success(t *testing.T) {
	p, err := NewProduct("Fiber 100Mbps", "100 Mbps fiber connection", "internet", 2999, "EUR", "monthly")
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "Fiber 100Mbps", p.Name)
	assert.Equal(t, "100 Mbps fiber connection", p.Description)
	assert.Equal(t, TypeInternet, p.Type)
	assert.Equal(t, int64(2999), p.Price.Amount)
	assert.Equal(t, "EUR", p.Price.Currency)
	assert.Equal(t, BillingMonthly, p.BillingPeriod)
	assert.True(t, p.IsActive)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())
}

func TestNewProduct_EmptyName(t *testing.T) {
	_, err := NewProduct("", "desc", "internet", 1000, "EUR", "monthly")
	assert.ErrorIs(t, err, ErrNameRequired)
}

func TestNewProduct_WhitespaceName(t *testing.T) {
	_, err := NewProduct("   ", "desc", "internet", 1000, "EUR", "monthly")
	assert.ErrorIs(t, err, ErrNameRequired)
}

func TestNewProduct_InvalidType(t *testing.T) {
	_, err := NewProduct("Test", "desc", "invalid", 1000, "EUR", "monthly")
	assert.ErrorIs(t, err, ErrInvalidType)
}

func TestNewProduct_InvalidBillingPeriod(t *testing.T) {
	_, err := NewProduct("Test", "desc", "internet", 1000, "EUR", "weekly")
	assert.ErrorIs(t, err, ErrInvalidBilling)
}

func TestNewProduct_AllTypes(t *testing.T) {
	types := []string{"internet", "voip", "hosting", "custom"}
	for _, pt := range types {
		p, err := NewProduct("Test", "", pt, 1000, "EUR", "monthly")
		require.NoError(t, err)
		assert.Equal(t, ProductType(pt), p.Type)
	}
}

func TestNewProduct_AllBillingPeriods(t *testing.T) {
	periods := []string{"monthly", "quarterly", "yearly"}
	for _, bp := range periods {
		p, err := NewProduct("Test", "", "internet", 1000, "EUR", bp)
		require.NoError(t, err)
		assert.Equal(t, BillingPeriod(bp), p.BillingPeriod)
	}
}

func TestProduct_Update(t *testing.T) {
	p, err := NewProduct("Old Name", "old desc", "internet", 1000, "EUR", "monthly")
	require.NoError(t, err)

	oldUpdated := p.UpdatedAt

	err = p.Update("New Name", "new desc", "voip", 5000, "USD", "yearly")
	assert.NoError(t, err)
	assert.Equal(t, "New Name", p.Name)
	assert.Equal(t, "new desc", p.Description)
	assert.Equal(t, TypeVoIP, p.Type)
	assert.Equal(t, int64(5000), p.Price.Amount)
	assert.Equal(t, "USD", p.Price.Currency)
	assert.Equal(t, BillingYearly, p.BillingPeriod)
	assert.True(t, p.UpdatedAt.After(oldUpdated) || p.UpdatedAt.Equal(oldUpdated))
}

func TestProduct_Update_EmptyName(t *testing.T) {
	p, err := NewProduct("Name", "desc", "internet", 1000, "EUR", "monthly")
	require.NoError(t, err)

	err = p.Update("", "desc", "internet", 1000, "EUR", "monthly")
	assert.ErrorIs(t, err, ErrNameRequired)
}

func TestProduct_Update_InvalidType(t *testing.T) {
	p, err := NewProduct("Name", "desc", "internet", 1000, "EUR", "monthly")
	require.NoError(t, err)

	err = p.Update("Name", "desc", "bad", 1000, "EUR", "monthly")
	assert.ErrorIs(t, err, ErrInvalidType)
}

func TestProduct_Update_InvalidBilling(t *testing.T) {
	p, err := NewProduct("Name", "desc", "internet", 1000, "EUR", "monthly")
	require.NoError(t, err)

	err = p.Update("Name", "desc", "internet", 1000, "EUR", "bad")
	assert.ErrorIs(t, err, ErrInvalidBilling)
}

func TestProduct_Deactivate(t *testing.T) {
	p, err := NewProduct("Name", "", "internet", 1000, "EUR", "monthly")
	require.NoError(t, err)
	assert.True(t, p.IsActive)

	err = p.Deactivate()
	assert.NoError(t, err)
	assert.False(t, p.IsActive)
}

func TestProduct_Deactivate_AlreadyInactive(t *testing.T) {
	p, err := NewProduct("Name", "", "internet", 1000, "EUR", "monthly")
	require.NoError(t, err)
	_ = p.Deactivate()

	err = p.Deactivate()
	assert.ErrorIs(t, err, ErrAlreadyInactive)
}

func TestProduct_Activate(t *testing.T) {
	p, err := NewProduct("Name", "", "internet", 1000, "EUR", "monthly")
	require.NoError(t, err)
	_ = p.Deactivate()

	err = p.Activate()
	assert.NoError(t, err)
	assert.True(t, p.IsActive)
}

func TestProduct_Activate_AlreadyActive(t *testing.T) {
	p, err := NewProduct("Name", "", "internet", 1000, "EUR", "monthly")
	require.NoError(t, err)

	err = p.Activate()
	assert.ErrorIs(t, err, ErrAlreadyActive)
}
