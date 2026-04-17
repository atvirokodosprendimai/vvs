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
	"github.com/vvs/isp/internal/modules/customer/domain"
	contactQueries "github.com/vvs/isp/internal/modules/contact/app/queries"
	dealQueries "github.com/vvs/isp/internal/modules/deal/app/queries"
	ticketQueries "github.com/vvs/isp/internal/modules/ticket/app/queries"
	taskQueries "github.com/vvs/isp/internal/modules/task/app/queries"
	serviceQueries "github.com/vvs/isp/internal/modules/service/app/queries"
	servicehttp "github.com/vvs/isp/internal/modules/service/adapters/http"
	contacthttp "github.com/vvs/isp/internal/modules/contact/adapters/http"
	dealhttp "github.com/vvs/isp/internal/modules/deal/adapters/http"
	tickethttp "github.com/vvs/isp/internal/modules/ticket/adapters/http"
	taskhttp "github.com/vvs/isp/internal/modules/task/adapters/http"
	emailhttp "github.com/vvs/isp/internal/modules/email/adapters/http"
	emailQueries "github.com/vvs/isp/internal/modules/email/app/queries"
	invoiceQueries "github.com/vvs/isp/internal/modules/invoice/app/queries"
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
	changeStatusCmd   *commands.ChangeCustomerStatusHandler
	addNoteCmd        *commands.AddNoteHandler
	listQuery         *queries.ListCustomersHandler
	getQuery          *queries.GetCustomerHandler
	listNotesQuery    *queries.ListNotesHandler
	subscriber           events.EventSubscriber
	publisher            events.EventPublisher
	reader               *gorm.DB // optional — for router dropdown; nil = no network section shown
	listServicesQuery    *serviceQueries.ListServicesForCustomerHandler
	listContactsQuery    *contactQueries.ListContactsForCustomerHandler
	listDealsQuery       *dealQueries.ListDealsForCustomerHandler
	listTicketsQuery     *ticketQueries.ListTicketsForCustomerHandler
	listTasksQuery       *taskQueries.ListTasksForCustomerHandler
	listEmailQuery       *emailQueries.ListThreadsForCustomerHandler
	listInvoicesQuery    *invoiceQueries.ListInvoicesForCustomerHandler
}

func NewHandlers(
	createCmd *commands.CreateCustomerHandler,
	updateCmd *commands.UpdateCustomerHandler,
	deleteCmd *commands.DeleteCustomerHandler,
	changeStatusCmd *commands.ChangeCustomerStatusHandler,
	addNoteCmd *commands.AddNoteHandler,
	listQuery *queries.ListCustomersHandler,
	getQuery *queries.GetCustomerHandler,
	listNotesQuery *queries.ListNotesHandler,
	subscriber events.EventSubscriber,
	publisher events.EventPublisher,
	listServicesQuery *serviceQueries.ListServicesForCustomerHandler,
) *Handlers {
	return &Handlers{
		createCmd:         createCmd,
		updateCmd:         updateCmd,
		deleteCmd:         deleteCmd,
		changeStatusCmd:   changeStatusCmd,
		addNoteCmd:        addNoteCmd,
		listQuery:         listQuery,
		getQuery:          getQuery,
		listNotesQuery:    listNotesQuery,
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

// WithContactsQuery injects the contact list query for the detail page.
func (h *Handlers) WithContactsQuery(q *contactQueries.ListContactsForCustomerHandler) *Handlers {
	h.listContactsQuery = q
	return h
}

// WithDealsQuery injects the deal list query for the detail page.
func (h *Handlers) WithDealsQuery(q *dealQueries.ListDealsForCustomerHandler) *Handlers {
	h.listDealsQuery = q
	return h
}

// WithTicketsQuery injects the ticket list query for the detail page.
func (h *Handlers) WithTicketsQuery(q *ticketQueries.ListTicketsForCustomerHandler) *Handlers {
	h.listTicketsQuery = q
	return h
}

// WithTasksQuery injects the task list query for the detail page.
func (h *Handlers) WithTasksQuery(q *taskQueries.ListTasksForCustomerHandler) *Handlers {
	h.listTasksQuery = q
	return h
}

// WithEmailThreadsQuery injects the email threads query for the customer detail page.
func (h *Handlers) WithEmailThreadsQuery(q *emailQueries.ListThreadsForCustomerHandler) *Handlers {
	h.listEmailQuery = q
	return h
}

// WithInvoicesQuery injects the invoice list query for the customer detail page.
func (h *Handlers) WithInvoicesQuery(q *invoiceQueries.ListInvoicesForCustomerHandler) *Handlers {
	h.listInvoicesQuery = q
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
	r.Post("/api/customers/{id}/status", h.changeStatusSSE)
	r.Get("/sse/customers/{id}/crm", h.crmLiveSSE)
	r.Get("/sse/customers/{id}/notes", h.listNotesSSE)
	r.Post("/api/customers/{id}/notes", h.addNoteSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	CustomerListPage().Render(r.Context(), w)
}

func (h *Handlers) createPage(w http.ResponseWriter, r *http.Request) {
	routers := h.loadRouters(r.Context())
	zones := h.loadZones(r.Context())
	CustomerFormPage(nil, routers, zones).Render(r.Context(), w)
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
	var contacts []contactQueries.ContactReadModel
	if h.listContactsQuery != nil {
		contacts, err = h.listContactsQuery.Handle(r.Context(), contactQueries.ListContactsForCustomerQuery{CustomerID: id})
		if err != nil {
			log.Printf("detailPage: list contacts: %v", err)
			contacts = nil
		}
	}
	var deals []dealQueries.DealReadModel
	if h.listDealsQuery != nil {
		deals, err = h.listDealsQuery.Handle(r.Context(), dealQueries.ListDealsForCustomerQuery{CustomerID: id})
		if err != nil {
			log.Printf("detailPage: list deals: %v", err)
			deals = nil
		}
	}
	var tickets []ticketQueries.TicketReadModel
	if h.listTicketsQuery != nil {
		tickets, err = h.listTicketsQuery.Handle(r.Context(), ticketQueries.ListTicketsForCustomerQuery{CustomerID: id})
		if err != nil {
			log.Printf("detailPage: list tickets: %v", err)
			tickets = nil
		}
	}
	var tasks []taskQueries.TaskReadModel
	if h.listTasksQuery != nil {
		tasks, err = h.listTasksQuery.Handle(r.Context(), taskQueries.ListTasksForCustomerQuery{CustomerID: id})
		if err != nil {
			log.Printf("detailPage: list tasks: %v", err)
			tasks = nil
		}
	}
	var emailThreads []emailQueries.ThreadReadModel
	if h.listEmailQuery != nil {
		emailThreads, err = h.listEmailQuery.Handle(r.Context(), id)
		if err != nil {
			log.Printf("detailPage: list email threads: %v", err)
			emailThreads = nil
		}
	}
	var invoices []invoiceQueries.InvoiceReadModel
	if h.listInvoicesQuery != nil {
		invoices, err = h.listInvoicesQuery.Handle(r.Context(), invoiceQueries.ListInvoicesForCustomerQuery{CustomerID: id})
		if err != nil {
			log.Printf("detailPage: list invoices: %v", err)
			invoices = nil
		}
	}
	routerName := h.loadRouterName(r.Context(), customer)
	var notes []queries.NoteReadModel
	if h.listNotesQuery != nil {
		notes, _ = h.listNotesQuery.Handle(r.Context(), id)
	}
	CustomerDetailPage(customer, services, routerName, notes, contacts, deals, tickets, tasks, emailThreads, invoices).Render(r.Context(), w)
}

func (h *Handlers) editPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	customer, err := h.getQuery.Handle(r.Context(), queries.GetCustomerQuery{ID: id})
	if err != nil {
		http.Error(w, "Customer not found", http.StatusNotFound)
		return
	}
	routers := h.loadRouters(r.Context())
	zones := h.loadZones(r.Context())
	CustomerFormPage(customer, routers, zones).Render(r.Context(), w)
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
	ch, cancel := h.subscriber.ChanSubscription(events.CustomerAll.String())
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
		NetworkZone string `json:"networkZone"`
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
		NetworkZone: signals.NetworkZone,
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
		NetworkZone string `json:"networkZone"`
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
		NetworkZone: signals.NetworkZone,
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
	id := chi.URLParam(r, "id")

	var signals struct {
		Action string `json:"arpAction"` // "enable" | "disable"
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("handler: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	type arpRequestedPayload struct {
		CustomerID string `json:"customer_id"`
		Action     string `json:"action"`
	}
	data, _ := json.Marshal(arpRequestedPayload{CustomerID: id, Action: signals.Action})
	h.publisher.Publish(r.Context(), events.NetworkARPRequested.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "network.arp_requested",
		AggregateID: id,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	sse.Redirect("/customers/" + id)
}

func (h *Handlers) changeStatusSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var signals struct {
		Action string `json:"statusAction"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("changeStatus: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.changeStatusCmd.Handle(r.Context(), commands.ChangeCustomerStatusCommand{ID: id, Action: signals.Action}); err != nil {
		log.Printf("changeStatus %s %s: %v", id, signals.Action, err)
		sse.PatchElementTempl(formError(err.Error()))
		return
	}
	sse.Redirect("/customers/" + id)
}

// crmLiveSSE is a single combined SSE stream for all CRM sections on the
// customer detail page. Using one stream instead of many keeps us under the
// browser HTTP/1.1 per-domain connection limit of 6.
func (h *Handlers) crmLiveSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)

	// Broad wildcard — any ISP event triggers a diff-and-patch cycle.
	ch, cancel := h.subscriber.ChanSubscription(events.Everything.String())
	defer cancel()

	type state struct {
		services []serviceQueries.ServiceReadModel
		contacts []contactQueries.ContactReadModel
		deals    []dealQueries.DealReadModel
		tickets  []ticketQueries.TicketReadModel
		tasks    []taskQueries.TaskReadModel
		emails   []emailQueries.ThreadReadModel
		invoices []invoiceQueries.InvoiceReadModel
	}

	queryAll := func() state {
		var s state
		if h.listServicesQuery != nil {
			s.services, _ = h.listServicesQuery.Handle(r.Context(), serviceQueries.ListServicesForCustomerQuery{CustomerID: customerID})
		}
		if h.listContactsQuery != nil {
			s.contacts, _ = h.listContactsQuery.Handle(r.Context(), contactQueries.ListContactsForCustomerQuery{CustomerID: customerID})
		}
		if h.listDealsQuery != nil {
			s.deals, _ = h.listDealsQuery.Handle(r.Context(), dealQueries.ListDealsForCustomerQuery{CustomerID: customerID})
		}
		if h.listTicketsQuery != nil {
			s.tickets, _ = h.listTicketsQuery.Handle(r.Context(), ticketQueries.ListTicketsForCustomerQuery{CustomerID: customerID})
		}
		if h.listTasksQuery != nil {
			s.tasks, _ = h.listTasksQuery.Handle(r.Context(), taskQueries.ListTasksForCustomerQuery{CustomerID: customerID})
		}
		if h.listEmailQuery != nil {
			s.emails, _ = h.listEmailQuery.Handle(r.Context(), customerID)
		}
		if h.listInvoicesQuery != nil {
			s.invoices, _ = h.listInvoicesQuery.Handle(r.Context(), invoiceQueries.ListInvoicesForCustomerQuery{CustomerID: customerID})
		}
		return s
	}

	patchTabBar := func(s state) {
		sse.PatchElementTempl(CRMTabBar(len(s.services), len(s.contacts), len(s.deals), len(s.tickets), len(s.tasks), len(s.emails), len(s.invoices)))
	}

	cur := queryAll()
	sse.PatchElementTempl(servicehttp.ServiceTable(customerID, cur.services))
	sse.PatchElementTempl(contacthttp.ContactList(customerID, cur.contacts))
	sse.PatchElementTempl(dealhttp.DealList(customerID, cur.deals))
	sse.PatchElementTempl(tickethttp.TicketList(customerID, cur.tickets))
	sse.PatchElementTempl(taskhttp.TaskList(customerID, cur.tasks))
	sse.PatchElementTempl(emailhttp.EmailThreadsSection(customerID, cur.emails))
	sse.PatchElementTempl(InvoiceSection(customerID, cur.invoices))
	patchTabBar(cur)

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next := queryAll()
			changed := false
			if !reflect.DeepEqual(cur.services, next.services) {
				sse.PatchElementTempl(servicehttp.ServiceTable(customerID, next.services))
				changed = true
			}
			if !reflect.DeepEqual(cur.contacts, next.contacts) {
				sse.PatchElementTempl(contacthttp.ContactList(customerID, next.contacts))
				changed = true
			}
			if !reflect.DeepEqual(cur.deals, next.deals) {
				sse.PatchElementTempl(dealhttp.DealList(customerID, next.deals))
				changed = true
			}
			if !reflect.DeepEqual(cur.tickets, next.tickets) {
				sse.PatchElementTempl(tickethttp.TicketList(customerID, next.tickets))
				changed = true
			}
			if !reflect.DeepEqual(cur.tasks, next.tasks) {
				sse.PatchElementTempl(taskhttp.TaskList(customerID, next.tasks))
				changed = true
			}
			if !reflect.DeepEqual(cur.emails, next.emails) {
				sse.PatchElementTempl(emailhttp.EmailThreadsSection(customerID, next.emails))
				changed = true
			}
			if !reflect.DeepEqual(cur.invoices, next.invoices) {
				sse.PatchElementTempl(InvoiceSection(customerID, next.invoices))
				changed = true
			}
			if changed {
				patchTabBar(next)
			}
			cur = next
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) listNotesSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	notes, err := h.listNotesQuery.Handle(r.Context(), id)
	if err != nil {
		log.Printf("listNotes %s: %v", id, err)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(CustomerNotesFeed(notes))
}

func (h *Handlers) addNoteSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var signals struct {
		NoteBody string `json:"noteBody"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("addNote: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	// Extract author from session cookie if available; best-effort.
	authorID := ""
	if c, err := r.Cookie("session"); err == nil {
		authorID = c.Value
	}

	if _, err := h.addNoteCmd.Handle(r.Context(), commands.AddNoteCommand{
		CustomerID: id,
		Body:       signals.NoteBody,
		AuthorID:   authorID,
	}); err != nil {
		sse.PatchElements(`<div id="note-error" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}

	notes, _ := h.listNotesQuery.Handle(r.Context(), id)
	sse.PatchElements(`<div id="note-error"></div>`)
	sse.PatchSignals([]byte(`{"noteBody":""}`))
	sse.PatchElementTempl(CustomerNotesFeed(notes))
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

// loadZones reads distinct locations from netbox_prefixes via the shared SQLite reader.
// Returns nil when reader is not set.
func (h *Handlers) loadZones(ctx context.Context) []string {
	if h.reader == nil {
		return nil
	}
	var zones []string
	h.reader.WithContext(ctx).Raw(
		"SELECT DISTINCT location FROM netbox_prefixes ORDER BY location",
	).Pluck("location", &zones)
	return zones
}

// loadRouterName returns the router name for the customer's RouterID.
// Returns empty string when reader is not set or customer has no router.
func (h *Handlers) loadRouterName(ctx context.Context, c *domain.Customer) string {
	if h.reader == nil || c.RouterID == nil || *c.RouterID == "" {
		return ""
	}
	var row struct {
		Name string `gorm:"column:name"`
	}
	h.reader.WithContext(ctx).Raw("SELECT name FROM routers WHERE id = ?", *c.RouterID).Scan(&row)
	return row.Name
}
