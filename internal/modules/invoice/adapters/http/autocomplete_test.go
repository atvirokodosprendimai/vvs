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
// Existing query params (e.g. ?id=X) are preserved.
func sseGETRequest(path, signalsJSON string) *http.Request {
	u, _ := url.Parse(path)
	q := u.Query()
	q.Set("datastar", signalsJSON)
	u.RawQuery = q.Encode()
	return httptest.NewRequest("GET", u.String(), nil)
}

// sseBodyRequest creates a non-GET request with signals as JSON body (Datastar POST/DELETE format).
func sseBodyRequest(method, path, signalsJSON string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(signalsJSON))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ===== Customer autocomplete =====

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

// ===== Customer select =====

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
	if !strings.Contains(body, "customer-ac") {
		t.Errorf("expected customer-ac element patch in response; body: %s", body)
	}
}

// ===== Product autocomplete =====

func TestProductAutocomplete_EmptyQuery(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "Internet 100M", 2999)

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/products", `{"newLineSearch":""}`)
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
	req := sseGETRequest("/api/autocomplete/products", `{"newLineSearch":"Internet"}`)
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

func TestProductAutocomplete_DropdownTarget(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "VoIP Service", 999)

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/products", `{"newLineSearch":"VoIP"}`)
	w := httptest.NewRecorder()

	h.productAutocompleteSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "new-line-product-ac") {
		t.Errorf("expected new-line-product-ac in response; body: %s", body)
	}
}

// ===== Product select =====

func TestProductSelect_FillsNewLineSignals(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "Internet 100M", 2999)

	h := &Handlers{reader: db}
	req := httptest.NewRequest("GET", "/api/autocomplete/products/select?id=p1", nil)
	w := httptest.NewRecorder()

	h.productSelectSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Internet 100M") {
		t.Errorf("expected product name in signals patch; body: %s", body)
	}
	if !strings.Contains(body, "29.99") {
		t.Errorf("expected unit price 29.99 in signals patch; body: %s", body)
	}
	if !strings.Contains(body, "newLineProductName") {
		t.Errorf("expected newLineProductName in signals patch; body: %s", body)
	}
	if !strings.Contains(body, "newLineProductId") {
		t.Errorf("expected newLineProductId in signals patch; body: %s", body)
	}
}

func TestProductSelect_PatchesAddLineRow(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "Internet 100M", 2999)

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/products/select?id=p1", `{"newLineDescription":"Monthly","newLineQty":2}`)
	w := httptest.NewRecorder()

	h.productSelectSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "new-line-row") {
		t.Errorf("expected #new-line-row element patch; body: %s", body)
	}
	// Product name and price should appear as input values in the row HTML
	if !strings.Contains(body, "Internet 100M") {
		t.Errorf("expected product name in row HTML; body: %s", body)
	}
	if !strings.Contains(body, "29.99") {
		t.Errorf("expected unit price 29.99 in row HTML; body: %s", body)
	}
}

func TestProductSelect_PreservesDescriptionAndQty(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "VoIP Service", 999)

	h := &Handlers{reader: db}
	req := sseGETRequest("/api/autocomplete/products/select?id=p1", `{"newLineDescription":"Custom desc","newLineQty":3}`)
	w := httptest.NewRecorder()

	h.productSelectSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Custom desc") {
		t.Errorf("expected user-entered description preserved; body: %s", body)
	}
	if !strings.Contains(body, `value="3"`) {
		t.Errorf("expected user-entered qty 3 preserved; body: %s", body)
	}
}

func TestProductSelect_ClearsDropdown(t *testing.T) {
	db := newTestDB(t)
	insertTestProduct(t, db, "p1", "Internet 100M", 2999)

	h := &Handlers{reader: db}
	req := httptest.NewRequest("GET", "/api/autocomplete/products/select?id=p1", nil)
	w := httptest.NewRecorder()

	h.productSelectSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "new-line-product-ac") {
		t.Errorf("expected new-line-product-ac element patch in response; body: %s", body)
	}
}

// ===== Add line =====

func TestAddLine_ValidLine(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("POST", "/api/invoices/lines",
		`{"lines":[],"newLineSearch":"Inter","newLineProductId":"p1","newLineProductName":"Internet 100M","newLineDescription":"Monthly","newLineQty":2,"newLineUnitPrice":"29.99"}`)
	w := httptest.NewRecorder()

	h.addLineSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Internet 100M") {
		t.Errorf("expected product name in table; body: %s", body)
	}
	// Add-form signals should be cleared
	if !strings.Contains(body, `"newLineProductName":""`) {
		t.Errorf("expected newLineProductName cleared; body: %s", body)
	}
	if !strings.Contains(body, `"newLineSearch":""`) {
		t.Errorf("expected newLineSearch cleared; body: %s", body)
	}
	// UnitPrice stored as int64 cents (2999) not string "29.99"
	if !strings.Contains(body, `"unitPrice":2999`) {
		t.Errorf("expected unitPrice stored as 2999 cents; body: %s", body)
	}
}

func TestAddLine_UnitPriceStoredAsCents(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("POST", "/api/invoices/lines",
		`{"lines":[],"newLineProductName":"Widget","newLineQty":1,"newLineUnitPrice":"49.99"}`)
	w := httptest.NewRecorder()

	h.addLineSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `"unitPrice":4999`) {
		t.Errorf("expected 49.99 stored as 4999 cents; body: %s", body)
	}
	if strings.Contains(body, `"unitPrice":"`) {
		t.Errorf("unitPrice must not be a JSON string; body: %s", body)
	}
}

func TestAddLine_EmptyProductName(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("POST", "/api/invoices/lines",
		`{"lines":[],"newLineProductId":"","newLineProductName":"","newLineDescription":"","newLineQty":1,"newLineUnitPrice":""}`)
	w := httptest.NewRecorder()

	h.addLineSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "required") {
		t.Errorf("expected validation error for empty product name; body: %s", body)
	}
}

func TestAddLine_PreservesExistingLines(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("POST", "/api/invoices/lines",
		`{"lines":[{"productId":"p1","productName":"Existing Product","description":"","quantity":1,"unitPrice":1000}],"newLineProductId":"p2","newLineProductName":"New Product","newLineDescription":"","newLineQty":1,"newLineUnitPrice":"20.00","newLineSearch":""}`)
	w := httptest.NewRecorder()

	h.addLineSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Existing Product") {
		t.Errorf("expected existing product preserved; body: %s", body)
	}
	if !strings.Contains(body, "New Product") {
		t.Errorf("expected new product added; body: %s", body)
	}
}

func TestAddLine_DefaultsQtyToOne(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("POST", "/api/invoices/lines",
		`{"lines":[],"newLineProductName":"Widget","newLineQty":0,"newLineUnitPrice":"5.00"}`)
	w := httptest.NewRecorder()

	h.addLineSSE(w, req)

	body := w.Body.String()
	// Qty 0 should default to 1
	if !strings.Contains(body, `"quantity":1`) {
		t.Errorf("expected quantity defaulted to 1; body: %s", body)
	}
}

// ===== Remove line =====

func TestRemoveLine_ValidIndex(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("DELETE", "/api/invoices/lines?idx=0",
		`{"lines":[{"productId":"p1","productName":"Product A","description":"","quantity":1,"unitPrice":1000},{"productId":"p2","productName":"Product B","description":"","quantity":1,"unitPrice":2000}]}`)
	w := httptest.NewRecorder()

	h.removeLineSSE(w, req)

	body := w.Body.String()
	if strings.Contains(body, "Product A") {
		t.Error("expected Product A removed from table")
	}
	if !strings.Contains(body, "Product B") {
		t.Errorf("expected Product B preserved; body: %s", body)
	}
}

func TestRemoveLine_LastLine(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("DELETE", "/api/invoices/lines?idx=0",
		`{"lines":[{"productId":"p1","productName":"Only Product","description":"","quantity":1,"unitPrice":1000}]}`)
	w := httptest.NewRecorder()

	h.removeLineSSE(w, req)

	body := w.Body.String()
	if strings.Contains(body, "Only Product") {
		t.Error("expected last product removed")
	}
	if !strings.Contains(body, "No line items yet") {
		t.Errorf("expected empty-state message after removing last line; body: %s", body)
	}
}

func TestRemoveLine_InvalidIndex(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("DELETE", "/api/invoices/lines?idx=99",
		`{"lines":[{"productId":"p1","productName":"Product A","description":"","quantity":1,"unitPrice":1000}]}`)
	w := httptest.NewRecorder()

	h.removeLineSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Product A") {
		t.Errorf("expected Product A preserved on out-of-range index; body: %s", body)
	}
}

func TestRemoveLine_RendersTableTarget(t *testing.T) {
	h := &Handlers{}
	req := sseBodyRequest("DELETE", "/api/invoices/lines?idx=0",
		`{"lines":[{"productId":"p1","productName":"Product A","description":"","quantity":1,"unitPrice":1000}]}`)
	w := httptest.NewRecorder()

	h.removeLineSSE(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "invoice-lines") {
		t.Errorf("expected invoice-lines element patch; body: %s", body)
	}
}
