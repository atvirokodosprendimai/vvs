package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	authdomain "github.com/vvs/isp/internal/modules/auth/domain"
	portalhttp "github.com/vvs/isp/internal/modules/portal/adapters/http"
	"github.com/vvs/isp/internal/modules/portal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stub token repo ───────────────────────────────────────────────────────────

type stubTokenRepo struct {
	tokens map[string]*domain.PortalToken // keyed by hash
}

func newStubTokenRepo() *stubTokenRepo {
	return &stubTokenRepo{tokens: make(map[string]*domain.PortalToken)}
}

func (r *stubTokenRepo) Save(_ context.Context, t *domain.PortalToken) error {
	r.tokens[t.TokenHash] = t
	return nil
}

func (r *stubTokenRepo) FindByHash(_ context.Context, hash string) (*domain.PortalToken, error) {
	t, ok := r.tokens[hash]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (r *stubTokenRepo) DeleteByCustomerID(_ context.Context, _ string) error { return nil }
func (r *stubTokenRepo) PruneExpired(_ context.Context) error                 { return nil }

// seedToken saves a valid token in the repo and returns the plaintext.
func seedToken(t *testing.T, repo *stubTokenRepo, customerID string, ttl time.Duration) string {
	t.Helper()
	tok, plain, err := domain.NewPortalToken(customerID, ttl)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), tok))
	return plain
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newPortalRouter(repo *stubTokenRepo) http.Handler {
	h := portalhttp.NewHandlers(repo, nil, nil) // invoice queries nil — not tested here
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	h.RegisterPublicRoutes(r)
	return r
}

func adminUser(t *testing.T) *authdomain.User {
	t.Helper()
	u, err := authdomain.NewUser("admin", "Password1!", authdomain.RoleAdmin)
	require.NoError(t, err)
	return u
}

// ── tests: GET /portal/auth ───────────────────────────────────────────────────

func TestPortalAuth_ValidToken_SetsCookieAndRedirects(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)
	plain := seedToken(t, repo, "cust-1", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/portal/auth?token="+plain, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, "/portal/invoices", rr.Header().Get("Location"))

	cookies := rr.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "vvs_portal" {
			found = true
			assert.Equal(t, plain, c.Value)
		}
	}
	assert.True(t, found, "vvs_portal cookie must be set")
}

func TestPortalAuth_InvalidToken_RendersExpiredPage(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/portal/auth?token=notvalid", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Access link expired")
}

func TestPortalAuth_NoToken_RendersExpiredPage(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/portal/auth", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Access link expired")
}

func TestPortalAuth_ExpiredToken_RendersExpiredPage(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)
	plain := seedToken(t, repo, "cust-1", time.Millisecond)
	time.Sleep(5 * time.Millisecond) // let it expire

	req := httptest.NewRequest(http.MethodGet, "/portal/auth?token="+plain, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Access link expired")
}

// ── tests: POST /portal/logout ────────────────────────────────────────────────

func TestPortalLogout_ClearsCookieAndRedirects(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/portal/logout", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "/portal/auth")

	cookies := rr.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "vvs_portal" {
			assert.Equal(t, -1, c.MaxAge, "cookie must be expired")
		}
	}
}

// ── tests: requirePortalAuth middleware ───────────────────────────────────────

func TestPortalInvoiceList_NoCookie_Redirects(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/portal/invoices", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "expired=1")
}

func TestPortalInvoiceList_InvalidCookie_Redirects(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/portal/invoices", nil)
	req.AddCookie(&http.Cookie{Name: "vvs_portal", Value: "bogus-token"})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Contains(t, rr.Header().Get("Location"), "expired=1")
}

// ── tests: POST /api/customers/{id}/portal-link ───────────────────────────────

func TestGeneratePortalLink_AdminUser_ReturnsSSEWithLink(t *testing.T) {
	repo := newStubTokenRepo()
	h := portalhttp.NewHandlers(repo, nil, nil).
		WithBaseURL("http://example.com")
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	h.RegisterPublicRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/customers/cust-99/portal-link", nil)
	req = req.WithContext(authhttp.WithUser(req.Context(), adminUser(t)))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// SSE stream — check that portal URL appears in response body
	body := rr.Body.String()
	assert.Contains(t, body, "http://example.com/portal/auth?token=")
	assert.Contains(t, body, "portal-link-result")
}

func TestGeneratePortalLink_NonAdmin_Forbidden(t *testing.T) {
	repo := newStubTokenRepo()
	h := portalhttp.NewHandlers(repo, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	h.RegisterPublicRoutes(r)

	viewer, err := authdomain.NewUser("viewer", "Password1!", authdomain.RoleViewer)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/customers/cust-99/portal-link", nil)
	req = req.WithContext(authhttp.WithUser(req.Context(), viewer))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestGeneratePortalLink_NoUser_Forbidden(t *testing.T) {
	repo := newStubTokenRepo()
	h := portalhttp.NewHandlers(repo, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	h.RegisterPublicRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/customers/cust-99/portal-link", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}
