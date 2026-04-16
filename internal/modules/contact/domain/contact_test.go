package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContact_Success(t *testing.T) {
	c, err := NewContact("id-1", "cust-1", "Jane", "Doe", "jane@x.com", "+370111", "CEO")
	require.NoError(t, err)
	assert.Equal(t, "Jane", c.FirstName)
	assert.Equal(t, "Doe", c.LastName)
	assert.Equal(t, "Jane Doe", c.FullName())
}

func TestNewContact_RequiresFirstName(t *testing.T) {
	_, err := NewContact("id-1", "cust-1", "", "Doe", "", "", "")
	assert.ErrorIs(t, err, ErrFirstNameRequired)
}

func TestNewContact_TrimsWhitespace(t *testing.T) {
	c, err := NewContact("id-1", "cust-1", "  Jane  ", "  Doe  ", "  jane@x.com  ", "", "")
	require.NoError(t, err)
	assert.Equal(t, "Jane", c.FirstName)
	assert.Equal(t, "Doe", c.LastName)
	assert.Equal(t, "jane@x.com", c.Email)
}

func TestContact_Update(t *testing.T) {
	c, _ := NewContact("id-1", "cust-1", "Jane", "Doe", "jane@x.com", "", "CEO")
	err := c.Update("John", "Smith", "john@x.com", "+1234", "CTO", "some notes")
	require.NoError(t, err)
	assert.Equal(t, "John", c.FirstName)
	assert.Equal(t, "CTO", c.Role)
	assert.Equal(t, "some notes", c.Notes)
}

func TestContact_Update_RequiresFirstName(t *testing.T) {
	c, _ := NewContact("id-1", "cust-1", "Jane", "", "", "", "")
	err := c.Update("", "", "", "", "", "")
	assert.ErrorIs(t, err, ErrFirstNameRequired)
}

func TestContact_FullName_NoLastName(t *testing.T) {
	c, _ := NewContact("id-1", "cust-1", "Jane", "", "", "", "")
	assert.Equal(t, "Jane", c.FullName())
}
