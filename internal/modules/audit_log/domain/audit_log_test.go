package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAuditLog_ValidFields(t *testing.T) {
	changes := json.RawMessage(`{"field":"value"}`)
	al, err := NewAuditLog("actor-1", "Alice", "customer.created", "customer", "cust-42", changes)

	assert.NoError(t, err)
	assert.NotNil(t, al)
	assert.NotEmpty(t, al.ID)
	assert.Equal(t, "actor-1", al.ActorID)
	assert.Equal(t, "Alice", al.ActorName)
	assert.Equal(t, "customer.created", al.Action)
	assert.Equal(t, "customer", al.Resource)
	assert.Equal(t, "cust-42", al.ResourceID)
	assert.Equal(t, changes, al.Changes)
	assert.False(t, al.CreatedAt.IsZero())
}

func TestNewAuditLog_EmptyAction(t *testing.T) {
	al, err := NewAuditLog("actor-1", "Alice", "", "customer", "cust-42", nil)

	assert.Nil(t, al)
	assert.ErrorIs(t, err, ErrMissingAction)
}

func TestNewAuditLog_EmptyResource(t *testing.T) {
	al, err := NewAuditLog("actor-1", "Alice", "customer.created", "", "cust-42", nil)

	assert.Nil(t, al)
	assert.ErrorIs(t, err, ErrMissingResource)
}

func TestNewAuditLog_EmptyResourceID(t *testing.T) {
	al, err := NewAuditLog("actor-1", "Alice", "customer.created", "customer", "", nil)

	assert.Nil(t, al)
	assert.ErrorIs(t, err, ErrMissingResourceID)
}

func TestNewAuditLog_EmptyActorAllowed(t *testing.T) {
	al, err := NewAuditLog("", "", "invoice.finalized", "invoice", "inv-99", nil)

	assert.NoError(t, err)
	assert.NotNil(t, al)
	assert.Empty(t, al.ActorID)
	assert.Empty(t, al.ActorName)
	assert.NotEmpty(t, al.ID)
}
