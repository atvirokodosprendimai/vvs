package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validLineItem(id string) LineItem {
	return LineItem{
		ID:          id,
		ProductID:   "prod-1",
		ProductName: "Fiber 100Mbps",
		Description: "Monthly subscription",
		Quantity:    2,
		UnitPrice:   1500, // 15.00 EUR
		TotalPrice:  3000, // 30.00 EUR
	}
}

func TestNewInvoice_CreatesDraft(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")

	assert.Equal(t, "inv-1", inv.ID)
	assert.Equal(t, "cust-1", inv.CustomerID)
	assert.Equal(t, "ACME Corp", inv.CustomerName)
	assert.Equal(t, "INV-001", inv.Code)
	assert.Equal(t, StatusDraft, inv.Status)
	assert.Equal(t, "EUR", inv.Currency)
	assert.Empty(t, inv.LineItems)
	assert.Equal(t, int64(0), inv.TotalAmount)
	assert.Nil(t, inv.PaidAt)
	assert.False(t, inv.CreatedAt.IsZero())
	assert.False(t, inv.UpdatedAt.IsZero())
}

func TestAddLineItem_InDraft_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	item := validLineItem("li-1")

	err := inv.AddLineItem(item)

	require.NoError(t, err)
	assert.Len(t, inv.LineItems, 1)
	assert.Equal(t, "li-1", inv.LineItems[0].ID)
}

func TestAddLineItem_InFinalized_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.AddLineItem(validLineItem("li-2"))

	assert.ErrorIs(t, err, ErrInvoiceNotDraft)
}

func TestRemoveLineItem_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.AddLineItem(validLineItem("li-2"))

	err := inv.RemoveLineItem("li-1")

	require.NoError(t, err)
	assert.Len(t, inv.LineItems, 1)
	assert.Equal(t, "li-2", inv.LineItems[0].ID)
}

func TestRemoveLineItem_UnknownID_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))

	err := inv.RemoveLineItem("li-999")

	assert.ErrorIs(t, err, ErrLineItemNotFound)
}

func TestRemoveLineItem_NotDraft_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.RemoveLineItem("li-1")

	assert.ErrorIs(t, err, ErrInvoiceNotDraft)
}

func TestRecalculate_SumsCorrectly(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")

	item1 := LineItem{
		ID:        "li-1",
		Quantity:  2,
		UnitPrice: 1000,
	}
	item2 := LineItem{
		ID:        "li-2",
		Quantity:  3,
		UnitPrice: 500,
	}
	_ = inv.AddLineItem(item1)
	_ = inv.AddLineItem(item2)

	inv.Recalculate()

	// item1: 2*1000=2000, item2: 3*500=1500, total=3500
	assert.Equal(t, int64(2000), inv.LineItems[0].TotalPrice)
	assert.Equal(t, int64(1500), inv.LineItems[1].TotalPrice)
	assert.Equal(t, int64(3500), inv.TotalAmount)
}

func TestFinalize_WithItems_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))

	err := inv.Finalize()

	require.NoError(t, err)
	assert.Equal(t, StatusFinalized, inv.Status)
}

func TestFinalize_WithNoItems_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")

	err := inv.Finalize()

	assert.ErrorIs(t, err, ErrNoLineItems)
}

func TestFinalize_FromNonDraft_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.Finalize()

	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestMarkPaid_FromFinalized_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.MarkPaid()

	require.NoError(t, err)
	assert.Equal(t, StatusPaid, inv.Status)
	assert.NotNil(t, inv.PaidAt)
}

func TestMarkPaid_FromDraft_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")

	err := inv.MarkPaid()

	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestVoid_FromDraft_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")

	err := inv.Void()

	require.NoError(t, err)
	assert.Equal(t, StatusVoid, inv.Status)
}

func TestVoid_FromFinalized_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.Void()

	require.NoError(t, err)
	assert.Equal(t, StatusVoid, inv.Status)
}

func TestVoid_FromPaid_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()
	_ = inv.MarkPaid()

	err := inv.Void()

	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestIsOverdue(t *testing.T) {
	t.Run("finalized and past due date is overdue", func(t *testing.T) {
		inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
		_ = inv.AddLineItem(validLineItem("li-1"))
		inv.DueDate = time.Now().Add(-24 * time.Hour) // yesterday
		_ = inv.Finalize()

		assert.True(t, inv.IsOverdue())
	})

	t.Run("finalized but not past due date is not overdue", func(t *testing.T) {
		inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
		_ = inv.AddLineItem(validLineItem("li-1"))
		inv.DueDate = time.Now().Add(24 * time.Hour) // tomorrow
		_ = inv.Finalize()

		assert.False(t, inv.IsOverdue())
	})

	t.Run("paid invoice is not overdue", func(t *testing.T) {
		inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
		_ = inv.AddLineItem(validLineItem("li-1"))
		inv.DueDate = time.Now().Add(-24 * time.Hour) // yesterday
		_ = inv.Finalize()
		_ = inv.MarkPaid()

		assert.False(t, inv.IsOverdue())
	})

	t.Run("draft invoice is not overdue", func(t *testing.T) {
		inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "INV-001")
		inv.DueDate = time.Now().Add(-24 * time.Hour) // yesterday

		assert.False(t, inv.IsOverdue())
	})
}
