package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authhttp "github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/http"
	authdomain "github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	invoicemigrations "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/migrations"
	infrahttp "github.com/atvirokodosprendimai/vvs/internal/infrastructure/http"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

// seedPaidInvoice inserts a minimal paid invoice row directly via SQL.
func seedPaidInvoice(t *testing.T, db *gormsqlite.DB, customerID, customerName string, totalCents int64, paidAt time.Time) {
	t.Helper()
	err := db.W.Exec(`
		INSERT INTO invoices
			(id, code, customer_id, customer_name, customer_code, status, issue_date, due_date,
			 sub_total, vat_total, total_amount, paid_at, created_at, updated_at)
		VALUES
			(?, ?, ?, ?, 'CLI-00001', 'paid', date('now'), date('now'),
			 ?, 0, ?, ?, datetime('now'), datetime('now'))
	`, "inv-"+customerID, "INV-0001", customerID, customerName, totalCents, totalCents, paidAt).Error
	require.NoError(t, err)
}

func newReportsRouter(t *testing.T) (http.Handler, *gormsqlite.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, invoicemigrations.FS, "goose_invoice")

	admin, err := authdomain.NewUser("admin", "Password1!", authdomain.RoleAdmin)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := authhttp.WithUser(r.Context(), admin)
			ctx = authdomain.WithPermissions(ctx, authdomain.AdminPermissionSet())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Get("/reports", func(w http.ResponseWriter, r *http.Request) {
		infrahttp.ReportsPage().Render(r.Context(), w)
	})
	r.Get("/api/reports/data", infrahttp.NewReportsHandler(db.R))
	return r, db
}

func TestReportsPage_EmptyDB_Renders200(t *testing.T) {
	router, _ := newReportsRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Reports")
}

func TestReportsData_EmptyDB_NoError(t *testing.T) {
	router, _ := newReportsRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/reports/data", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	body := rr.Body.String()
	assert.Contains(t, body, "reports-content")
}

func TestReportsData_WithPaidInvoice_ShowsCustomer(t *testing.T) {
	router, db := newReportsRouter(t)
	seedPaidInvoice(t, db, "cust-99", "Acme Corp", 150000, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/api/reports/data", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Contains(t, rr.Body.String(), "Acme Corp")
}
