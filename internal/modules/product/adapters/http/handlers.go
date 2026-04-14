package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/product/app/commands"
	"github.com/vvs/isp/internal/modules/product/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

type Handlers struct {
	createCmd  *commands.CreateProductHandler
	updateCmd  *commands.UpdateProductHandler
	deleteCmd  *commands.DeleteProductHandler
	listQuery  *queries.ListProductsHandler
	getQuery   *queries.GetProductHandler
	subscriber events.EventSubscriber
}

func NewHandlers(
	createCmd *commands.CreateProductHandler,
	updateCmd *commands.UpdateProductHandler,
	deleteCmd *commands.DeleteProductHandler,
	listQuery *queries.ListProductsHandler,
	getQuery *queries.GetProductHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		createCmd:  createCmd,
		updateCmd:  updateCmd,
		deleteCmd:  deleteCmd,
		listQuery:  listQuery,
		getQuery:   getQuery,
		subscriber: subscriber,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/products", h.listPage)
	r.Get("/products/new", h.createPage)
	r.Get("/products/{id}", h.detailPage)
	r.Get("/products/{id}/edit", h.editPage)

	r.Get("/api/products", h.listSSE)
	r.Post("/api/products", h.createSSE)
	r.Put("/api/products/{id}", h.updateSSE)
	r.Delete("/api/products/{id}", h.deleteSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	ProductListPage().Render(r.Context(), w)
}

func (h *Handlers) createPage(w http.ResponseWriter, r *http.Request) {
	ProductFormPage(nil).Render(r.Context(), w)
}

func (h *Handlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	product, err := h.getQuery.Handle(r.Context(), queries.GetProductQuery{ID: id})
	if err != nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	ProductDetailPage(product).Render(r.Context(), w)
}

func (h *Handlers) editPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	product, err := h.getQuery.Handle(r.Context(), queries.GetProductQuery{ID: id})
	if err != nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	ProductFormPage(product).Render(r.Context(), w)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Search   string `json:"search"`
		Type     string `json:"filterType"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	if signals.PageSize == 0 {
		signals.PageSize = 25
	}

	result, err := h.listQuery.Handle(r.Context(), queries.ListProductsQuery{
		Search:   signals.Search,
		Type:     signals.Type,
		Page:     signals.Page,
		PageSize: signals.PageSize,
	})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.PatchElementTempl(ProductTable(result))
}

func (h *Handlers) createSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Type          string `json:"productType"`
		PriceAmount   string `json:"priceAmount"`
		PriceCurrency string `json:"priceCurrency"`
		BillingPeriod string `json:"billingPeriod"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	priceAmount, err := parsePriceCents(signals.PriceAmount)
	if err != nil {
		sse.PatchElementTempl(formError("Invalid price amount"))
		return
	}

	currency := signals.PriceCurrency
	if currency == "" {
		currency = "EUR"
	}

	_, err = h.createCmd.Handle(r.Context(), commands.CreateProductCommand{
		Name:          signals.Name,
		Description:   signals.Description,
		Type:          signals.Type,
		PriceAmount:   priceAmount,
		PriceCurrency: currency,
		BillingPeriod: signals.BillingPeriod,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/products")
}

func (h *Handlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var signals struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Type          string `json:"productType"`
		PriceAmount   string `json:"priceAmount"`
		PriceCurrency string `json:"priceCurrency"`
		BillingPeriod string `json:"billingPeriod"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	priceAmount, err := parsePriceCents(signals.PriceAmount)
	if err != nil {
		sse.PatchElementTempl(formError("Invalid price amount"))
		return
	}

	currency := signals.PriceCurrency
	if currency == "" {
		currency = "EUR"
	}

	err = h.updateCmd.Handle(r.Context(), commands.UpdateProductCommand{
		ID:            id,
		Name:          signals.Name,
		Description:   signals.Description,
		Type:          signals.Type,
		PriceAmount:   priceAmount,
		PriceCurrency: currency,
		BillingPeriod: signals.BillingPeriod,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/products/" + id)
}

func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	err := h.deleteCmd.Handle(r.Context(), commands.DeleteProductCommand{ID: id})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.Redirect("/products")
}

func parsePriceCents(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f * 100), nil
}
