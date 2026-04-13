package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/shared/domain"
)

func TestNewCustomer_Success(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, err := NewCustomer(code, "ACME ISP", "John Doe", "john@acme.com", "+3701234567")

	require.NoError(t, err)
	assert.NotEmpty(t, c.ID)
	assert.Equal(t, "CLI-00001", c.Code.String())
	assert.Equal(t, "ACME ISP", c.CompanyName)
	assert.Equal(t, "John Doe", c.ContactName)
	assert.Equal(t, StatusActive, c.Status)
}

func TestNewCustomer_RequiresCompanyName(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	_, err := NewCustomer(code, "", "John", "john@x.com", "")

	assert.ErrorIs(t, err, ErrCompanyNameRequired)
}

func TestNewCustomer_TrimsWhitespace(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, err := NewCustomer(code, "  ACME  ", "  John  ", "  john@x.com  ", "")

	require.NoError(t, err)
	assert.Equal(t, "ACME", c.CompanyName)
	assert.Equal(t, "John", c.ContactName)
	assert.Equal(t, "john@x.com", c.Email)
}

func TestCustomer_Update(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "John", "j@x.com", "")

	err := c.Update("ACME Corp", "Jane", "jane@x.com", "+370111", "Main St", "Vilnius", "01001", "LT", "LT123", "VIP")
	require.NoError(t, err)
	assert.Equal(t, "ACME Corp", c.CompanyName)
	assert.Equal(t, "Jane", c.ContactName)
	assert.Equal(t, "Vilnius", c.City)
	assert.Equal(t, "VIP", c.Notes)
}

func TestCustomer_Update_EmptyName(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")

	err := c.Update("", "", "", "", "", "", "", "", "", "")
	assert.ErrorIs(t, err, ErrCompanyNameRequired)
}

func TestCustomer_Suspend(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")

	err := c.Suspend()
	require.NoError(t, err)
	assert.Equal(t, StatusSuspended, c.Status)
}

func TestCustomer_Suspend_AlreadySuspended(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	c.Suspend()

	err := c.Suspend()
	assert.ErrorIs(t, err, ErrAlreadySuspended)
}

func TestCustomer_Activate(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	c.Suspend()

	err := c.Activate()
	require.NoError(t, err)
	assert.Equal(t, StatusActive, c.Status)
}

func TestCustomer_Activate_AlreadyActive(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")

	err := c.Activate()
	assert.ErrorIs(t, err, ErrAlreadyActive)
}
