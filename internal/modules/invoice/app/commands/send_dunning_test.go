package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	customerdomain "github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
	sharedomain "github.com/atvirokodosprendimai/vvs/internal/shared/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"
)

// ── mocks ──────────────────────────────────────────────────────────────────

type mockInvoiceRepo struct {
	overdue []*domain.Invoice
	saved   []*domain.Invoice
}

func (m *mockInvoiceRepo) Save(_ context.Context, inv *domain.Invoice) error {
	m.saved = append(m.saved, inv)
	return nil
}
func (m *mockInvoiceRepo) FindByID(_ context.Context, id string) (*domain.Invoice, error) {
	return nil, domain.ErrInvoiceNotFound
}
func (m *mockInvoiceRepo) ListByCustomer(_ context.Context, _ string) ([]*domain.Invoice, error) {
	return nil, nil
}
func (m *mockInvoiceRepo) ListAll(_ context.Context) ([]*domain.Invoice, error) { return nil, nil }
func (m *mockInvoiceRepo) NextCode(_ context.Context) (string, error)           { return "", nil }
func (m *mockInvoiceRepo) FindByCode(_ context.Context, _ string) (*domain.Invoice, error) {
	return nil, domain.ErrInvoiceNotFound
}
func (m *mockInvoiceRepo) ListOverdue(_ context.Context) ([]*domain.Invoice, error) {
	return m.overdue, nil
}

type mockCustomerRepo struct {
	customers map[string]*customerdomain.Customer
}

func (m *mockCustomerRepo) NextCode(_ context.Context) (sharedomain.CompanyCode, error) {
	return sharedomain.CompanyCode{}, nil
}
func (m *mockCustomerRepo) Save(_ context.Context, _ *customerdomain.Customer) error { return nil }
func (m *mockCustomerRepo) FindByID(_ context.Context, id string) (*customerdomain.Customer, error) {
	if c, ok := m.customers[id]; ok {
		return c, nil
	}
	return nil, customerdomain.ErrCustomerNotFound
}
func (m *mockCustomerRepo) FindByCode(_ context.Context, _ string) (*customerdomain.Customer, error) {
	return nil, customerdomain.ErrCustomerNotFound
}
func (m *mockCustomerRepo) FindAll(_ context.Context, _ customerdomain.CustomerFilter, _ sharedomain.Pagination) ([]*customerdomain.Customer, int64, error) {
	return nil, 0, nil
}
func (m *mockCustomerRepo) Delete(_ context.Context, _ string) error { return nil }

type mockMailer struct {
	sent []string // "to:subject"
	err  error
}

func (m *mockMailer) SendPlain(_ context.Context, to, subject, _ string) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, to+":"+subject)
	return nil
}

// ── helpers ────────────────────────────────────────────────────────────────

func overdueInvoice(id, code, customerID string) *domain.Invoice {
	inv := domain.NewInvoice(id, customerID, "ACME Corp", "ACM-001", code)
	inv.Status = domain.StatusFinalized
	inv.DueDate = time.Now().Add(-48 * time.Hour)
	inv.TotalAmount = 10000
	return inv
}

func testCustomer(id string) *customerdomain.Customer {
	c, _ := customerdomain.NewCustomer(
		sharedomain.NewCompanyCode("ACM", 1), "ACME Corp", "John Doe", "john@acme.lt", "+37060000000",
	)
	c.ID = id
	return c
}

// ── tests ──────────────────────────────────────────────────────────────────

func TestSendDunningReminders_SendsToOverdueWithEmail(t *testing.T) {
	inv := overdueInvoice("inv-1", "INV-001", "cust-1")
	cust := testCustomer("cust-1")

	repo := &mockInvoiceRepo{overdue: []*domain.Invoice{inv}}
	custRepo := &mockCustomerRepo{customers: map[string]*customerdomain.Customer{"cust-1": cust}}
	mailer := &mockMailer{}

	h := commands.NewSendDunningRemindersHandler(repo, custRepo, mailer)
	result, err := h.Handle(context.Background(), commands.SendDunningRemindersCommand{Interval: time.Millisecond})

	require.NoError(t, err)
	assert.Equal(t, []string{"INV-001"}, result.Sent)
	assert.Empty(t, result.Errors)
	assert.Len(t, mailer.sent, 1)
	assert.Contains(t, mailer.sent[0], "john@acme.lt")
	assert.NotNil(t, inv.ReminderSentAt)
	assert.Len(t, repo.saved, 1)
}

func TestSendDunningReminders_SkipsRecentlyReminded(t *testing.T) {
	inv := overdueInvoice("inv-1", "INV-001", "cust-1")
	recent := time.Now().Add(-1 * time.Hour)
	inv.ReminderSentAt = &recent

	repo := &mockInvoiceRepo{overdue: []*domain.Invoice{inv}}
	custRepo := &mockCustomerRepo{customers: map[string]*customerdomain.Customer{}}
	mailer := &mockMailer{}

	h := commands.NewSendDunningRemindersHandler(repo, custRepo, mailer)
	result, err := h.Handle(context.Background(), commands.SendDunningRemindersCommand{Interval: 24 * time.Hour})

	require.NoError(t, err)
	assert.Empty(t, result.Sent)
	assert.Empty(t, mailer.sent)
}

func TestSendDunningReminders_SkipsNoEmail(t *testing.T) {
	inv := overdueInvoice("inv-1", "INV-001", "cust-1")
	c, _ := customerdomain.NewCustomer(sharedomain.NewCompanyCode("ACM", 1), "ACME", "John", "", "+370")
	c.ID = "cust-1"

	repo := &mockInvoiceRepo{overdue: []*domain.Invoice{inv}}
	custRepo := &mockCustomerRepo{customers: map[string]*customerdomain.Customer{"cust-1": c}}
	mailer := &mockMailer{}

	h := commands.NewSendDunningRemindersHandler(repo, custRepo, mailer)
	result, err := h.Handle(context.Background(), commands.SendDunningRemindersCommand{Interval: time.Millisecond})

	require.NoError(t, err)
	assert.Empty(t, result.Sent)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "no email")
}

func TestSendDunningReminders_PartialMailerError(t *testing.T) {
	inv1 := overdueInvoice("inv-1", "INV-001", "cust-1")
	inv2 := overdueInvoice("inv-2", "INV-002", "cust-1")
	cust := testCustomer("cust-1")

	repo := &mockInvoiceRepo{overdue: []*domain.Invoice{inv1, inv2}}
	custRepo := &mockCustomerRepo{customers: map[string]*customerdomain.Customer{"cust-1": cust}}
	callCount := 0
	mailer := &mockMailer{}

	// fail on second call
	h := commands.NewSendDunningRemindersHandler(repo, custRepo, mailer)
	_ = h // use custom mailer inline via wrapper

	type failOnSecond struct{ *mockMailer; n int }
	failMailer := &struct {
		mockMailer
		n int
	}{}
	hh := commands.NewSendDunningRemindersHandler(repo, custRepo, &failMailerImpl{base: mailer, failAfter: 1, callCount: &callCount})
	result, err := hh.Handle(context.Background(), commands.SendDunningRemindersCommand{Interval: time.Millisecond})
	require.NoError(t, err)
	_ = failOnSecond{}
	_ = failMailer
	assert.Len(t, result.Sent, 1)
	assert.Len(t, result.Errors, 1)
}

type failMailerImpl struct {
	base      *mockMailer
	failAfter int
	callCount *int
}

func (f *failMailerImpl) SendPlain(ctx context.Context, to, subject, body string) error {
	*f.callCount++
	if *f.callCount > f.failAfter {
		return assert.AnError
	}
	return f.base.SendPlain(ctx, to, subject, body)
}

func TestSendDunningReminders_EmptyOverdue(t *testing.T) {
	repo := &mockInvoiceRepo{overdue: []*domain.Invoice{}}
	custRepo := &mockCustomerRepo{}
	mailer := &mockMailer{}

	h := commands.NewSendDunningRemindersHandler(repo, custRepo, mailer)
	result, err := h.Handle(context.Background(), commands.SendDunningRemindersCommand{})

	require.NoError(t, err)
	assert.Empty(t, result.Sent)
	assert.Empty(t, result.Errors)
}
