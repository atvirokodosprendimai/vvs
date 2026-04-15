package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/starfederation/datastar-go/datastar"
	"gorm.io/gorm"

	"github.com/vvs/isp/internal/modules/customer/app/commands"
	"github.com/vvs/isp/internal/modules/customer/app/queries"
	serviceQueries "github.com/vvs/isp/internal/modules/service/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

// RouterSummary is the minimal router data needed for the customer form dropdown.
// Populated by reading the routers table directly — avoids importing the network module.
type RouterSummary struct {
	ID   string
	Name string
	Host string
}

type Handlers struct {
	createCmd         *commands.CreateCustomerHandler
	updateCmd         *commands.UpdateCustomerHandler
	deleteCmd         *commands.DeleteCustomerHandler
	listQuery         *queries.ListCustomersHandler
	getQuery          *queries.GetCustomerHandler
	subscriber        events.EventSubscriber
	publisher         events.EventPublisher
	reader            *gorm.DB // optional — for router dropdown; nil = no network section shown
	listServicesQuery *serviceQueries.ListServicesForCustomerHandler
}

func NewHandlers(
	createCmd *commands.CreateCustomerHandler,
	updateCmd *commands.UpdateCustomerHandler,
	deleteCmd *commands.DeleteCustomerHandler,
	listQuery *queries.ListCustomersHandler,
	getQuery *queries.GetCustomerHandler,
	subscriber events.EventSubscriber,
	publisher events.EventPublisher,
	listServicesQuery *serviceQueries.ListServicesForCustomerHandler,
) *Handlers {
	return &Handlers{
		createCmd:         createCmd,
		updateCmd:         updateCmd,
		deleteCmd:         deleteCmd,
		listQuery:         listQuery,
		getQuery:          getQuery,
		subscriber:        subscriber,
		publisher:         publisher,
		listServicesQuery: listServicesQuery,
	}
}

// WithReader enables the router dropdown on the customer form by reading the
// routers table from the shared SQLite reader. Call from app.go when the
// network module is enabled.
func (h *Handlers) WithReader(reader *gorm.DB) *Handlers {
	h.reader = reader
	return h
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
	r.Post("/api/customers/{id}/arp", h.arpSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	CustomerListPage().Render(r.Context(), w)
}

func (h *Handlers) createPage(w http.ResponseWriter, r *http.Request) {
	routers := h.loadRouters(r.Context())
	CustomerFormPage(nil, routers).Render(r.Context(), w)
}

func (h *Handlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customer, err := h.getQuery.Handle(r.Context(), queries.GetCustomerQuery{ID: id})
	if err != nil {
		http.Error(w, "Customer not found", http.StatusNotFound)
		return
	}
	var services []serviceQueries.ServiceReadModel
	if h.listServicesQuery != nil {
		services, err = h.listServicesQuery.Handle(r.Context(), serviceQueries.ListServicesForCustomerQuery{CustomerID: id})
		if err != nil {
			log.Printf("detailPage: list services: %v", err)
			services = nil
		}
	}
	CustomerDetailPage(customer, services).Render(r.Context(), w)
}

func (h *Handlers) editPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customer, err := h.getQuery.Handle(r.Context(), queries.GetCustomerQuery{ID: id})
	if err != nil {
		http.Error(w, "Customer not found", http.StatusNotFound)
		return
	}
	routers := h.loadRouters(r.Context())
	CustomerFormPage(customer, routers).Render(r.Context(), w)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Search   string `json:"search"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("handler: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	if signals.PageSize == 0 {
		signals.PageSize = 25
	}

	// Subscribe before initial render so no event is missed
	ch, cancel := h.subscriber.ChanSubscription("isp.customer.*")
	defer cancel()

	q := queries.ListCustomersQuery{
		Search:   signals.Search,
		Page:     signals.Page,
		PageSize: signals.PageSize,
	}

	// current is the server-side record of what FE has rendered
	current, err := h.listQuery.Handle(r.Context(), q)
	if err != nil {
		log.Printf("handler error: %v", err)
		return
	}
	sse.PatchElementTempl(CustomerTable(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listQuery.Handle(r.Context(), q)
			if err != nil {
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(CustomerTable(next))
				current = next
			}
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
		log.Printf("handler: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
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
		RouterID    string `json:"routerId"`
		IPAddress   string `json:"ipAddress"`
		MACAddress  string `json:"macAddress"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("handler: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
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
		RouterID:    signals.RouterID,
		IPAddress:   signals.IPAddress,
		MACAddress:  signals.MACAddress,
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
		log.Printf("handler error: %v", err)
		return
	}

	sse.Redirect("/customers")
}

// arpSSE publishes isp.network.arp_requested and redirects back to the detail page.
// The network module's ARPWorker handles the event asynchronously.
func (h *Handlers) arpSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	var signals struct {
		Action string `json:"arpAction"` // "enable" | "disable"
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("handler: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	type arpRequestedPayload struct {
		CustomerID string `json:"customer_id"`
		Action     string `json:"action"`
	}
	data, _ := json.Marshal(arpRequestedPayload{CustomerID: id, Action: signals.Action})
	h.publisher.Publish(r.Context(), "isp.network.arp_requested", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "network.arp_requested",
		AggregateID: id,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	sse.Redirect("/customers/" + id)
}

// loadRouters reads the routers table directly from the shared SQLite reader.
// Returns nil when reader is not set (network module disabled).
func (h *Handlers) loadRouters(ctx context.Context) []RouterSummary {
	if h.reader == nil {
		return nil
	}
	var rows []struct {
		ID   string `gorm:"column:id"`
		Name string `gorm:"column:name"`
		Host string `gorm:"column:host"`
	}
	h.reader.WithContext(ctx).Raw("SELECT id, name, host FROM routers ORDER BY name").Scan(&rows)
	summaries := make([]RouterSummary, len(rows))
	for i, row := range rows {
		summaries[i] = RouterSummary{ID: row.ID, Name: row.Name, Host: row.Host}
	}
	return summaries
}
