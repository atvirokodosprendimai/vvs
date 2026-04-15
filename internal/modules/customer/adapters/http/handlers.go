package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/customer/app/commands"
	"github.com/vvs/isp/internal/modules/customer/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

type Handlers struct {
	createCmd  *commands.CreateCustomerHandler
	updateCmd  *commands.UpdateCustomerHandler
	deleteCmd  *commands.DeleteCustomerHandler
	listQuery  *queries.ListCustomersHandler
	getQuery   *queries.GetCustomerHandler
	subscriber events.EventSubscriber
}

func NewHandlers(
	createCmd *commands.CreateCustomerHandler,
	updateCmd *commands.UpdateCustomerHandler,
	deleteCmd *commands.DeleteCustomerHandler,
	listQuery *queries.ListCustomersHandler,
	getQuery *queries.GetCustomerHandler,
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
	r.Get("/customers", h.listPage)
	r.Get("/customers/new", h.createPage)
	r.Get("/customers/{id}", h.detailPage)
	r.Get("/customers/{id}/edit", h.editPage)

	r.Get("/api/customers", h.listSSE)
	r.Post("/api/customers", h.createSSE)
	r.Put("/api/customers/{id}", h.updateSSE)
	r.Delete("/api/customers/{id}", h.deleteSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	CustomerListPage().Render(r.Context(), w)
}

func (h *Handlers) createPage(w http.ResponseWriter, r *http.Request) {
	CustomerFormPage(nil).Render(r.Context(), w)
}

func (h *Handlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customer, err := h.getQuery.Handle(r.Context(), queries.GetCustomerQuery{ID: id})
	if err != nil {
		http.Error(w, "Customer not found", http.StatusNotFound)
		return
	}
	CustomerDetailPage(customer).Render(r.Context(), w)
}

func (h *Handlers) editPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customer, err := h.getQuery.Handle(r.Context(), queries.GetCustomerQuery{ID: id})
	if err != nil {
		http.Error(w, "Customer not found", http.StatusNotFound)
		return
	}
	CustomerFormPage(customer).Render(r.Context(), w)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Search   string `json:"search"`
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

	// Subscribe before initial render so no event is missed
	ch, cancel := h.subscriber.ChanSubscription("isp.customer.*")
	defer cancel()

	result, err := h.listQuery.Handle(r.Context(), queries.ListCustomersQuery{
		Search:   signals.Search,
		Page:     signals.Page,
		PageSize: signals.PageSize,
	})
	if err != nil {
		sse.ConsoleError(err)
		return
	}
	sse.PatchElementTempl(CustomerTable(result))

	// Live updates: patch individual row per NATS event — no re-query
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			var rm queries.CustomerReadModel
			if err := json.Unmarshal(event.Data, &rm); err != nil {
				continue
			}
			sse.PatchElementTempl(CustomerRow(rm.ToDomain()))
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) createSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		CompanyName string `json:"companyName"`
		ContactName string `json:"contactName"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	_, err := h.createCmd.Handle(r.Context(), commands.CreateCustomerCommand{
		CompanyName: signals.CompanyName,
		ContactName: signals.ContactName,
		Email:       signals.Email,
		Phone:       signals.Phone,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/customers")
}

func (h *Handlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var signals struct {
		CompanyName string `json:"companyName"`
		ContactName string `json:"contactName"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Street      string `json:"street"`
		City        string `json:"city"`
		PostalCode  string `json:"postalCode"`
		Country     string `json:"country"`
		TaxID       string `json:"taxId"`
		Notes       string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	err := h.updateCmd.Handle(r.Context(), commands.UpdateCustomerCommand{
		ID:          id,
		CompanyName: signals.CompanyName,
		ContactName: signals.ContactName,
		Email:       signals.Email,
		Phone:       signals.Phone,
		Street:      signals.Street,
		City:        signals.City,
		PostalCode:  signals.PostalCode,
		Country:     signals.Country,
		TaxID:       signals.TaxID,
		Notes:       signals.Notes,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/customers/" + id)
}

func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	err := h.deleteCmd.Handle(r.Context(), commands.DeleteCustomerCommand{ID: id})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.Redirect("/customers")
}
