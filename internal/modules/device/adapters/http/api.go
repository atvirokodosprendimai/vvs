package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/vvs/isp/internal/infrastructure/http/jsonapi"
	"github.com/vvs/isp/internal/modules/device/app/commands"
	"github.com/vvs/isp/internal/modules/device/app/queries"
	"github.com/vvs/isp/internal/modules/device/domain"
)

// RegisterAPIRoutes implements APIRoutes for the bearer-token-protected REST JSON API.
func (h *DeviceHandlers) RegisterAPIRoutes(r chi.Router) {
	r.Get("/api/v1/devices", h.apiList)
	r.Post("/api/v1/devices", h.apiRegister)
	r.Get("/api/v1/devices/{id}", h.apiGet)
	r.Put("/api/v1/devices/{id}", h.apiUpdate)
	r.Post("/api/v1/devices/{id}/deploy", h.apiDeploy)
	r.Post("/api/v1/devices/{id}/return", h.apiReturn)
	r.Post("/api/v1/devices/{id}/decommission", h.apiDecommission)
}

func (h *DeviceHandlers) apiList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	result, err := h.listQuery.Handle(r.Context(), queries.ListDevicesQuery{
		Status:     q.Get("status"),
		CustomerID: q.Get("customerID"),
		DeviceType: q.Get("type"),
		Search:     q.Get("search"),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		log.Printf("device api: list: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, result)
}

func (h *DeviceHandlers) apiRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string `json:"name"`
		DeviceType   string `json:"deviceType"`
		SerialNumber string `json:"serialNumber"`
		Notes        string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}
	d, err := h.registerCmd.Handle(r.Context(), commands.RegisterDeviceCommand{
		Name: body.Name, DeviceType: body.DeviceType,
		SerialNumber: body.SerialNumber, Notes: body.Notes,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNameRequired) {
			jsonapi.WriteBadRequest(w, err.Error())
			return
		}
		log.Printf("device api: register: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusCreated, d)
}

func (h *DeviceHandlers) apiGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.getQuery.Handle(r.Context(), queries.GetDeviceQuery{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("device api: get: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, d)
}

func (h *DeviceHandlers) apiUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name     string `json:"name"`
		Notes    string `json:"notes"`
		Location string `json:"location"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}
	d, err := h.updateCmd.Handle(r.Context(), commands.UpdateDeviceCommand{
		ID: id, Name: body.Name, Notes: body.Notes, Location: body.Location,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("device api: update: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, d)
}

func (h *DeviceHandlers) apiDeploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		CustomerID string `json:"customerID"`
		Location   string `json:"location"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}
	d, err := h.deployCmd.Handle(r.Context(), commands.DeployDeviceCommand{
		ID: id, CustomerID: body.CustomerID, Location: body.Location,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			jsonapi.WriteError(w, http.StatusConflict, "invalid status transition")
			return
		}
		log.Printf("device api: deploy: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, d)
}

func (h *DeviceHandlers) apiReturn(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.returnCmd.Handle(r.Context(), commands.ReturnDeviceCommand{ID: id}); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			jsonapi.WriteError(w, http.StatusConflict, "invalid status transition")
			return
		}
		log.Printf("device api: return: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, nil)
}

func (h *DeviceHandlers) apiDecommission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.decommissionCmd.Handle(r.Context(), commands.DecommissionDeviceCommand{ID: id}); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			jsonapi.WriteError(w, http.StatusConflict, "invalid status transition")
			return
		}
		log.Printf("device api: decommission: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	jsonapi.WriteJSON(w, http.StatusOK, nil)
}
