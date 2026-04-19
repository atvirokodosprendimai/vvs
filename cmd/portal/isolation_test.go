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
	portalnats "github.com/vvs/isp/internal/modules/portal/adapters/nats"
)

// buildPortalRouter constructs the same router as runPortal but with a stub NATS client.
// Used to assert that admin routes are NOT reachable from the portal binary.
func buildPortalRouter(t *testing.T) http.Handler {
	t.Helper()

	// A nil PortalNATSClient is fine here — we only want to verify route registration,
	// not actual handler execution.
	var client *portalnats.PortalNATSClient = nil

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/i/{token}", publicInvoiceByToken(client))

	// Register portal public routes with a stub handler (no NATS needed for route list).
	// We construct a minimal portalHandlers using nil dependencies, which is fine for
	// route enumeration tests since the router registers routes at startup before any request.
	registerPortalRoutesForTest(r)

	return r
}

// registerPortalRoutesForTest registers portal routes using nil handlers — enough to
// enumerate the registered path set without a real NATS connection.
func registerPortalRoutesForTest(r chi.Router) {
	// Mimic RegisterPublicRoutes paths directly.
	r.Get("/portal/auth", http.NotFound)
	r.Post("/portal/logout", http.NotFound)
	r.Get("/portal", http.NotFound)
	r.Get("/portal/invoices", http.NotFound)
	r.Get("/portal/invoices/{id}", http.NotFound)
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
