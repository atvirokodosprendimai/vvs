package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/auth/app/commands"
	authdomain "github.com/vvs/isp/internal/modules/auth/domain"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubSessionRepo struct {
	sessions map[string]*authdomain.Session
}

func newStubSessionRepo() *stubSessionRepo {
	return &stubSessionRepo{sessions: make(map[string]*authdomain.Session)}
}
func (s *stubSessionRepo) Save(_ context.Context, sess *authdomain.Session) error {
	s.sessions[sess.TokenHash] = sess
	return nil
}
func (s *stubSessionRepo) FindByTokenHash(_ context.Context, hash string) (*authdomain.Session, error) {
	return s.sessions[hash], nil
}
func (s *stubSessionRepo) DeleteByID(_ context.Context, id string) error {
	for k, v := range s.sessions {
		if v.ID == id {
			delete(s.sessions, k)
		}
	}
	return nil
}
func (s *stubSessionRepo) DeleteByUserID(_ context.Context, _ string) error { return nil }
func (s *stubSessionRepo) PruneExpired(_ context.Context) error             { return nil }

type stubUserRepo struct {
	users map[string]*authdomain.User
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{users: make(map[string]*authdomain.User)}
}
func (s *stubUserRepo) Save(_ context.Context, u *authdomain.User) error {
	s.users[u.ID] = u
	return nil
}
func (s *stubUserRepo) FindByID(_ context.Context, id string) (*authdomain.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, authdomain.ErrUserNotFound
	}
	return u, nil
}
func (s *stubUserRepo) FindByUsername(_ context.Context, username string) (*authdomain.User, error) {
	for _, u := range s.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, authdomain.ErrUserNotFound
}
func (s *stubUserRepo) ListAll(_ context.Context) ([]*authdomain.User, error) { return nil, nil }
func (s *stubUserRepo) Delete(_ context.Context, _ string) error              { return nil }

// ── test helper ───────────────────────────────────────────────────────────────

func buildTOTPHandlers(t *testing.T, userRepo *stubUserRepo, sessRepo *stubSessionRepo) (http.Handler, *authhttp.Handlers) {
	t.Helper()
	loginCmd := commands.NewLoginHandler(userRepo, sessRepo)
	logoutCmd := commands.NewLogoutHandler(sessRepo)
	createSessionCmd := commands.NewCreateSessionHandler(sessRepo)

	h := authhttp.NewHandlers(loginCmd, logoutCmd, nil, nil, nil, nil, nil, nil).
		WithTOTPUsers(userRepo).
		WithCreateSession(createSessionCmd)

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, h
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestTOTPLoginPage_NoPendingCookie_RedirectsToLogin(t *testing.T) {
	router, _ := buildTOTPHandlers(t, newStubUserRepo(), newStubSessionRepo())

	req := httptest.NewRequest(http.MethodGet, "/login/totp", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, "/login", rr.Header().Get("Location"))
}

func TestTOTPLoginPage_WithPendingCookie_RendersForm(t *testing.T) {
	router, _ := buildTOTPHandlers(t, newStubUserRepo(), newStubSessionRepo())

	// newTOTPPending registers a server-side nonce — the cookie must hold this nonce.
	nonce := authhttp.NewTOTPPendingForTest("some-user-id")

	req := httptest.NewRequest(http.MethodGet, "/login/totp", nil)
	req.AddCookie(&http.Cookie{Name: "vvs_totp_pending", Value: nonce})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Authenticator Code")
}

func TestTOTPLoginSSE_NoPendingCookie_RedirectsToLogin(t *testing.T) {
	router, _ := buildTOTPHandlers(t, newStubUserRepo(), newStubSessionRepo())

	req := httptest.NewRequest(http.MethodPost, "/api/login/totp", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusFound, rr.Code)
	assert.Equal(t, "/login", rr.Header().Get("Location"))
}

func TestProfileTOTPSetupPage_RendersQR(t *testing.T) {
	userRepo := newStubUserRepo()
	sessRepo := newStubSessionRepo()

	u, err := authdomain.NewUser("alice", "Password1!", authdomain.RoleAdmin)
	require.NoError(t, err)
	require.NoError(t, userRepo.Save(context.Background(), u))

	_, h := buildTOTPHandlers(t, userRepo, sessRepo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/profile/2fa", nil)
	req = req.WithContext(authhttp.WithUser(req.Context(), u))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Two-Factor Auth")
	assert.Contains(t, rr.Body.String(), "data:image/png;base64,")
}

func TestProfileTOTPSetupPage_AlreadyEnabled_ShowsDisableButton(t *testing.T) {
	userRepo := newStubUserRepo()
	sessRepo := newStubSessionRepo()

	u, err := authdomain.NewUser("alice", "Password1!", authdomain.RoleAdmin)
	require.NoError(t, err)
	u.EnableTOTP("JBSWY3DPEHPK3PXP")
	require.NoError(t, userRepo.Save(context.Background(), u))

	_, h := buildTOTPHandlers(t, userRepo, sessRepo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/profile/2fa", nil)
	req = req.WithContext(authhttp.WithUser(req.Context(), u))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Disable Two-Factor Auth")
}

func TestDisableTOTPSSE_ClearsTOTPAndRedirects(t *testing.T) {
	userRepo := newStubUserRepo()
	sessRepo := newStubSessionRepo()

	u, err := authdomain.NewUser("alice", "Password1!", authdomain.RoleAdmin)
	require.NoError(t, err)
	u.EnableTOTP("JBSWY3DPEHPK3PXP")
	require.NoError(t, userRepo.Save(context.Background(), u))

	_, h := buildTOTPHandlers(t, userRepo, sessRepo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/users/me/totp/disable", nil)
	req = req.WithContext(authhttp.WithUser(req.Context(), u))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// SSE redirect to /profile
	assert.Contains(t, rr.Body.String(), "/profile")

	// User TOTP should be cleared in store
	saved, err := userRepo.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.False(t, saved.TOTPEnabled)
	assert.Empty(t, saved.TOTPSecret)
}
