package http

import (
	"encoding/json"
	"log"
	"net/http"
	"reflect"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/ticket/app/commands"
	"github.com/vvs/isp/internal/modules/ticket/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

// Handlers wires together all ticket command/query handlers for the SSE layer.
type Handlers struct {
	openCmd         *commands.OpenTicketHandler
	updateCmd       *commands.UpdateTicketHandler
	deleteCmd       *commands.DeleteTicketHandler
	changeStatusCmd *commands.ChangeTicketStatusHandler
	addCommentCmd   *commands.AddCommentHandler
	listQuery       *queries.ListTicketsForCustomerHandler
	listComments    *queries.ListCommentsHandler
	subscriber      events.EventSubscriber
	publisher       events.EventPublisher
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

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/sse/customers/{id}/tickets", h.listSSE)
	r.Post("/api/customers/{id}/tickets", h.openSSE)
	r.Put("/api/tickets/{ticketID}", h.updateSSE)
	r.Put("/api/tickets/{ticketID}/status", h.changeStatusSSE)
	r.Delete("/api/tickets/{ticketID}", h.deleteSSE)
	r.Post("/api/tickets/{ticketID}/comments", h.addCommentSSE)
	r.Get("/sse/tickets/{ticketID}/comments", h.listCommentsSSE)
}

// listSSE streams the ticket table for a customer, refreshing on any isp.ticket.* event.
func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription("isp.ticket.*")
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

	_, err := h.openCmd.Handle(r.Context(), commands.OpenTicketCommand{
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

	cleared, _ := json.Marshal(map[string]any{"_ticketModalOpen": false, "ticketSubject": "", "ticketBody": ""})
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

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

	// Clear the comment textarea signal.
	cleared, _ := json.Marshal(map[string]any{"commentBody": ""})
	sse.PatchSignals(cleared)
}

// listCommentsSSE streams the comment list for a ticket, refreshing on comment events.
func (h *Handlers) listCommentsSSE(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "ticketID")
	if ticketID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription("isp.ticket.comment_added")
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
