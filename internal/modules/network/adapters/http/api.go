package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vvs/isp/internal/infrastructure/http/jsonapi"
	"github.com/vvs/isp/internal/modules/network/app/commands"
	"github.com/vvs/isp/internal/modules/network/domain"
)

// RegisterAPIRoutes registers REST JSON endpoints under /api/v1/routers.
// Implements infrastructure/http.APIRoutes.
func (h *Handlers) RegisterAPIRoutes(r chi.Router) {
	r.Get("/api/v1/routers", h.apiListRouters)
	r.Post("/api/v1/routers", h.apiCreateRouter)
	r.Get("/api/v1/routers/{id}", h.apiGetRouter)
	r.Put("/api/v1/routers/{id}", h.apiUpdateRouter)
	r.Delete("/api/v1/routers/{id}", h.apiDeleteRouter)
	r.Post("/api/v1/customers/{id}/arp", h.apiSyncCustomerARP)
}

func (h *Handlers) apiListRouters(w http.ResponseWriter, r *http.Request) {
	routers, err := h.listQuery.Handle(r.Context())
	if err != nil {
		log.Printf("api: list routers: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, routers)
}

func (h *Handlers) apiGetRouter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	router, err := h.getQuery.Handle(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrRouterNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("api: get router %s: %v", id, err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, router)
}

func (h *Handlers) apiCreateRouter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string `json:"name"`
		RouterType string `json:"router_type"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}

	router, err := h.createCmd.Handle(r.Context(), commands.CreateRouterCommand{
		Name:       body.Name,
		RouterType: body.RouterType,
		Host:       body.Host,
		Port:       body.Port,
		Username:   body.Username,
		Password:   body.Password,
		Notes:      body.Notes,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNameRequired) ||
			errors.Is(err, domain.ErrHostRequired) ||
			errors.Is(err, domain.ErrInvalidRouterType) {
			jsonapi.WriteBadRequest(w, err.Error())
			return
		}
		log.Printf("api: create router: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Return read model (no password)
	rm, err := h.getQuery.Handle(r.Context(), router.ID)
	if err != nil {
		log.Printf("api: create router: fetch after create: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusCreated, rm)
}

func (h *Handlers) apiUpdateRouter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var body struct {
		Name       string `json:"name"`
		RouterType string `json:"router_type"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
		Username   string `json:"username"`
		Password   string `json:"password"` // empty = keep existing
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}

	_, err := h.updateCmd.Handle(r.Context(), commands.UpdateRouterCommand{
		ID:         id,
		Name:       body.Name,
		RouterType: body.RouterType,
		Host:       body.Host,
		Port:       body.Port,
		Username:   body.Username,
		Password:   body.Password,
		Notes:      body.Notes,
	})
	if err != nil {
		if errors.Is(err, domain.ErrRouterNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrNameRequired) ||
			errors.Is(err, domain.ErrHostRequired) ||
			errors.Is(err, domain.ErrInvalidRouterType) {
			jsonapi.WriteBadRequest(w, err.Error())
			return
		}
		log.Printf("api: update router %s: %v", id, err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	rm, err := h.getQuery.Handle(r.Context(), id)
	if err != nil {
		log.Printf("api: update router: fetch after update: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, rm)
}

func (h *Handlers) apiDeleteRouter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.deleteCmd.Handle(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrRouterNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("api: delete router %s: %v", id, err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) apiSyncCustomerARP(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")

	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}
	if body.Action != commands.ARPActionEnable && body.Action != commands.ARPActionDisable {
		jsonapi.WriteBadRequest(w, `action must be "enable" or "disable"`)
		return
	}

	if err := h.syncARPCmd.Handle(r.Context(), commands.SyncCustomerARPCommand{
		CustomerID: customerID,
		Action:     body.Action,
	}); err != nil {
		log.Printf("api: sync arp customer %s: %v", customerID, err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
