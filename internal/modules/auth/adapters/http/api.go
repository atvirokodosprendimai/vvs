package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vvs/isp/internal/infrastructure/http/jsonapi"
	"github.com/vvs/isp/internal/modules/auth/app/commands"
	"github.com/vvs/isp/internal/modules/auth/domain"
)

func (h *Handlers) RegisterAPIRoutes(r chi.Router) {
	r.Get("/api/v1/users", h.apiListUsers)
	r.Post("/api/v1/users", h.apiCreateUser)
	r.Delete("/api/v1/users/{id}", h.apiDeleteUser)
}

func (h *Handlers) apiListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.listUsersQuery.Handle(r.Context())
	if err != nil {
		log.Printf("apiListUsers: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	type userResponse struct {
		ID        string `json:"id"`
		Username  string `json:"username"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	}

	out := make([]userResponse, len(rows))
	for i, u := range rows {
		out[i] = userResponse{
			ID:        u.ID,
			Username:  u.Username,
			Role:      string(u.Role),
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	jsonapi.WriteJSON(w, http.StatusOK, out)
}

func (h *Handlers) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}

	role := domain.Role(body.Role)
	if !domain.ValidRole(role) {
		jsonapi.WriteBadRequest(w, "role must be admin or operator")
		return
	}

	u, err := h.createUserCmd.Handle(r.Context(), commands.CreateUserCommand{
		Username: body.Username,
		Password: body.Password,
		Role:     role,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUsernameRequired),
			errors.Is(err, domain.ErrPasswordRequired),
			errors.Is(err, domain.ErrInvalidRole),
			errors.Is(err, domain.ErrUsernameTaken):
			jsonapi.WriteBadRequest(w, err.Error())
		default:
			log.Printf("apiCreateUser: %v", err)
			jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	jsonapi.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":         u.ID,
		"username":   u.Username,
		"role":       string(u.Role),
		"created_at": u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *Handlers) apiDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.deleteUserCmd.Handle(r.Context(), commands.DeleteUserCommand{ID: id}); err != nil {
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			jsonapi.WriteNotFound(w)
		default:
			log.Printf("apiDeleteUser: %v", err)
			jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
