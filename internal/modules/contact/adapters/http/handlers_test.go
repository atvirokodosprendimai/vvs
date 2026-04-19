package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	contacthttp "github.com/atvirokodosprendimai/vvs/internal/modules/contact/adapters/http"
	contactpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/contact/adapters/persistence"
	contactcommands "github.com/atvirokodosprendimai/vvs/internal/modules/contact/app/commands"
	contactqueries "github.com/atvirokodosprendimai/vvs/internal/modules/contact/app/queries"
	contactmigrations "github.com/atvirokodosprendimai/vvs/internal/modules/contact/migrations"
	customermigrations "github.com/atvirokodosprendimai/vvs/internal/modules/customer/migrations"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

// ── stubs ────────────────────────────────────────────────────────────────────

type contactNoopPub struct{}

func (n *contactNoopPub) Publish(_ context.Context, _ string, _ events.DomainEvent) error {
	return nil
}

type contactNoopSub struct{}

func (n *contactNoopSub) Subscribe(_ string, _ events.EventHandler) error { return nil }
func (n *contactNoopSub) ChanSubscription(_ string) (<-chan events.DomainEvent, func()) {
	ch := make(chan events.DomainEvent)
	close(ch)
	return ch, func() {}
}
func (n *contactNoopSub) Close() error { return nil }

// ── helpers ──────────────────────────────────────────────────────────────────

const testCustomerID = "cust-http-test-001"

func contactRouter(t *testing.T) (http.Handler, *contactpersistence.GormContactRepository) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, customermigrations.FS, "goose_customer")
	testutil.RunMigrations(t, db, contactmigrations.FS, "goose_contact")

	// Seed customer row to satisfy FK
	err := db.W.Exec(
		"INSERT INTO customers (id, code, company_name, status) VALUES (?, ?, ?, ?)",
		testCustomerID, "CLI-HTTP-001", "HTTP Test Corp", "active",
	).Error
	if err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	repo := contactpersistence.NewGormContactRepository(db)
	pub := &contactNoopPub{}
	sub := &contactNoopSub{}

	h := contacthttp.NewHandlers(
		contactcommands.NewAddContactHandler(repo, pub),
		contactcommands.NewUpdateContactHandler(repo, pub),
		contactcommands.NewDeleteContactHandler(repo, pub),
		contactqueries.NewListContactsForCustomerHandler(db),
		sub,
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, repo
}

func addContactViaHandler(t *testing.T, router http.Handler, firstName string) {
	t.Helper()
	body := `{"contactFirstName":"` + firstName + `","contactLastName":"Test","contactEmail":"test@example.com"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers/"+testCustomerID+"/contacts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestContactListSSE_ContentType(t *testing.T) {
	router, _ := contactRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/customers/"+testCustomerID+"/contacts", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestContactAddSSE_CreatesContact(t *testing.T) {
	router, repo := contactRouter(t)
	addContactViaHandler(t, router, "Alice")

	contacts, err := repo.ListForCustomer(context.Background(), testCustomerID)
	if err != nil {
		t.Fatalf("ListForCustomer: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("want 1 contact, got %d", len(contacts))
	}
	if contacts[0].FirstName != "Alice" {
		t.Fatalf("want 'Alice', got %q", contacts[0].FirstName)
	}
}

func TestContactUpdateSSE_UpdatesContact(t *testing.T) {
	router, repo := contactRouter(t)
	addContactViaHandler(t, router, "Bob")

	contacts, _ := repo.ListForCustomer(context.Background(), testCustomerID)
	if len(contacts) == 0 {
		t.Fatal("contact not created")
	}
	id := contacts[0].ID

	updateBody := `{"contactFirstName":"Bobby","contactLastName":"Updated","contactEmail":"bobby@example.com","contactRole":"tech"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/contacts/"+id, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	updated, _ := repo.FindByID(context.Background(), id)
	if updated.FirstName != "Bobby" {
		t.Fatalf("want 'Bobby', got %q", updated.FirstName)
	}
}

func TestContactDeleteSSE_DeletesContact(t *testing.T) {
	router, repo := contactRouter(t)
	addContactViaHandler(t, router, "Charlie")

	contacts, _ := repo.ListForCustomer(context.Background(), testCustomerID)
	if len(contacts) == 0 {
		t.Fatal("contact not created")
	}
	id := contacts[0].ID

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/contacts/"+id, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	remaining, _ := repo.ListForCustomer(context.Background(), testCustomerID)
	if len(remaining) != 0 {
		t.Fatalf("want 0 contacts after delete, got %d", len(remaining))
	}
}

func TestContactDeleteSSE_NotFound_Returns500(t *testing.T) {
	router, _ := contactRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/contacts/nonexistent", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 for not-found delete, got %d", rr.Code)
	}
}
