package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validLineItem(id string) LineItem {
	return LineItem{
		ID:             id,
		ProductID:      "prod-1",
		ProductName:    "Fiber 100Mbps",
		Description:    "Monthly subscription",
		Quantity:       2,
		UnitPriceGross: 1815, // 18.15 EUR gross (15.00 * 1.21)
		VATRate:        21,
	}
}

func TestNewInvoice_CreatesDraft(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")

	assert.Equal(t, "inv-1", inv.ID)
	assert.Equal(t, "cust-1", inv.CustomerID)
	assert.Equal(t, "ACME Corp", inv.CustomerName)
	assert.Equal(t, "ACM-00001", inv.CustomerCode)
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
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	item := validLineItem("li-1")

	err := inv.AddLineItem(item)

	require.NoError(t, err)
	assert.Len(t, inv.LineItems, 1)
	assert.Equal(t, "li-1", inv.LineItems[0].ID)
}

func TestAddLineItem_InFinalized_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.AddLineItem(validLineItem("li-2"))

	assert.ErrorIs(t, err, ErrInvoiceNotDraft)
}

func TestRemoveLineItem_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.AddLineItem(validLineItem("li-2"))

	err := inv.RemoveLineItem("li-1")

	require.NoError(t, err)
	assert.Len(t, inv.LineItems, 1)
	assert.Equal(t, "li-2", inv.LineItems[0].ID)
}

func TestRemoveLineItem_UnknownID_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))

	err := inv.RemoveLineItem("li-999")

	assert.ErrorIs(t, err, ErrLineItemNotFound)
}

func TestRemoveLineItem_NotDraft_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.RemoveLineItem("li-1")

	assert.ErrorIs(t, err, ErrInvoiceNotDraft)
}

func TestRecalculate_WithVAT(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")

	// Item at 21% VAT: gross 12.10 per unit → net 10.00
	item1 := LineItem{
		ID:             "li-1",
		Quantity:       2,
		UnitPriceGross: 1210, // 12.10 EUR gross
		VATRate:        21,
	}
	// Item at 0% VAT: gross = net
	item2 := LineItem{
		ID:             "li-2",
		Quantity:       3,
		UnitPriceGross: 500, // 5.00 EUR, no VAT
		VATRate:        0,
	}
	_ = inv.AddLineItem(item1)
	_ = inv.AddLineItem(item2)

	inv.Recalculate()

	// item1: net/unit=1000, totalGross=2420, totalNet=2000, totalVAT=420
	assert.Equal(t, int64(1000), inv.LineItems[0].UnitPrice)
	assert.Equal(t, int64(2000), inv.LineItems[0].TotalPrice)
	assert.Equal(t, int64(2420), inv.LineItems[0].TotalGross)
	assert.Equal(t, int64(420), inv.LineItems[0].TotalVAT)

	// item2: no VAT, net=gross
	assert.Equal(t, int64(500), inv.LineItems[1].UnitPrice)
	assert.Equal(t, int64(1500), inv.LineItems[1].TotalPrice)
	assert.Equal(t, int64(1500), inv.LineItems[1].TotalGross)
	assert.Equal(t, int64(0), inv.LineItems[1].TotalVAT)

	// Invoice totals: sub=3500, vat=420, total=3920
	assert.Equal(t, int64(3500), inv.SubTotal)
	assert.Equal(t, int64(420), inv.VATTotal)
	assert.Equal(t, int64(3920), inv.TotalAmount)
}

func TestRecalculate_9PercentVAT(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")

	item := LineItem{
		ID:             "li-1",
		Quantity:       1,
		UnitPriceGross: 10900, // 109.00 EUR gross at 9%
		VATRate:        9,
	}
	_ = inv.AddLineItem(item)
	inv.Recalculate()

	// net = 10900 * 100 / 109 = 10000
	assert.Equal(t, int64(10000), inv.LineItems[0].UnitPrice)
	assert.Equal(t, int64(900), inv.LineItems[0].TotalVAT)
	assert.Equal(t, int64(10900), inv.TotalAmount)
}

func TestFinalize_WithItems_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))

	err := inv.Finalize()

	require.NoError(t, err)
	assert.Equal(t, StatusFinalized, inv.Status)
}

func TestFinalize_WithNoItems_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")

	err := inv.Finalize()

	assert.ErrorIs(t, err, ErrNoLineItems)
}

func TestFinalize_FromNonDraft_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.Finalize()

	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestMarkPaid_FromFinalized_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.MarkPaid()

	require.NoError(t, err)
	assert.Equal(t, StatusPaid, inv.Status)
	assert.NotNil(t, inv.PaidAt)
}

func TestMarkPaid_FromDraft_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")

	err := inv.MarkPaid()

	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestVoid_FromDraft_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")

	err := inv.Void()

	require.NoError(t, err)
	assert.Equal(t, StatusVoid, inv.Status)
}

func TestVoid_FromFinalized_Succeeds(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()

	err := inv.Void()

	require.NoError(t, err)
	assert.Equal(t, StatusVoid, inv.Status)
}

func TestVoid_FromPaid_Fails(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(validLineItem("li-1"))
	_ = inv.Finalize()
	_ = inv.MarkPaid()

	err := inv.Void()

	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestIsOverdue(t *testing.T) {
	t.Run("finalized and past due date is overdue", func(t *testing.T) {
		inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
		_ = inv.AddLineItem(validLineItem("li-1"))
		inv.DueDate = time.Now().Add(-24 * time.Hour)
		_ = inv.Finalize()

		assert.True(t, inv.IsOverdue())
	})

	t.Run("finalized but not past due date is not overdue", func(t *testing.T) {
		inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
		_ = inv.AddLineItem(validLineItem("li-1"))
		inv.DueDate = time.Now().Add(24 * time.Hour)
		_ = inv.Finalize()

		assert.False(t, inv.IsOverdue())
	})

	t.Run("paid invoice is not overdue", func(t *testing.T) {
		inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
		_ = inv.AddLineItem(validLineItem("li-1"))
		inv.DueDate = time.Now().Add(-24 * time.Hour)
		_ = inv.Finalize()
		_ = inv.MarkPaid()

		assert.False(t, inv.IsOverdue())
	})

	t.Run("draft invoice is not overdue", func(t *testing.T) {
		inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
		inv.DueDate = time.Now().Add(-24 * time.Hour)

		assert.False(t, inv.IsOverdue())
	})
}

func TestUpdateLineItem_WithVAT(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME Corp", "ACM-00001", "INV-001")
	_ = inv.AddLineItem(LineItem{
		ID:             "li-1",
		ProductName:    "Old",
		Quantity:       1,
		UnitPriceGross: 1000,
		VATRate:        21,
	})

	err := inv.UpdateLineItem("li-1", "New Product", "New desc", 5, 2420, 21)
	require.NoError(t, err)

	assert.Equal(t, "New Product", inv.LineItems[0].ProductName)
	assert.Equal(t, int64(2420), inv.LineItems[0].UnitPriceGross)
	assert.Equal(t, 21, inv.LineItems[0].VATRate)
	assert.Equal(t, 5, inv.LineItems[0].Quantity)
}

// ── Dunning ──────────────────────────────────────────────────────────────────

func TestMarkReminderSent_SetsTimestamp(t *testing.T) {
	inv := NewInvoice("inv-1", "cust-1", "ACME", "ACM-001", "INV-001")
	inv.Status = StatusFinalized
	inv.DueDate = time.Now().Add(-48 * time.Hour)

	err := inv.MarkReminderSent()
	require.NoError(t, err)
	require.NotNil(t, inv.ReminderSentAt)
	assert.WithinDuration(t, time.Now(), *inv.ReminderSentAt, 2*time.Second)
}

func TestMarkReminderSent_OnlyForFinalizedOverdue(t *testing.T) {
	tests := []struct {
		name    string
		status  InvoiceStatus
		dueDate time.Time
		wantErr bool
	}{
		{"finalized overdue", StatusFinalized, time.Now().Add(-1 * time.Hour), false},
		{"finalized not due", StatusFinalized, time.Now().Add(72 * time.Hour), true},
		{"paid", StatusPaid, time.Now().Add(-1 * time.Hour), true},
		{"draft", StatusDraft, time.Now().Add(-1 * time.Hour), true},
		{"void", StatusVoid, time.Now().Add(-1 * time.Hour), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := NewInvoice("i", "c", "N", "C", "INV-X")
			inv.Status = tt.status
			inv.DueDate = tt.dueDate
			err := inv.MarkReminderSent()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNeedsReminder(t *testing.T) {
	now := time.Now()
	past := now.Add(-48 * time.Hour)
	recent := now.Add(-12 * time.Hour)

	inv := NewInvoice("i", "c", "N", "C", "INV-X")
	inv.Status = StatusFinalized
	inv.DueDate = past

	// No reminder sent yet — needs reminder
	assert.True(t, inv.NeedsReminder(24*time.Hour))

	// Reminder sent recently — no reminder
	inv.ReminderSentAt = &recent
	assert.False(t, inv.NeedsReminder(24*time.Hour))

	// Reminder sent long ago — needs reminder
	old := now.Add(-25 * time.Hour)
	inv.ReminderSentAt = &old
	assert.True(t, inv.NeedsReminder(24*time.Hour))

	// Not overdue — no reminder
	inv.ReminderSentAt = nil
	inv.DueDate = now.Add(72 * time.Hour)
	assert.False(t, inv.NeedsReminder(24*time.Hour))
}
