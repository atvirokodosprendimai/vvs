package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/ticket/app/commands"
	"github.com/vvs/isp/internal/modules/ticket/app/queries"
	"github.com/vvs/isp/internal/modules/ticket/domain"
	"github.com/vvs/isp/internal/shared/audit"
	"github.com/vvs/isp/internal/shared/events"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
)

// CustomerSearchResult holds a customer match for the ticket creation search dropdown.
type CustomerSearchResult struct {
	ID          string
	Code        string
	CompanyName string
}

// CustomerSearcher searches customers by name/code for ticket creation.
type CustomerSearcher interface {
	SearchCustomers(ctx context.Context, query string, limit int) ([]CustomerSearchResult, error)
}

// Handlers wires together all ticket command/query handlers for the SSE layer.
type Handlers struct {
	openCmd         *commands.OpenTicketHandler
	updateCmd       *commands.UpdateTicketHandler
	deleteCmd       *commands.DeleteTicketHandler
	changeStatusCmd *commands.ChangeTicketStatusHandler
	addCommentCmd   *commands.AddCommentHandler
	listQuery       *queries.ListTicketsForCustomerHandler
	listAllQuery    *queries.ListAllTicketsHandler
	getTicketQuery  *queries.GetTicketHandler
	listComments    *queries.ListCommentsHandler
	subscriber      events.EventSubscriber
	publisher       events.EventPublisher
	custSearch      CustomerSearcher
	auditLogger     audit.Logger
}

func NewHandlers(
	openCmd *commands.OpenTicketHandler,
	updateCmd *commands.UpdateTicketHandler,
	deleteCmd *commands.DeleteTicketHandler,
	changeStatusCmd *commands.ChangeTicketStatusHandler,
	addCommentCmd *commands.AddCommentHandler,
	listQuery *queries.ListTicketsForCustomerHandler,
	listComments *queries.ListCommentsHandler,
	subscriber events.EventSubscriber,
	publisher events.EventPublisher,
) *Handlers {
	return &Handlers{
		openCmd:         openCmd,
		updateCmd:       updateCmd,
		deleteCmd:       deleteCmd,
		changeStatusCmd: changeStatusCmd,
		addCommentCmd:   addCommentCmd,
		listQuery:       listQuery,
		listComments:    listComments,
		subscriber:      subscriber,
		publisher:       publisher,
	}
}

// WithListAll adds the list-all-tickets query handler for the standalone page.
func (h *Handlers) WithListAll(q *queries.ListAllTicketsHandler) *Handlers {
	h.listAllQuery = q
	return h
}

// WithGetTicket adds the get-ticket query handler for the detail page.
func (h *Handlers) WithGetTicket(q *queries.GetTicketHandler) *Handlers {
	h.getTicketQuery = q
	return h
}

// WithCustomerSearch adds customer search for the standalone new-ticket modal.
func (h *Handlers) WithAuditLogger(l audit.Logger) *Handlers {
	h.auditLogger = l
	return h
}

func (h *Handlers) audit(r *http.Request, action, resourceID string) {
	if h.auditLogger == nil {
		return
	}
	user := authhttp.UserFromContext(r.Context())
	actorID, actorName := "", ""
	if user != nil {
		actorID = user.ID
		actorName = user.Username
	}
	go func() { _ = h.auditLogger.Log(context.Background(), actorID, actorName, action, "ticket", resourceID, nil) }()
}

func (h *Handlers) WithCustomerSearch(cs CustomerSearcher) *Handlers {
	h.custSearch = cs
	return h
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Customer-scoped routes (existing)
	r.Get("/sse/customers/{id}/tickets", h.listSSE)
	r.Post("/api/customers/{id}/tickets", h.openSSE)
	r.Put("/api/tickets/{ticketID}", h.updateSSE)
	r.Put("/api/tickets/{ticketID}/status", h.changeStatusSSE)
	r.Delete("/api/tickets/{ticketID}", h.deleteSSE)
	r.Post("/api/tickets/{ticketID}/comments", h.addCommentSSE)
	r.Get("/sse/tickets/{ticketID}/comments", h.listCommentsSSE)

	// Standalone ticket pages
	r.Get("/tickets", h.ticketsPage)
	r.Get("/tickets/{ticketID}", h.ticketDetailPage)
	r.Get("/sse/tickets", h.listAllSSE)
	r.Get("/sse/tickets/{ticketID}/detail", h.detailSSE)
	r.Post("/api/tickets", h.createTicket)
	r.Get("/sse/tickets/customers/search", h.ticketCustomerSearch)
}

// listSSE streams the ticket table for a customer, refreshing on any isp.ticket.* event.
func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.TicketAll.String())
	defer cancel()

	q := queries.ListTicketsForCustomerQuery{CustomerID: customerID}

	current, err := h.listQuery.Handle(r.Context(), q)
	if err != nil {
		log.Printf("ticket handler: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(TicketList(customerID, current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listQuery.Handle(r.Context(), q)
			if err != nil {
				log.Printf("ticket handler: listSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(TicketList(customerID, next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// openSSE handles ticket creation from the modal form.
func (h *Handlers) openSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		TicketSubject  string `json:"ticketSubject"`
		TicketBody     string `json:"ticketBody"`
		TicketPriority string `json:"ticketPriority"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("ticket handler: openSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	tk, err := h.openCmd.Handle(r.Context(), commands.OpenTicketCommand{
		CustomerID: customerID,
		Subject:    signals.TicketSubject,
		Body:       signals.TicketBody,
		Priority:   signals.TicketPriority,
	})
	if err != nil {
		log.Printf("ticket handler: openSSE Handle: %v", err)
		sse.PatchElementTempl(ticketFormError(err.Error()))
		return
	}
	h.audit(r, "ticket.opened", tk.ID)

	// Close modal.
	cleared, _ := json.Marshal(map[string]any{"_ticketModalOpen": false, "ticketSubject": "", "ticketBody": ""})
	sse.PatchSignals(cleared)
}

// updateSSE handles ticket subject/body/priority edits.
func (h *Handlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	if ticketID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		TicketSubject  string `json:"ticketSubject"`
		TicketBody     string `json:"ticketBody"`
		TicketPriority string `json:"ticketPriority"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("ticket handler: updateSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	if err := h.updateCmd.Handle(r.Context(), commands.UpdateTicketCommand{
		ID:       ticketID,
		Subject:  signals.TicketSubject,
		Body:     signals.TicketBody,
		Priority: signals.TicketPriority,
	}); err != nil {
		log.Printf("ticket handler: updateSSE Handle: %v", err)
		sse.PatchElementTempl(ticketFormError(err.Error()))
		return
	}
	h.audit(r, "ticket.updated", ticketID)

	cleared, _ := json.Marshal(map[string]any{"_ticketModalOpen": false, "_ticketEditOpen": false, "ticketSubject": "", "ticketBody": ""})
	sse.PatchSignals(cleared)
}

// changeStatusSSE performs a named status transition (start/resolve/close/reopen).
func (h *Handlers) changeStatusSSE(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	if ticketID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		TicketAction string `json:"ticketAction"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("ticket handler: changeStatusSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.changeStatusCmd.Handle(r.Context(), commands.ChangeTicketStatusCommand{
		ID:     ticketID,
		Action: signals.TicketAction,
	}); err != nil {
		log.Printf("ticket handler: changeStatusSSE Handle: %v", err)
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(ticketFormError(err.Error()))
		return
	}
	h.audit(r, "ticket.status_changed", ticketID)
	w.WriteHeader(http.StatusOK)
}

// deleteSSE deletes a ticket.
func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	if ticketID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.deleteCmd.Handle(r.Context(), commands.DeleteTicketCommand{ID: ticketID}); err != nil {
		log.Printf("ticket handler: deleteSSE Handle: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, "ticket.deleted", ticketID)
	w.WriteHeader(http.StatusOK)
}

// addCommentSSE adds a comment to a ticket.
func (h *Handlers) addCommentSSE(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	if ticketID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		CommentBody string `json:"commentBody"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("ticket handler: addCommentSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	if _, err := h.addCommentCmd.Handle(r.Context(), commands.AddCommentCommand{
		TicketID: ticketID,
		Body:     signals.CommentBody,
	}); err != nil {
		log.Printf("ticket handler: addCommentSSE Handle: %v", err)
		sse.PatchElementTempl(ticketFormError(err.Error()))
		return
	}

	// Clear signal and textarea, then re-render the comment section with updated list.
	sse.PatchSignals([]byte(`{"commentBody":""}`))
	sse.PatchElementTempl(commentInput(ticketID))
	if comments, err := h.listComments.Handle(r.Context(), queries.ListCommentsQuery{TicketID: ticketID}); err == nil {
		sse.PatchElementTempl(ticketCommentSection(ticketID, comments))
	}
}

// listCommentsSSE streams the comment list for a ticket, refreshing on comment events.
func (h *Handlers) listCommentsSSE(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	if ticketID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.TicketCommentAdded.String())
	defer cancel()

	q := queries.ListCommentsQuery{TicketID: ticketID}

	current, err := h.listComments.Handle(r.Context(), q)
	if err != nil {
		log.Printf("ticket handler: listCommentsSSE: %v", err)
		return
	}
	sse.PatchElementTempl(CommentList(ticketID, current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listComments.Handle(r.Context(), q)
			if err != nil {
				log.Printf("ticket handler: listCommentsSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(CommentList(ticketID, next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// --- Standalone ticket pages ---

func (h *Handlers) ticketsPage(w http.ResponseWriter, r *http.Request) {
	TicketsPage().Render(r.Context(), w)
}

func (h *Handlers) ticketDetailPage(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	if ticketID == "" || h.getTicketQuery == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	tk, err := h.getTicketQuery.Handle(r.Context(), queries.GetTicketQuery{ID: ticketID})
	if err != nil {
		log.Printf("ticket handler: ticketDetailPage: %v", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	TicketDetailPage(*tk).Render(r.Context(), w)
}

// listAllSSE streams the all-tickets table, refreshing on any isp.ticket.* event.
func (h *Handlers) listAllSSE(w http.ResponseWriter, r *http.Request) {
	if h.listAllQuery == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.TicketAll.String())
	defer cancel()

	var signals struct {
		TicketSearch       string `json:"ticketSearch"`
		TicketStatusFilter string `json:"ticketStatusFilter"`
	}
	_ = datastar.ReadSignals(r, &signals)
	if signals.TicketStatusFilter == "" {
		signals.TicketStatusFilter = "active"
	}
	search := strings.TrimSpace(strings.ToLower(signals.TicketSearch))

	all, err := h.listAllQuery.Handle(r.Context())
	if err != nil {
		log.Printf("ticket handler: listAllSSE: %v", err)
		return
	}
	current := filterTickets(all, signals.TicketStatusFilter, search)
	sse.PatchElementTempl(AllTicketList(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			all, err := h.listAllQuery.Handle(r.Context())
			if err != nil {
				log.Printf("ticket handler: listAllSSE refresh: %v", err)
				continue
			}
			next := filterTickets(all, signals.TicketStatusFilter, search)
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(AllTicketList(next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// detailSSE streams ticket detail partials (status actions + badge), refreshing on isp.ticket.* events.
func (h *Handlers) detailSSE(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	if ticketID == "" || h.getTicketQuery == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.TicketAll.String())
	defer cancel()

	tk, err := h.getTicketQuery.Handle(r.Context(), queries.GetTicketQuery{ID: ticketID})
	if err != nil {
		log.Printf("ticket handler: detailSSE: %v", err)
		return
	}
	sse.PatchElementTempl(ticketDetailHeader(*tk))
	sse.PatchElementTempl(ticketStatusActions(*tk))
	sse.PatchElementTempl(ticketStatusInline(tk.Status))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.getTicketQuery.Handle(r.Context(), queries.GetTicketQuery{ID: ticketID})
			if err != nil {
				log.Printf("ticket handler: detailSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(tk, next) {
				sse.PatchElementTempl(ticketDetailHeader(*next))
				sse.PatchElementTempl(ticketStatusActions(*next))
				sse.PatchElementTempl(ticketStatusInline(next.Status))
				tk = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func filterTickets(tickets []queries.TicketReadModel, statusFilter, search string) []queries.TicketReadModel {
	var out []queries.TicketReadModel
	for _, tk := range tickets {
		terminal := tk.Status == domain.StatusResolved || tk.Status == domain.StatusClosed
		switch statusFilter {
		case "closed":
			if !terminal {
				continue
			}
		default: // "active"
			if terminal {
				continue
			}
		}
		if search != "" {
			if !strings.Contains(strings.ToLower(tk.Subject), search) &&
				!strings.Contains(strings.ToLower(tk.CustomerName), search) &&
				!strings.Contains(strings.ToLower(tk.Status), search) &&
				!strings.Contains(strings.ToLower(tk.Priority), search) {
				continue
			}
		}
		out = append(out, tk)
	}
	return out
}

// createTicket creates a ticket from the standalone page modal (customer from signals).
func (h *Handlers) createTicket(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		CustomerID string `json:"newTicketCustomerID"`
		Subject    string `json:"newTicketSubject"`
		Body       string `json:"newTicketBody"`
		Priority   string `json:"newTicketPriority"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("ticket handler: createTicket ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	if strings.TrimSpace(signals.CustomerID) == "" {
		sse.PatchElementTempl(newTicketError("Please select a customer"))
		return
	}
	if strings.TrimSpace(signals.Subject) == "" {
		sse.PatchElementTempl(newTicketError("Subject is required"))
		return
	}

	tk, err := h.openCmd.Handle(r.Context(), commands.OpenTicketCommand{
		CustomerID: signals.CustomerID,
		Subject:    signals.Subject,
		Body:       signals.Body,
		Priority:   signals.Priority,
	})
	if err != nil {
		log.Printf("ticket handler: createTicket Handle: %v", err)
		sse.PatchElementTempl(newTicketError("Failed to create ticket"))
		return
	}
	h.audit(r, "ticket.opened", tk.ID)

	cleared, _ := json.Marshal(map[string]any{
		"_newTicketOpen":            false,
		"newTicketCustomerID":      "",
		"newTicketCustomerName":    "",
		"newTicketCustomerSearch":  "",
		"newTicketSubject":         "",
		"newTicketBody":            "",
		"newTicketPriority":        "normal",
	})
	sse.PatchSignals(cleared)
}

// ticketCustomerSearch searches customers for the new-ticket modal.
func (h *Handlers) ticketCustomerSearch(w http.ResponseWriter, r *http.Request) {
	if h.custSearch == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}

	var signals struct {
		Search string `json:"newTicketCustomerSearch"`
	}
	_ = datastar.ReadSignals(r, &signals)
	q := strings.TrimSpace(signals.Search)
	if len(q) < 2 {
		return
	}

	results, err := h.custSearch.SearchCustomers(r.Context(), q, 10)
	if err != nil {
		log.Printf("ticket handler: ticketCustomerSearch: %v", err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(ticketCustomerSearchResults(results))
}
