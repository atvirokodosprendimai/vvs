package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/http/jsonapi"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
)

// RegisterAPIRoutes implements the APIRoutes interface for the REST JSON API.
func (h *Handlers) RegisterAPIRoutes(r chi.Router) {
	r.Get("/api/v1/customers", h.apiList)
	r.Post("/api/v1/customers", h.apiCreate)
	r.Get("/api/v1/customers/{id}", h.apiGet)
	r.Put("/api/v1/customers/{id}", h.apiUpdate)
	r.Delete("/api/v1/customers/{id}", h.apiDelete)
}

func (h *Handlers) apiList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	search := q.Get("search")

	page := 1
	if s := q.Get("page"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			page = v
		}
	}

	pageSize := 25
	if s := q.Get("pageSize"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			pageSize = v
		}
	}

	result, err := h.listQuery.Handle(r.Context(), queries.ListCustomersQuery{
		Search:   search,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		log.Printf("apiList: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handlers) apiCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CompanyName string `json:"company_name"`
		ContactName string `json:"contact_name"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}

	customer, err := h.createCmd.Handle(r.Context(), commands.CreateCustomerCommand{
		CompanyName: req.CompanyName,
		ContactName: req.ContactName,
		Email:       req.Email,
		Phone:       req.Phone,
	})
	if err != nil {
		if errors.Is(err, domain.ErrCompanyNameRequired) {
			jsonapi.WriteBadRequest(w, err.Error())
			return
		}
		log.Printf("apiCreate: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusCreated, customer)
}

func (h *Handlers) apiGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	customer, err := h.getQuery.Handle(r.Context(), queries.GetCustomerQuery{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrCustomerNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("apiGet: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, customer)
}

func (h *Handlers) apiUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		CompanyName string `json:"company_name"`
		ContactName string `json:"contact_name"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Street      string `json:"street"`
		City        string `json:"city"`
		PostalCode  string `json:"postal_code"`
		Country     string `json:"country"`
		TaxID       string `json:"tax_id"`
		Notes       string `json:"notes"`
		RouterID    string `json:"router_id"`
		IPAddress   string `json:"ip_address"`
		MACAddress  string `json:"mac_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}

	err := h.updateCmd.Handle(r.Context(), commands.UpdateCustomerCommand{
		ID:          id,
		CompanyName: req.CompanyName,
		ContactName: req.ContactName,
		Email:       req.Email,
		Phone:       req.Phone,
		Street:      req.Street,
		City:        req.City,
		PostalCode:  req.PostalCode,
		Country:     req.Country,
		TaxID:       req.TaxID,
		Notes:       req.Notes,
		RouterID:    req.RouterID,
		IPAddress:   req.IPAddress,
		MACAddress:  req.MACAddress,
	})
	if err != nil {
		if errors.Is(err, domain.ErrCustomerNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrCompanyNameRequired) {
			jsonapi.WriteBadRequest(w, err.Error())
			return
		}
		log.Printf("apiUpdate: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Return the updated customer
	customer, err := h.getQuery.Handle(r.Context(), queries.GetCustomerQuery{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrCustomerNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("apiUpdate get: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, customer)
}

func (h *Handlers) apiDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := h.deleteCmd.Handle(r.Context(), commands.DeleteCustomerCommand{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrCustomerNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("apiDelete: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
