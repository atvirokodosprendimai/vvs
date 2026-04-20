package http

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

const (
	cookieName        = "vvs_session"
	totpPendingCookie = "vvs_totp_pending"
	totpPendingMaxAge = 300 // 5 minutes
)

// totpUserStore is a local interface for saving TOTP state on a user.
type totpUserStore interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Save(ctx context.Context, u *domain.User) error
}

// loginRateLimiter tracks failed login attempts per IP address.
// Allows up to 10 failures per 15-minute window before locking out.
type loginRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginEntry
}

type loginEntry struct {
	failures int
	resetAt  time.Time
}

const (
	loginMaxFailures = 10
	loginWindow      = 15 * time.Minute
)

var globalLoginLimiter = &loginRateLimiter{entries: make(map[string]*loginEntry)}

// ── TOTP pending nonce store ──────────────────────────────────────────────────
// Stores a server-side mapping from random nonce → userID for the TOTP login step.
// The cookie holds the opaque nonce; the userID is never sent to the client.

type totpPendingNonce struct {
	userID    string
	expiresAt time.Time
}

var (
	totpPendingMu    sync.Mutex
	totpPendingStore = make(map[string]totpPendingNonce)
)

// newTOTPPending generates a random nonce, stores userID→nonce, returns the nonce.
func newTOTPPending(userID string) string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	nonce := hex.EncodeToString(buf)
	exp := time.Now().Add(totpPendingMaxAge * time.Second)
	totpPendingMu.Lock()
	defer totpPendingMu.Unlock()
	// prune stale entries
	for k, v := range totpPendingStore {
		if time.Now().After(v.expiresAt) {
			delete(totpPendingStore, k)
		}
	}
	totpPendingStore[nonce] = totpPendingNonce{userID: userID, expiresAt: exp}
	return nonce
}

// lookupTOTPPending returns the userID for a nonce without consuming it (for page render).
func lookupTOTPPending(nonce string) (string, bool) {
	totpPendingMu.Lock()
	defer totpPendingMu.Unlock()
	e, ok := totpPendingStore[nonce]
	if !ok || time.Now().After(e.expiresAt) {
		delete(totpPendingStore, nonce)
		return "", false
	}
	return e.userID, true
}

// consumeTOTPPending returns the userID and deletes the nonce (single-use).
func consumeTOTPPending(nonce string) (string, bool) {
	totpPendingMu.Lock()
	defer totpPendingMu.Unlock()
	e, ok := totpPendingStore[nonce]
	if !ok || time.Now().After(e.expiresAt) {
		delete(totpPendingStore, nonce)
		return "", false
	}
	delete(totpPendingStore, nonce)
	return e.userID, true
}

func (l *loginRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.entries[ip]
	if !ok || now.After(e.resetAt) {
		l.entries[ip] = &loginEntry{failures: 0, resetAt: now.Add(loginWindow)}
		return true
	}
	return e.failures < loginMaxFailures
}

func (l *loginRateLimiter) record(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.entries[ip]
	if !ok || now.After(e.resetAt) {
		l.entries[ip] = &loginEntry{failures: 1, resetAt: now.Add(loginWindow)}
		return
	}
	e.failures++
}

type Handlers struct {
	loginCmd              *commands.LoginHandler
	logoutCmd             *commands.LogoutHandler
	createUserCmd         *commands.CreateUserHandler
	deleteUserCmd         *commands.DeleteUserHandler
	changeSelfPasswordCmd *commands.ChangeSelfPasswordHandler
	updateUserCmd         *commands.UpdateUserHandler
	createSessionCmd      *commands.CreateSessionHandler
	createRoleCmd         *commands.CreateRoleHandler
	deleteRoleCmd         *commands.DeleteRoleHandler
	listUsersQuery        *queries.ListUsersHandler
	currentUser           *queries.GetCurrentUserHandler
	permRepo              domain.RolePermissionsRepository
	roleRepo              domain.RoleRepository
	totpUsers             totpUserStore
	maxAge                int
	secureCookie          bool
}

func (h *Handlers) WithPermRepo(r domain.RolePermissionsRepository) *Handlers {
	h.permRepo = r
	return h
}

func (h *Handlers) WithRoleHandlers(create *commands.CreateRoleHandler, delete *commands.DeleteRoleHandler, repo domain.RoleRepository) *Handlers {
	h.createRoleCmd = create
	h.deleteRoleCmd = delete
	h.roleRepo = repo
	return h
}

func (h *Handlers) WithMaxAge(secs int) *Handlers {
	h.maxAge = secs
	return h
}

func (h *Handlers) WithSecureCookie(secure bool) *Handlers {
	h.secureCookie = secure
	return h
}

func (h *Handlers) WithTOTPUsers(s totpUserStore) *Handlers {
	h.totpUsers = s
	return h
}

func (h *Handlers) WithCreateSession(cmd *commands.CreateSessionHandler) *Handlers {
	h.createSessionCmd = cmd
	return h
}

func NewHandlers(
	loginCmd *commands.LoginHandler,
	logoutCmd *commands.LogoutHandler,
	createUserCmd *commands.CreateUserHandler,
	deleteUserCmd *commands.DeleteUserHandler,
	changeSelfPasswordCmd *commands.ChangeSelfPasswordHandler,
	updateUserCmd *commands.UpdateUserHandler,
	listUsersQuery *queries.ListUsersHandler,
	currentUser *queries.GetCurrentUserHandler,
) *Handlers {
	return &Handlers{
		loginCmd:              loginCmd,
		logoutCmd:             logoutCmd,
		createUserCmd:         createUserCmd,
		deleteUserCmd:         deleteUserCmd,
		changeSelfPasswordCmd: changeSelfPasswordCmd,
		updateUserCmd:         updateUserCmd,
		listUsersQuery:        listUsersQuery,
		currentUser:           currentUser,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/login", h.loginPage)
	r.Post("/api/login", h.loginSSE)
	r.Post("/api/logout", h.logoutSSE)
	// TOTP login step (public — no session required)
	r.Get("/login/totp", h.loginTOTPPage)
	r.Post("/api/login/totp", h.loginTOTPSSE)
	r.Get("/users", h.usersPage)
	r.Get("/api/users", h.listUsersSSE)
	r.Post("/api/users", h.createUserSSE)
	r.Delete("/api/users/{id}", h.deleteUserSSE)
	r.Put("/api/users/{id}", h.updateUserSSE)
	r.Get("/profile", h.profilePage)
	r.Post("/api/users/me/password", h.changeSelfPasswordSSE)
	r.Put("/api/users/me/profile", h.updateSelfProfileSSE)
	// TOTP 2FA setup
	r.Get("/profile/2fa", h.profileTOTPSetupPage)
	r.Post("/api/users/me/totp/enable", h.enableTOTPSSE)
	r.Post("/api/users/me/totp/disable", h.disableTOTPSSE)
	r.Get("/settings/permissions", h.permissionsPage)
	r.Get("/sse/settings/permissions", h.permissionsSSE)
	r.Post("/api/permissions/{role}/{module}", h.savePermission)
	r.Get("/settings/roles", h.rolesPage)
	r.Post("/api/roles", h.createRoleSSE)
	r.Delete("/api/roles/{name}", h.deleteRoleSSE)
}

func (h *Handlers) loginPage(w http.ResponseWriter, r *http.Request) {
	LoginPage("").Render(r.Context(), w)
}

func (h *Handlers) loginSSE(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = xff
	}
	if !globalLoginLimiter.allow(ip) {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(loginError("Too many attempts. Try again in 15 minutes."))
		return
	}

	var signals struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("loginSSE: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	result, err := h.loginCmd.Handle(r.Context(), commands.LoginCommand{
		Username: signals.Username,
		Password: signals.Password,
	})
	if err != nil {
		globalLoginLimiter.record(ip)
		// Only create SSE on the error path so headers stay unlocked for cookie on success.
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(loginError("Invalid username or password"))
		return
	}

	// If TOTP is enabled, store the pending user ID in a short-lived cookie and
	// redirect to the TOTP step. The session created above is valid but the session
	// cookie is NOT set until the code is verified.
	if result.User.TOTPEnabled {
		// Revoke the eagerly-created session — we'll create a fresh one after TOTP.
		pendingSum := sha256.Sum256([]byte(result.Token))
		if err := h.logoutCmd.Handle(r.Context(), commands.LogoutCommand{TokenHash: hex.EncodeToString(pendingSum[:])}); err != nil {
			log.Printf("loginSSE: revoke pre-TOTP session: %v", err)
			// Non-fatal: session will expire naturally; continue to TOTP step.
		}
		// Store an opaque server-side nonce — never send the userID to the client.
		nonce := newTOTPPending(result.User.ID)
		http.SetCookie(w, &http.Cookie{
			Name:     totpPendingCookie,
			Value:    nonce,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   totpPendingMaxAge,
			Secure:   h.secureCookie,
		})
		sse := datastar.NewSSE(w, r)
		sse.Redirect("/login/totp")
		return
	}

	// Set cookie BEFORE NewSSE — NewSSE locks headers.
	maxAge := h.maxAge
	if maxAge <= 0 {
		maxAge = 86400
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Secure:   h.secureCookie,
	})
	sse := datastar.NewSSE(w, r)
	sse.Redirect("/")
}

func (h *Handlers) logoutSSE(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err == nil {
		sum := sha256.Sum256([]byte(cookie.Value))
		_ = h.logoutCmd.Handle(r.Context(), commands.LogoutCommand{
			TokenHash: hex.EncodeToString(sum[:]),
		})
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	sse := datastar.NewSSE(w, r)
	sse.Redirect("/login")
}

func (h *Handlers) usersPage(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil || !u.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var roles []domain.RoleDefinition
	if h.roleRepo != nil {
		if rs, err := h.roleRepo.List(r.Context()); err == nil {
			roles = rs
		}
	}
	if roles == nil {
		roles = []domain.RoleDefinition{
			{Name: domain.RoleAdmin, DisplayName: "Administrator"},
			{Name: domain.RoleOperator, DisplayName: "Operator"},
			{Name: domain.RoleViewer, DisplayName: "Viewer (read-only)"},
		}
	}
	UserListPage(roles).Render(r.Context(), w)
}

func (h *Handlers) listUsersSSE(w http.ResponseWriter, r *http.Request) {
	currentUser := userFromContext(r)
	if currentUser == nil || !currentUser.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	sse := datastar.NewSSE(w, r)
	currentID := currentUser.ID

	rows, err := h.listUsersQuery.Handle(r.Context())
	if err != nil {
		log.Printf("listUsersSSE: %v", err)
		return
	}
	sse.PatchElementTempl(UserTable(rows, currentID))
}

func (h *Handlers) createUserSSE(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil || !current.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var signals struct {
		NewUsername string `json:"newUsername"`
		NewPassword string `json:"newPassword"`
		NewRole     string `json:"newRole"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("createUserSSE: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	_, err := h.createUserCmd.Handle(r.Context(), commands.CreateUserCommand{
		Username: signals.NewUsername,
		Password: signals.NewPassword,
		Role:     domain.Role(signals.NewRole),
	})
	if err != nil {
		sse.PatchElementTempl(createUserError(err.Error()))
		return
	}

	rows, err := h.listUsersQuery.Handle(r.Context())
	if err != nil {
		log.Printf("createUserSSE: listUsersQuery: %v", err)
		return
	}
	currentID := ""
	if current != nil {
		currentID = current.ID
	}
	sse.PatchElementTempl(UserTable(rows, currentID))
}

func (h *Handlers) deleteUserSSE(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil || !current.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	if id == current.ID {
		http.Error(w, "cannot delete own account", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	if err := h.deleteUserCmd.Handle(r.Context(), commands.DeleteUserCommand{ID: id}); err != nil {
		log.Printf("deleteUserSSE: deleteUserCmd: %v", err)
		return
	}

	rows, err := h.listUsersQuery.Handle(r.Context())
	if err != nil {
		log.Printf("deleteUserSSE: listUsersQuery: %v", err)
		return
	}
	sse.PatchElementTempl(UserTable(rows, current.ID))
}

func (h *Handlers) profilePage(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	ProfilePage(u).Render(r.Context(), w)
}

func (h *Handlers) changeSelfPasswordSSE(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var signals struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("changeSelfPasswordSSE: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	err := h.changeSelfPasswordCmd.Handle(r.Context(), commands.ChangeSelfPasswordCommand{
		UserID:          current.ID,
		CurrentPassword: signals.CurrentPassword,
		NewPassword:     signals.NewPassword,
	})
	if err != nil {
		sse.PatchElementTempl(changePwError(err.Error()))
		return
	}
	sse.PatchElementTempl(changePwSuccess())
}

func (h *Handlers) permissionsPage(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil || !u.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	PermissionsPage().Render(r.Context(), w)
}

func (h *Handlers) permissionsSSE(w http.ResponseWriter, r *http.Request) {
	if h.permRepo == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}
	var roles []domain.RoleDefinition
	if h.roleRepo != nil {
		if rs, err := h.roleRepo.List(r.Context()); err == nil {
			roles = rs
		} else {
			log.Printf("permissionsSSE: list roles: %v", err)
		}
	}
	perms := make(map[domain.Role]domain.PermissionSet, len(roles))
	for _, rd := range roles {
		if rd.Name == domain.RoleAdmin {
			continue
		}
		ps, err := h.permRepo.FindByRole(r.Context(), rd.Name)
		if err != nil {
			log.Printf("permissionsSSE: %s: %v", rd.Name, err)
			ps = domain.DefaultPermissions(rd.Name)
		}
		perms[rd.Name] = ps
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(PermissionsGrid(roles, perms))
}

func (h *Handlers) rolesPage(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil || !u.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var roles []domain.RoleDefinition
	if h.roleRepo != nil {
		if rs, err := h.roleRepo.List(r.Context()); err == nil {
			roles = rs
		}
	}
	RolesPage(roles).Render(r.Context(), w)
}

func (h *Handlers) createRoleSSE(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil || !u.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var rawSignals map[string]interface{}
	if err := datastar.ReadSignals(r, &rawSignals); err != nil {
		log.Printf("createRoleSSE: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	roleName, _ := rawSignals["roleName"].(string)
	roleDisplay, _ := rawSignals["roleDisplay"].(string)
	roleCanWrite, _ := rawSignals["roleCanWrite"].(bool)

	perms := make(map[domain.Module]commands.ModulePermInput, len(domain.AllModules))
	for _, m := range domain.AllModules {
		base := moduleSignalBase(m)
		canView, _ := rawSignals["rolePerm"+base+"View"].(bool)
		canEdit, _ := rawSignals["rolePerm"+base+"Edit"].(bool)
		if canView || canEdit {
			perms[m] = commands.ModulePermInput{CanView: canView, CanEdit: canEdit}
		}
	}

	sse := datastar.NewSSE(w, r)
	if h.createRoleCmd == nil {
		sse.PatchElementTempl(addRoleError("role management not configured"))
		return
	}
	if _, err := h.createRoleCmd.Handle(r.Context(), commands.CreateRoleCommand{
		Name:        roleName,
		DisplayName: roleDisplay,
		CanWrite:    roleCanWrite,
		Permissions: perms,
	}); err != nil {
		sse.PatchElementTempl(addRoleError(err.Error()))
		return
	}
	roles, _ := h.roleRepo.List(r.Context())
	sse.PatchElementTempl(RoleRows(roles))
	sse.PatchSignals([]byte(`{"roleName":"","roleDisplay":""}`))
}

func (h *Handlers) deleteRoleSSE(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil || !u.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	name := domain.Role(chi.URLParam(r, "name"))
	if h.deleteRoleCmd == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}
	if err := h.deleteRoleCmd.Handle(r.Context(), commands.DeleteRoleCommand{Name: name}); err != nil {
		log.Printf("deleteRoleSSE: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	roles, _ := h.roleRepo.List(r.Context())
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(RoleRows(roles))
}

// validRoles is the set of configurable roles (admin is hardcoded, never in DB).
var validRoles = map[domain.Role]bool{domain.RoleOperator: true, domain.RoleViewer: true}

func (h *Handlers) savePermission(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil || !u.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if h.permRepo == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}

	role := domain.Role(chi.URLParam(r, "role"))
	module := domain.Module(chi.URLParam(r, "module"))
	field := r.URL.Query().Get("f")
	rawVal := r.URL.Query().Get("v")

	// Validate role
	if !validRoles[role] {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	// Validate module
	validModule := false
	for _, m := range domain.AllModules {
		if m == module {
			validModule = true
			break
		}
	}
	if !validModule {
		http.Error(w, "invalid module", http.StatusBadRequest)
		return
	}
	// Validate field
	if field != "view" && field != "edit" {
		http.Error(w, "invalid field", http.StatusBadRequest)
		return
	}
	// Validate value (strict — not "true"/"false" → 400)
	if rawVal != "true" && rawVal != "false" {
		http.Error(w, "invalid value", http.StatusBadRequest)
		return
	}
	val := rawVal == "true"

	// Load current row (or build default) to preserve the other field
	ps, err := h.permRepo.FindByRole(r.Context(), role)
	if err != nil || len(ps) == 0 {
		ps = domain.DefaultPermissions(role)
	}
	perm, ok := ps[module]
	if !ok {
		perm = &domain.RoleModulePermission{Role: role, Module: module}
	}

	switch field {
	case "view":
		perm.CanView = val
	case "edit":
		perm.CanEdit = val
	}

	if err := h.permRepo.Save(r.Context(), perm); err != nil {
		log.Printf("savePermission: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) updateUserSSE(w http.ResponseWriter, r *http.Request) {
	actor := userFromContext(r)
	if actor == nil || !actor.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	var signals struct {
		EditFullName string `json:"editFullName"`
		EditDivision string `json:"editDivision"`
		EditRole     string `json:"editRole"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("updateUserSSE: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	err := h.updateUserCmd.Handle(r.Context(), commands.UpdateUserCommand{
		ActorID:  actor.ID,
		UserID:   id,
		FullName: signals.EditFullName,
		Division: signals.EditDivision,
		Role:     domain.Role(signals.EditRole),
	})
	if err != nil {
		sse.PatchElementTempl(editUserError(err.Error()))
		return
	}

	rows, err := h.listUsersQuery.Handle(r.Context())
	if err != nil {
		log.Printf("updateUserSSE: listUsersQuery: %v", err)
		return
	}
	sse.PatchElementTempl(UserTable(rows, actor.ID))
}

func (h *Handlers) updateSelfProfileSSE(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var signals struct {
		ProfileFullName string `json:"profileFullName"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("updateSelfProfileSSE: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	// Self-service: full name only, division stays unchanged
	err := h.updateUserCmd.Handle(r.Context(), commands.UpdateUserCommand{
		ActorID:  current.ID,
		UserID:   current.ID,
		FullName: signals.ProfileFullName,
		Division: current.Division,
		Role:     current.Role,
	})
	if err != nil {
		sse.PatchElementTempl(changePwError(err.Error()))
		return
	}
	sse.PatchElementTempl(profileSaveSuccess())
}

// Note: auth Handlers intentionally does NOT implement ModuleNamed.
// Login/logout/profile/change-password must never be wrapped in RequireModuleAccess.
// User management routes are guarded by explicit IsAdmin() checks in each handler.
