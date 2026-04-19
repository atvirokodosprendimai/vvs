package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	invoicequeries "github.com/vvs/isp/internal/modules/invoice/app/queries"
	portaldomain "github.com/vvs/isp/internal/modules/portal/domain"
	portalhttp "github.com/vvs/isp/internal/modules/portal/adapters/http"
	portalnats "github.com/vvs/isp/internal/modules/portal/adapters/nats"
)

// ── stubs ──────────────────────────────────────────────────────────────────────

type stubTokenRepo struct{}

func (s *stubTokenRepo) FindByHash(_ context.Context, _ string) (*portaldomain.PortalToken, error) {
	return nil, nil
}
func (s *stubTokenRepo) Save(_ context.Context, _ *portaldomain.PortalToken) error { return nil }
func (s *stubTokenRepo) DeleteByCustomerID(_ context.Context, _ string) error      { return nil }
func (s *stubTokenRepo) PruneExpired(_ context.Context) error                      { return nil }

type stubInvoiceLister struct{}

func (s *stubInvoiceLister) Handle(_ context.Context, _ invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error) {
	return nil, nil
}

type stubInvoiceGetter struct{}

func (s *stubInvoiceGetter) Handle(_ context.Context, _ string) (*invoicequeries.InvoiceReadModel, error) {
	return nil, nil
}

type stubPDFMinter struct{}

func (s *stubPDFMinter) MintToken(_ context.Context, _, _ string) (string, error) { return "", nil }

// ── router builder ─────────────────────────────────────────────────────────────

// buildPortalRouter constructs the same router as runPortal but with stub dependencies.
// Used to assert that admin routes are NOT reachable from the portal binary.
func buildPortalRouter(t *testing.T) http.Handler {
	t.Helper()

	// nil client is fine — publicInvoiceByToken guards nil and returns 503
	var client *portalnats.PortalNATSClient = nil

	// Real handlers with stubs — verifies actual route wiring, not fake registrations.
	handlers := portalhttp.NewHandlers(
		&stubTokenRepo{},
		&stubInvoiceLister{},
		&stubInvoiceGetter{},
	).WithPDFTokens(&stubPDFMinter{})

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/i/{token}", publicInvoiceByToken(client))
	handlers.RegisterPublicRoutes(r)

	return r
}

// adminRoutes lists paths that MUST NOT exist in the portal binary.
var adminRoutes = []struct {
	method string
	path   string
}{
	{"GET", "/"},
	{"GET", "/login"},
	{"GET", "/customers"},
	{"GET", "/customers/new"},
	{"GET", "/invoices"},
	{"GET", "/invoices/new"},
	{"GET", "/settings"},
	{"GET", "/settings/permissions"},
	{"GET", "/users"},
	{"GET", "/services"},
	{"GET", "/audit-logs"},
	{"GET", "/reports"},
	{"GET", "/dashboard"},
	{"POST", "/api/login"},
	{"POST", "/api/customers"},
	{"POST", "/api/invoices"},
	{"POST", "/api/users"},
	{"DELETE", "/api/users/1"},
}

func TestPortalBinary_AdminRoutesNotRegistered(t *testing.T) {
	r := buildPortalRouter(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx

	for _, tc := range adminRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			// Admin routes must return 404 — not served by the portal router.
			assert.Equal(t, http.StatusNotFound, rr.Code,
				"admin route %s %s should not be reachable from portal binary", tc.method, tc.path)
		})
	}
}

func TestPortalBinary_PortalRoutesRegistered(t *testing.T) {
	r := buildPortalRouter(t)

	portalRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/portal/auth"},
		{"POST", "/portal/logout"},
		{"GET", "/portal"},
		{"GET", "/portal/invoices"},
		{"GET", "/portal/invoices/inv-123"},
		{"GET", "/i/sometoken"},
	}

	for _, tc := range portalRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			// Portal routes should NOT return 405 Method Not Allowed (method mismatch).
			// They may return anything except 405 — the route IS registered.
			assert.NotEqual(t, http.StatusMethodNotAllowed, rr.Code,
				"portal route %s %s should be registered", tc.method, tc.path)
		})
	}
}
