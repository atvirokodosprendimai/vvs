package http

import (
	"encoding/json"
	"log"
	"net/http"
	"reflect"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/atvirokodosprendimai/vvs/internal/modules/contact/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/contact/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	authdomain "github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

type Handlers struct {
	addCmd     *commands.AddContactHandler
	updateCmd  *commands.UpdateContactHandler
	deleteCmd  *commands.DeleteContactHandler
	listQuery  *queries.ListContactsForCustomerHandler
	subscriber events.EventSubscriber
}

func NewHandlers(
	addCmd *commands.AddContactHandler,
	updateCmd *commands.UpdateContactHandler,
	deleteCmd *commands.DeleteContactHandler,
	listQuery *queries.ListContactsForCustomerHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		addCmd:     addCmd,
		updateCmd:  updateCmd,
		deleteCmd:  deleteCmd,
		listQuery:  listQuery,
		subscriber: subscriber,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/sse/customers/{id}/contacts", h.listSSE)
	r.Post("/api/customers/{id}/contacts", h.addSSE)
	r.Put("/api/contacts/{contactID}", h.updateSSE)
	r.Delete("/api/contacts/{contactID}", h.deleteSSE)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.ContactAll.String())
	defer cancel()

	q := queries.ListContactsForCustomerQuery{CustomerID: customerID}
	current, err := h.listQuery.Handle(r.Context(), q)
	if err != nil {
		log.Printf("contact handler: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(ContactList(customerID, current))

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
				sse.PatchElementTempl(ContactList(customerID, next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) addSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")

	var signals struct {
		FirstName string `json:"contactFirstName"`
		LastName  string `json:"contactLastName"`
		Email     string `json:"contactEmail"`
		Phone     string `json:"contactPhone"`
		Role      string `json:"contactRole"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("contact handler: addSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	if _, err := h.addCmd.Handle(r.Context(), commands.AddContactCommand{
		CustomerID: customerID,
		FirstName:  signals.FirstName,
		LastName:   signals.LastName,
		Email:      signals.Email,
		Phone:      signals.Phone,
		Role:       signals.Role,
	}); err != nil {
		sse.PatchElements(`<div id="contact-form-error" class="text-red-400 text-xs mt-1">` + err.Error() + `</div>`)
		return
	}

	// Close modal + clear fields
	cleared, _ := json.Marshal(map[string]any{
		"_contactModalOpen": false,
		"_contactId":        "",
		"contactFirstName":  "",
		"contactLastName":   "",
		"contactEmail":      "",
		"contactPhone":      "",
		"contactRole":       "",
		"contactNotes":      "",
	})
	sse.PatchSignals(cleared)
}

func (h *Handlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	contactID := chi.URLParam(r, "contactID")

	var signals struct {
		FirstName string `json:"contactFirstName"`
		LastName  string `json:"contactLastName"`
		Email     string `json:"contactEmail"`
		Phone     string `json:"contactPhone"`
		Role      string `json:"contactRole"`
		Notes     string `json:"contactNotes"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("contact handler: updateSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	if err := h.updateCmd.Handle(r.Context(), commands.UpdateContactCommand{
		ID:        contactID,
		FirstName: signals.FirstName,
		LastName:  signals.LastName,
		Email:     signals.Email,
		Phone:     signals.Phone,
		Role:      signals.Role,
		Notes:     signals.Notes,
	}); err != nil {
		sse.PatchElements(`<div id="contact-form-error" class="text-red-400 text-xs mt-1">` + err.Error() + `</div>`)
		return
	}

	cleared, _ := json.Marshal(map[string]any{
		"_contactModalOpen": false,
		"_contactId":        "",
		"contactFirstName":  "",
		"contactLastName":   "",
		"contactEmail":      "",
		"contactPhone":      "",
		"contactRole":       "",
		"contactNotes":      "",
	})
	sse.PatchSignals(cleared)
}

func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	contactID := chi.URLParam(r, "contactID")

	if err := h.deleteCmd.Handle(r.Context(), commands.DeleteContactCommand{ID: contactID}); err != nil {
		log.Printf("contact handler: deleteSSE: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ModuleName() authdomain.Module { return authdomain.ModuleContacts }
