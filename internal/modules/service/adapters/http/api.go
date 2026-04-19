package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/http/jsonapi"
	"github.com/atvirokodosprendimai/vvs/internal/modules/service/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/service/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/service/domain"
)

// RegisterAPIRoutes implements APIRoutes for the bearer-token-protected REST JSON API.
func (h *ServiceHandlers) RegisterAPIRoutes(r chi.Router) {
	r.Get("/api/v1/customers/{id}/services", h.apiListServices)
	r.Post("/api/v1/customers/{id}/services", h.apiAssignService)
	r.Put("/api/v1/services/{serviceID}/suspend", h.apiSuspendService)
	r.Put("/api/v1/services/{serviceID}/reactivate", h.apiReactivateService)
	r.Delete("/api/v1/services/{serviceID}", h.apiCancelService)
}

func (h *ServiceHandlers) apiListServices(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		jsonapi.WriteBadRequest(w, "missing customer id")
		return
	}

	result, err := h.listQuery.Handle(r.Context(), queries.ListServicesForCustomerQuery{CustomerID: customerID})
	if err != nil {
		log.Printf("service api: apiListServices: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, result)
}

func (h *ServiceHandlers) apiAssignService(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		jsonapi.WriteBadRequest(w, "missing customer id")
		return
	}

	var body struct {
		ProductID   string `json:"productID"`
		ProductName string `json:"productName"`
		PriceAmount int64  `json:"priceAmount"`
		Currency    string `json:"currency"`
		StartDate   string `json:"startDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonapi.WriteBadRequest(w, "invalid JSON body")
		return
	}

	startDate, err := time.Parse("2006-01-02", body.StartDate)
	if err != nil {
		jsonapi.WriteBadRequest(w, "invalid startDate: expected YYYY-MM-DD")
		return
	}

	currency := body.Currency
	if currency == "" {
		currency = "EUR"
	}

	svc, err := h.assignCmd.Handle(r.Context(), commands.AssignServiceCommand{
		CustomerID:  customerID,
		ProductID:   body.ProductID,
		ProductName: body.ProductName,
		PriceAmount: body.PriceAmount,
		Currency:    currency,
		StartDate:   startDate,
	})
	if err != nil {
		log.Printf("service api: apiAssignService: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandlers) apiSuspendService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if serviceID == "" {
		jsonapi.WriteBadRequest(w, "missing service id")
		return
	}

	if err := h.suspendCmd.Handle(r.Context(), commands.SuspendServiceCommand{ID: serviceID}); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			jsonapi.WriteError(w, http.StatusConflict, "invalid status transition")
			return
		}
		log.Printf("service api: apiSuspendService: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, nil)
}

func (h *ServiceHandlers) apiReactivateService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if serviceID == "" {
		jsonapi.WriteBadRequest(w, "missing service id")
		return
	}

	if err := h.reactivateCmd.Handle(r.Context(), commands.ReactivateServiceCommand{ID: serviceID}); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			jsonapi.WriteError(w, http.StatusConflict, "invalid status transition")
			return
		}
		log.Printf("service api: apiReactivateService: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, nil)
}

func (h *ServiceHandlers) apiCancelService(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if serviceID == "" {
		jsonapi.WriteBadRequest(w, "missing service id")
		return
	}

	if err := h.cancelCmd.Handle(r.Context(), commands.CancelServiceCommand{ID: serviceID}); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			jsonapi.WriteError(w, http.StatusConflict, "invalid status transition")
			return
		}
		log.Printf("service api: apiCancelService: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
