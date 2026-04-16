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

func TestCustomer_SetNetworkInfo(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")

	c.SetNetworkInfo("router-1", "10.0.1.55", "AA:BB:CC:DD:EE:FF")

	assert.NotNil(t, c.RouterID)
	assert.Equal(t, "router-1", *c.RouterID)
	assert.Equal(t, "10.0.1.55", c.IPAddress)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", c.MACAddress)
}

func TestCustomer_SetNetworkInfo_ClearsRouterID(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	c.SetNetworkInfo("router-1", "10.0.1.55", "AA:BB:CC:DD:EE:FF")

	c.SetNetworkInfo("", "", "")

	assert.Nil(t, c.RouterID)
	assert.Empty(t, c.IPAddress)
	assert.Empty(t, c.MACAddress)
}

func TestCustomer_HasNetworkProvisioning(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")

	assert.False(t, c.HasNetworkProvisioning())

	c.SetNetworkInfo("router-1", "10.0.1.55", "AA:BB:CC:DD:EE:FF")
	assert.True(t, c.HasNetworkProvisioning())
}

func TestCustomer_Qualify(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	c.Status = StatusLead

	err := c.Qualify()
	require.NoError(t, err)
	assert.Equal(t, StatusProspect, c.Status)
}

func TestCustomer_Qualify_NotLead(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	// status is active

	err := c.Qualify()
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestCustomer_Convert(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	c.Status = StatusProspect

	err := c.Convert()
	require.NoError(t, err)
	assert.Equal(t, StatusActive, c.Status)
}

func TestCustomer_Convert_NotProspect(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	// status is active

	err := c.Convert()
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestCustomer_Churn(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")

	err := c.Churn()
	require.NoError(t, err)
	assert.Equal(t, StatusChurned, c.Status)
}

func TestCustomer_Churn_AlreadyChurned(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	c.Churn()

	err := c.Churn()
	assert.ErrorIs(t, err, ErrAlreadyChurned)
}

func TestCustomer_Activate_FromChurned_Rejected(t *testing.T) {
	code := domain.NewCompanyCode("CLI", 1)
	c, _ := NewCustomer(code, "ACME", "", "", "")
	c.Status = StatusChurned

	err := c.Activate()
	assert.ErrorIs(t, err, ErrInvalidTransition)
}
