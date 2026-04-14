package http

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/auth/app/commands"
	"github.com/vvs/isp/internal/modules/auth/app/queries"
	"github.com/vvs/isp/internal/modules/auth/domain"
)

const cookieName = "vvs_session"

type Handlers struct {
	loginCmd       *commands.LoginHandler
	logoutCmd      *commands.LogoutHandler
	createUserCmd  *commands.CreateUserHandler
	deleteUserCmd  *commands.DeleteUserHandler
	listUsersQuery *queries.ListUsersHandler
	currentUser    *queries.GetCurrentUserHandler
}

func NewHandlers(
	loginCmd *commands.LoginHandler,
	logoutCmd *commands.LogoutHandler,
	createUserCmd *commands.CreateUserHandler,
	deleteUserCmd *commands.DeleteUserHandler,
	listUsersQuery *queries.ListUsersHandler,
	currentUser *queries.GetCurrentUserHandler,
) *Handlers {
	return &Handlers{
		loginCmd:       loginCmd,
		logoutCmd:      logoutCmd,
		createUserCmd:  createUserCmd,
		deleteUserCmd:  deleteUserCmd,
		listUsersQuery: listUsersQuery,
		currentUser:    currentUser,
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	sse := datastar.NewSSE(w, r)

	currentUser := userFromContext(r)
	currentID := ""
	if currentUser != nil {
		currentID = currentUser.ID
	}

	rows, err := h.listUsersQuery.Handle(r.Context())
	if err != nil {
		sse.ConsoleError(err)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		sse.ConsoleError(err)
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
		sse := datastar.NewSSE(w, r)
		sse.ConsoleError(nil)
		return
	}

	sse := datastar.NewSSE(w, r)
	if err := h.deleteUserCmd.Handle(r.Context(), commands.DeleteUserCommand{ID: id}); err != nil {
		sse.ConsoleError(err)
		return
	}

	rows, err := h.listUsersQuery.Handle(r.Context())
	if err != nil {
		sse.ConsoleError(err)
		return
	}
	sse.PatchElementTempl(UserTable(rows, current.ID))
}
