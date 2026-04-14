package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE customers (
		id TEXT PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		company_name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create customers table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE products (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		price_amount INTEGER NOT NULL DEFAULT 0,
		price_currency TEXT NOT NULL DEFAULT 'EUR',
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create products table: %v", err)
	}
	return db
}

func insertTestCustomer(t *testing.T, db *gorm.DB, id, code, name string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO customers (id, code, company_name, status, created_at, updated_at) VALUES (?, ?, ?, 'active', ?, ?)`,
		id, code, name, time.Now(), time.Now(),
	).Error; err != nil {
		t.Fatalf("insert customer: %v", err)
	}
}

func insertTestProduct(t *testing.T, db *gorm.DB, id, name string, priceAmount int64) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO products (id, name, price_amount, price_currency, is_active, created_at, updated_at) VALUES (?, ?, ?, 'EUR', 1, ?, ?)`,
		id, name, priceAmount, time.Now(), time.Now(),
	).Error; err != nil {
		t.Fatalf("insert product: %v", err)
	}
}

// sseGETRequest creates a GET request with Datastar signals as ?datastar= query param.
// The path may already contain query params (e.g. "?id=X&line=0") — signals are added alongside.
func sseGETRequest(path, signalsJSON string) *http.Request {
	u, _ := url.Parse(path)
	q := u.Query()
	q.Set("datastar", signalsJSON)
	u.RawQuery = q.Encode()
	return httptest.NewRequest("GET", u.String(), nil)
}

// --- Customer autocomplete tests ---

func TestCustomerAutocomplete_EmptyQuery(t *testing.T) {
	db := newTestDB(t)
	insertTestCustomer(t, db, "c1", "CLI-00001", "Acme Corp")

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/customers", `{"customerSearch":""}`)
	w := httptest.NewRecorder()

	h.customerAutocompleteSSE(w, req)

	body := w.Body.String()
	if strings.Contains(body, "Acme Corp") {
		t.Error("empty query should return no suggestions")
	}
}

func TestCustomerAutocomplete_MatchingQuery(t *testing.T) {
	db := newTestDB(t)
	insertTestCustomer(t, db, "c1", "CLI-00001", "Acme Corp")
	insertTestCustomer(t, db, "c2", "CLI-00002", "Beta LLC")

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/customers", `{"customerSearch":"Acme"}`)
	w := httptest.NewRecorder()

	h.customerAutocompleteSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Acme Corp") {
		t.Errorf("expected Acme Corp in response; body: %s", body)
	}
	if strings.Contains(body, "Beta LLC") {
		t.Error("Beta LLC should not appear for 'Acme' query")
	}
}

func TestCustomerAutocomplete_CaseInsensitive(t *testing.T) {
	db := newTestDB(t)
	insertTestCustomer(t, db, "c1", "CLI-00001", "Internet Service Ltd")

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/customers", `{"customerSearch":"internet"}`)
	w := httptest.NewRecorder()

	h.customerAutocompleteSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Internet Service Ltd") {
		t.Errorf("expected case-insensitive match; body: %s", body)
	}
}

func TestCustomerAutocomplete_MatchesByCode(t *testing.T) {
	db := newTestDB(t)
	insertTestCustomer(t, db, "c1", "CLI-00042", "Some Company")

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/customers", `{"customerSearch":"CLI-00042"}`)
	w := httptest.NewRecorder()

	h.customerAutocompleteSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Some Company") {
		t.Errorf("expected match by code; body: %s", body)
	}
}

// --- Customer select tests ---

func TestCustomerSelect_SetsSignals(t *testing.T) {
	db := newTestDB(t)
	insertTestCustomer(t, db, "c1", "CLI-00001", "Acme Corp")

	h := &Handlers{reader: db}
	req := httptest.NewRequest("GET", "/api/autocomplete/customers/select?id=c1", nil)
	w := httptest.NewRecorder()

	h.customerSelectSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Acme Corp") {
		t.Errorf("expected Acme Corp in signals patch; body: %s", body)
	}
	if !strings.Contains(body, "customerId") {
		t.Errorf("expected customerId field in signals patch; body: %s", body)
	}
	if !strings.Contains(body, "c1") {
		t.Errorf("expected customer id c1 in signals patch; body: %s", body)
	}
}

func TestCustomerSelect_ClearsDropdown(t *testing.T) {
	db := newTestDB(t)
	insertTestCustomer(t, db, "c1", "CLI-00001", "Acme Corp")

	h := &Handlers{reader: db}
	req := httptest.NewRequest("GET", "/api/autocomplete/customers/select?id=c1", nil)
	w := httptest.NewRecorder()

	h.customerSelectSSE(w, req)

	body := w.Body.String()
	// Should send PatchElements for customer-ac to clear it
	if !strings.Contains(body, "customer-ac") {
		t.Errorf("expected customer-ac element patch in response; body: %s", body)
	}
}

// --- Product autocomplete tests ---

func TestProductAutocomplete_EmptyQuery(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "Internet 100M", 2999)

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/products?line=0", `{"line0Query":""}`)
	w := httptest.NewRecorder()

	h.productAutocompleteSSE(w, req)

	body := w.Body.String()
	if strings.Contains(body, "Internet 100M") {
		t.Error("empty query should return no suggestions")
	}
}

func TestProductAutocomplete_MatchingQuery(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "Internet 100M", 2999)
	insertTestProduct(t, db, "p2", "VoIP Service", 999)

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/products?line=0", `{"line0Query":"Internet"}`)
	w := httptest.NewRecorder()

	h.productAutocompleteSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Internet 100M") {
		t.Errorf("expected Internet 100M in response; body: %s", body)
	}
	if strings.Contains(body, "VoIP Service") {
		t.Error("VoIP Service should not appear for 'Internet' query")
	}
}

func TestProductAutocomplete_Line1Query(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "VoIP Service", 999)

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/products?line=1", `{"line1Query":"VoIP"}`)
	w := httptest.NewRecorder()

	h.productAutocompleteSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "VoIP Service") {
		t.Errorf("expected VoIP Service for line1 query; body: %s", body)
	}
	// Dropdown target should be product-ac-1
	if !strings.Contains(body, "product-ac-1") {
		t.Errorf("expected product-ac-1 in response; body: %s", body)
	}
}

// --- Product select tests ---

func TestProductSelect_SetsSignals(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "Internet 100M", 2999)

	h := &Handlers{reader: db}
	signalsJSON := `{"line0Query":"","line1Query":"","line2Query":"","lines":[{"productId":"","productName":"","description":"","quantity":1,"unitPrice":""},{"productId":"","productName":"","description":"","quantity":1,"unitPrice":""},{"productId":"","productName":"","description":"","quantity":1,"unitPrice":""}]}`
	req := sseGETRequest("/api/autocomplete/products/select?id=p1&line=0", signalsJSON)
	w := httptest.NewRecorder()

	h.productSelectSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Internet 100M") {
		t.Errorf("expected product name in signals patch; body: %s", body)
	}
	if !strings.Contains(body, "29.99") {
		t.Errorf("expected unit price 29.99 in signals patch; body: %s", body)
	}
}

func TestProductSelect_ClearsDropdown(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "Internet 100M", 2999)

	h := &Handlers{reader: db}
	signalsJSON := `{"line0Query":"","line1Query":"","line2Query":"","lines":[{"productId":"","productName":"","description":"","quantity":1,"unitPrice":""}]}`
	req := sseGETRequest("/api/autocomplete/products/select?id=p1&line=0", signalsJSON)
	w := httptest.NewRecorder()

	h.productSelectSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "product-ac-0") {
		t.Errorf("expected product-ac-0 element patch in response; body: %s", body)
	}
}
