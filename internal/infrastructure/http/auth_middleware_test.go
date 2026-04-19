package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	authdomain "github.com/vvs/isp/internal/modules/auth/domain"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
)

func makeTestUser(t *testing.T, role authdomain.Role) *authdomain.User {
	t.Helper()
	u, err := authdomain.NewUser("testuser", "password123", role)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	return u
}

func withUserMiddleware(u *authdomain.User, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := authhttp.WithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireWrite_ViewerBlocked_OnMutatingMethods(t *testing.T) {
	viewer := makeTestUser(t, authdomain.RoleViewer)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			handler := withUserMiddleware(viewer, infrahttp.RequireWrite(okHandler()))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/api/something", nil)
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code, "viewer should be blocked on %s", method)
		})
	}
}

func TestRequireWrite_OperatorAllowed_OnMutatingMethods(t *testing.T) {
	op := makeTestUser(t, authdomain.RoleOperator)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			handler := withUserMiddleware(op, infrahttp.RequireWrite(okHandler()))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/api/something", nil)
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "operator should pass on %s", method)
		})
	}
}

func TestRequireWrite_AdminAllowed_OnMutatingMethods(t *testing.T) {
	admin := makeTestUser(t, authdomain.RoleAdmin)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			handler := withUserMiddleware(admin, infrahttp.RequireWrite(okHandler()))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/api/something", nil)
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "admin should pass on %s", method)
		})
	}
}

func TestRequireWrite_ViewerAllowed_OnGetRequests(t *testing.T) {
	viewer := makeTestUser(t, authdomain.RoleViewer)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			handler := withUserMiddleware(viewer, infrahttp.RequireWrite(okHandler()))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/api/something", nil)
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "viewer should pass on %s", method)
		})
	}
}

func TestRequireWrite_NoUser_PassesThrough(t *testing.T) {
	// No user in context — RequireAuth is responsible for auth; RequireWrite should not double-gate
	handler := infrahttp.RequireWrite(okHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/something", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "no user in context should pass through RequireWrite")
}

func TestRequireWrite_ViewerAllowed_OnSelfServicePasswordChange(t *testing.T) {
	// Viewer-role users must be able to change their own password even though it's a POST.
	viewer := makeTestUser(t, authdomain.RoleViewer)
	handler := withUserMiddleware(viewer, infrahttp.RequireWrite(okHandler()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/users/me/password", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "viewer must be allowed to POST to self-service password route")
}

// ── RequireModuleAccess tests ──────────────────────────────────────────────

func withPermissions(ps authdomain.PermissionSet, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := infrahttp.WithPermissions(r.Context(), ps)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestRequireModuleAccess_ViewDenied_Returns403(t *testing.T) {
	ps := authdomain.PermissionSet{} // no modules allowed
	handler := withPermissions(ps, infrahttp.RequireModuleAccess(authdomain.ModuleInvoices)(okHandler()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/invoices", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireModuleAccess_ViewAllowed_EditDenied_OnMutation(t *testing.T) {
	ps := authdomain.PermissionSet{
		authdomain.ModuleInvoices: {Module: authdomain.ModuleInvoices, CanView: true, CanEdit: false},
	}
	handler := withPermissions(ps, infrahttp.RequireModuleAccess(authdomain.ModuleInvoices)(okHandler()))

	// GET passes
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/invoices", nil)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// POST blocked
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/invoices", nil)
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusForbidden, rec2.Code)
}

func TestRequireModuleAccess_FullAccess_AllMethodsPass(t *testing.T) {
	ps := authdomain.PermissionSet{
		authdomain.ModuleInvoices: {Module: authdomain.ModuleInvoices, CanView: true, CanEdit: true},
	}
	handler := withPermissions(ps, infrahttp.RequireModuleAccess(authdomain.ModuleInvoices)(okHandler()))
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/invoices", nil)
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "method %s should pass with full access", method)
	}
}

func TestRequireModuleAccess_AdminPermissionSet_AlwaysPasses(t *testing.T) {
	ps := authdomain.AdminPermissionSet()
	handler := withPermissions(ps, infrahttp.RequireModuleAccess(authdomain.ModuleNetwork)(okHandler()))
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/network", nil)
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "admin should pass on %s", method)
	}
}
