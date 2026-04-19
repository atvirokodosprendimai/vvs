package http

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/auth/app/commands"
	"github.com/vvs/isp/internal/modules/auth/app/queries"
	"github.com/vvs/isp/internal/modules/auth/domain"
)

const cookieName = "vvs_session"

type Handlers struct {
	loginCmd              *commands.LoginHandler
	logoutCmd             *commands.LogoutHandler
	createUserCmd         *commands.CreateUserHandler
	deleteUserCmd         *commands.DeleteUserHandler
	changeSelfPasswordCmd *commands.ChangeSelfPasswordHandler
	listUsersQuery        *queries.ListUsersHandler
	currentUser           *queries.GetCurrentUserHandler
	permRepo              domain.RolePermissionsRepository
}

func (h *Handlers) WithPermRepo(r domain.RolePermissionsRepository) *Handlers {
	h.permRepo = r
	return h
}

func NewHandlers(
	loginCmd *commands.LoginHandler,
	logoutCmd *commands.LogoutHandler,
	createUserCmd *commands.CreateUserHandler,
	deleteUserCmd *commands.DeleteUserHandler,
	changeSelfPasswordCmd *commands.ChangeSelfPasswordHandler,
	listUsersQuery *queries.ListUsersHandler,
	currentUser *queries.GetCurrentUserHandler,
) *Handlers {
	return &Handlers{
		loginCmd:             loginCmd,
		logoutCmd:            logoutCmd,
		createUserCmd:        createUserCmd,
		deleteUserCmd:        deleteUserCmd,
		changeSelfPasswordCmd: changeSelfPasswordCmd,
		listUsersQuery:       listUsersQuery,
		currentUser:          currentUser,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/login", h.loginPage)
	r.Post("/api/login", h.loginSSE)
	r.Post("/api/logout", h.logoutSSE)
	r.Get("/users", h.usersPage)
	r.Get("/api/users", h.listUsersSSE)
	r.Post("/api/users", h.createUserSSE)
	r.Delete("/api/users/{id}", h.deleteUserSSE)
	r.Get("/profile", h.profilePage)
	r.Post("/api/users/me/password", h.changeSelfPasswordSSE)
	r.Get("/settings/permissions", h.permissionsPage)
	r.Get("/sse/settings/permissions", h.permissionsSSE)
	r.Post("/api/permissions/{role}/{module}", h.savePermission)
}

func (h *Handlers) loginPage(w http.ResponseWriter, r *http.Request) {
	LoginPage("").Render(r.Context(), w)
}

func (h *Handlers) loginSSE(w http.ResponseWriter, r *http.Request) {
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
		// Only create SSE on the error path so headers stay unlocked for cookie on success.
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(loginError("Invalid username or password"))
		return
	}

	// Set cookie BEFORE NewSSE — NewSSE locks headers.
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
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
	UserListPage().Render(r.Context(), w)
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
	opPerms, err := h.permRepo.FindByRole(r.Context(), domain.RoleOperator)
	if err != nil {
		log.Printf("permissionsSSE: operator: %v", err)
		opPerms = domain.DefaultPermissions(domain.RoleOperator)
	}
	viewerPerms, err := h.permRepo.FindByRole(r.Context(), domain.RoleViewer)
	if err != nil {
		log.Printf("permissionsSSE: viewer: %v", err)
		viewerPerms = domain.DefaultPermissions(domain.RoleViewer)
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(PermissionsGrid(opPerms, viewerPerms))
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

// Note: auth Handlers intentionally does NOT implement ModuleNamed.
// Login/logout/profile/change-password must never be wrapped in RequireModuleAccess.
// User management routes are guarded by explicit IsAdmin() checks in each handler.
