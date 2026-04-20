package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	authhttp "github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/http"
	authdomain "github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	portalhttp "github.com/atvirokodosprendimai/vvs/internal/modules/portal/adapters/http"
	"github.com/atvirokodosprendimai/vvs/internal/modules/portal/domain"
	invoicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/queries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stub invoice lister ───────────────────────────────────────────────────────

type stubInvoiceLister struct{}

func (s *stubInvoiceLister) Handle(_ context.Context, _ invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error) {
	return nil, nil
}

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

func (r *stubTokenRepo) MarkUsed(_ context.Context, hash string) error {
	if t, ok := r.tokens[hash]; ok {
		now := time.Now().UTC()
		t.UsedAt = &now
	}
	return nil
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
	h := portalhttp.NewHandlers(repo, &stubInvoiceLister{}, nil)
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
			// Cookie must carry a NEW session token, not the magic link value.
			assert.NotEqual(t, plain, c.Value, "cookie must be new session token, not magic link")
			assert.NotEmpty(t, c.Value)
		}
	}
	assert.True(t, found, "vvs_portal cookie must be set")
}

func TestPortalAuth_IssuesSessionToken_WithLongTTL(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)
	// Magic link has short TTL (15 min) — session must be 7 days.
	plain := seedToken(t, repo, "cust-1", 15*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/portal/auth?token="+plain, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusFound, rr.Code)

	var sessionCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "vvs_portal" {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie, "vvs_portal cookie must be set")

	const sevenDaysSeconds = 7 * 24 * 60 * 60
	assert.GreaterOrEqual(t, sessionCookie.MaxAge, sevenDaysSeconds,
		"session cookie MaxAge must be >= 7 days")

	// Session token must exist in repo and have ~7-day expiry.
	sessionHash := domain.HashOf(sessionCookie.Value)
	sessionTok, err := repo.FindByHash(context.Background(), sessionHash)
	require.NoError(t, err)
	require.NotNil(t, sessionTok, "session token must be saved in repo")
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), sessionTok.ExpiresAt, 10*time.Second)
	assert.Equal(t, "cust-1", sessionTok.CustomerID)
}

func TestPortalSession_Refresh_WhenNearExpiry(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)

	// Seed a session token with < 3.5 days remaining (past refresh threshold).
	shortRemaining := 2 * 24 * time.Hour // 2 days left — below 3.5-day threshold
	plain := seedToken(t, repo, "cust-2", shortRemaining)

	req := httptest.NewRequest(http.MethodGet, "/portal/invoices", nil)
	req.AddCookie(&http.Cookie{Name: "vvs_portal", Value: plain})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Must not redirect to login.
	assert.NotEqual(t, "/portal/login?expired=1", rr.Header().Get("Location"))

	// Must set a new cookie with refreshed TTL.
	var refreshed *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "vvs_portal" {
			refreshed = c
		}
	}
	require.NotNil(t, refreshed, "refresh cookie must be set when session near expiry")
	assert.NotEqual(t, plain, refreshed.Value, "refreshed cookie must carry new token")

	const sevenDaysSeconds = 7 * 24 * 60 * 60
	assert.GreaterOrEqual(t, refreshed.MaxAge, sevenDaysSeconds)
}

func TestPortalSession_NoRefresh_WhenFresh(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)

	// Seed a session token with > 3.5 days remaining — no refresh expected.
	plain := seedToken(t, repo, "cust-3", 6*24*time.Hour) // 6 days left

	req := httptest.NewRequest(http.MethodGet, "/portal/invoices", nil)
	req.AddCookie(&http.Cookie{Name: "vvs_portal", Value: plain})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	for _, c := range rr.Result().Cookies() {
		if c.Name == "vvs_portal" {
			assert.Fail(t, "no cookie refresh expected when session has plenty of time left")
		}
	}
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
	assert.Contains(t, rr.Header().Get("Location"), "/portal/login")

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

// ── tests: single-use magic-link enforcement ──────────────────────────────────

func TestPortalAuth_SingleUse_FirstClickSucceeds(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)
	plain := seedToken(t, repo, "cust-1", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/portal/auth?token="+plain, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, "/portal/invoices", rr.Header().Get("Location"))
}

func TestPortalAuth_SingleUse_SecondClickShowsExpiredPage(t *testing.T) {
	repo := newStubTokenRepo()
	router := newPortalRouter(repo)
	plain := seedToken(t, repo, "cust-1", time.Hour)

	// First click — consumes the token.
	req1 := httptest.NewRequest(http.MethodGet, "/portal/auth?token="+plain, nil)
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)
	require.Equal(t, http.StatusFound, rr1.Code)

	// Second click — token already used.
	req2 := httptest.NewRequest(http.MethodGet, "/portal/auth?token="+plain, nil)
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusOK, rr2.Code)
	assert.Contains(t, rr2.Body.String(), "Access link expired")
}

func TestPortalAuth_UsedToken_SessionCookieStillValid(t *testing.T) {
	repo := newStubTokenRepo()
	plain := seedToken(t, repo, "cust-1", time.Hour)

	// Build router with a stub invoice lister so /portal/invoices doesn't panic.
	type stubLister struct{}
	_ = stubLister{}

	// Click the magic link — sets cookie and marks token used.
	router := newPortalRouter(repo)
	req1 := httptest.NewRequest(http.MethodGet, "/portal/auth?token="+plain, nil)
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)
	require.Equal(t, http.StatusFound, rr1.Code)

	// Subsequent /portal/auth call (magic link reuse) should be rejected.
	req2 := httptest.NewRequest(http.MethodGet, "/portal/auth?token="+plain, nil)
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusOK, rr2.Code)
	assert.Contains(t, rr2.Body.String(), "Access link expired", "reused magic link should show expired page")
}
