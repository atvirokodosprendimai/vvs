package http

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/audit_log/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

func (h *Handlers) customerAuditLogsSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if h.listForResource == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}
	logs, err := h.listForResource.Handle(r.Context(), queries.ListForResourceQuery{
		Resource:   "customer",
		ResourceID: customerID,
	})
	if err != nil {
		log.Printf("audit_log handler: customerAuditLogsSSE: %v", err)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(AuditLogList(logs))
}

// Handlers wires together all audit log query handlers for the HTTP layer.
type Handlers struct {
	listQuery           *queries.ListAuditLogsHandler
	listForResource     *queries.ListForResourceHandler
	subscriber          events.EventSubscriber
}

func NewHandlers(listQuery *queries.ListAuditLogsHandler, subscriber events.EventSubscriber) *Handlers {
	return &Handlers{
		listQuery:  listQuery,
		subscriber: subscriber,
	}
}

func (h *Handlers) WithListForResource(q *queries.ListForResourceHandler) *Handlers {
	h.listForResource = q
	return h
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/audit-logs", h.auditLogsPage)
	r.Get("/sse/audit-logs", h.listSSE)
	r.Get("/sse/customers/{id}/audit-logs", h.customerAuditLogsSSE)
}

func (h *Handlers) auditLogsPage(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	actor := r.URL.Query().Get("actor")

	logs, err := h.listQuery.Handle(r.Context(), queries.ListAuditLogsQuery{
		ActorID:  actor,
		Resource: resource,
	})
	if err != nil {
		log.Printf("audit_log handler: auditLogsPage: %v", err)
		logs = nil
	}
	AuditLogsPage(logs).Render(r.Context(), w)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		AuditResourceFilter string `json:"auditResourceFilter"`
	}
	_ = datastar.ReadSignals(r, &signals)

	q := queries.ListAuditLogsQuery{
		Resource: signals.AuditResourceFilter,
	}

	logs, err := h.listQuery.Handle(r.Context(), q)
	if err != nil {
		log.Printf("audit_log handler: listSSE: %v", err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(AuditLogList(logs))
}
