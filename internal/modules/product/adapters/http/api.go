package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/http/jsonapi"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/domain"
)

func (h *Handlers) RegisterAPIRoutes(r chi.Router) {
	r.Get("/api/v1/products", h.apiListProducts)
	r.Post("/api/v1/products", h.apiCreateProduct)
	r.Get("/api/v1/products/{id}", h.apiGetProduct)
	r.Put("/api/v1/products/{id}", h.apiUpdateProduct)
	r.Delete("/api/v1/products/{id}", h.apiDeleteProduct)
}

func (h *Handlers) apiListProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	search := q.Get("search")

	page := 1
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}

	pageSize := 25
	if v := q.Get("pageSize"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}

	result, err := h.listQuery.Handle(r.Context(), queries.ListProductsQuery{
		Search:   search,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		log.Printf("api: list products: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handlers) apiCreateProduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Type          string `json:"type"`
		PriceAmount   int64  `json:"price_amount"`
		PriceCurrency string `json:"price_currency"`
		BillingPeriod string `json:"billing_period"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonapi.WriteBadRequest(w, "invalid request body")
		return
	}

	product, err := h.createCmd.Handle(r.Context(), commands.CreateProductCommand{
		Name:          req.Name,
		Description:   req.Description,
		Type:          req.Type,
		PriceAmount:   req.PriceAmount,
		PriceCurrency: req.PriceCurrency,
		BillingPeriod: req.BillingPeriod,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNameRequired) ||
			errors.Is(err, domain.ErrInvalidType) ||
			errors.Is(err, domain.ErrInvalidBilling) {
			jsonapi.WriteBadRequest(w, err.Error())
			return
		}
		log.Printf("api: create product: %v", err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusCreated, product)
}

func (h *Handlers) apiGetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	product, err := h.getQuery.Handle(r.Context(), queries.GetProductQuery{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("api: get product %s: %v", id, err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, product)
}

func (h *Handlers) apiUpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Type          string `json:"type"`
		PriceAmount   int64  `json:"price_amount"`
		PriceCurrency string `json:"price_currency"`
		BillingPeriod string `json:"billing_period"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonapi.WriteBadRequest(w, "invalid request body")
		return
	}

	err := h.updateCmd.Handle(r.Context(), commands.UpdateProductCommand{
		ID:            id,
		Name:          req.Name,
		Description:   req.Description,
		Type:          req.Type,
		PriceAmount:   req.PriceAmount,
		PriceCurrency: req.PriceCurrency,
		BillingPeriod: req.BillingPeriod,
	})
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		if errors.Is(err, domain.ErrNameRequired) ||
			errors.Is(err, domain.ErrInvalidType) ||
			errors.Is(err, domain.ErrInvalidBilling) {
			jsonapi.WriteBadRequest(w, err.Error())
			return
		}
		log.Printf("api: update product %s: %v", id, err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	product, err := h.getQuery.Handle(r.Context(), queries.GetProductQuery{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("api: get product after update %s: %v", id, err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jsonapi.WriteJSON(w, http.StatusOK, product)
}

func (h *Handlers) apiDeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := h.deleteCmd.Handle(r.Context(), commands.DeleteProductCommand{ID: id})
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			jsonapi.WriteNotFound(w)
			return
		}
		log.Printf("api: delete product %s: %v", id, err)
		jsonapi.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
