package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	"github.com/vvs/isp/internal/modules/service/app/commands"
	"github.com/vvs/isp/internal/modules/service/app/queries"
	"github.com/vvs/isp/internal/shared/audit"
	"github.com/vvs/isp/internal/shared/events"
)

type ServiceHandlers struct {
	assignCmd      *commands.AssignServiceHandler
	suspendCmd     *commands.SuspendServiceHandler
	reactivateCmd  *commands.ReactivateServiceHandler
	cancelCmd      *commands.CancelServiceHandler
	listQuery      *queries.ListServicesForCustomerHandler
	subscriber     events.EventSubscriber
	publisher      events.EventPublisher
	auditLogger    audit.Logger
}

func NewServiceHandlers(
	assignCmd *commands.AssignServiceHandler,
	suspendCmd *commands.SuspendServiceHandler,
	reactivateCmd *commands.ReactivateServiceHandler,
	cancelCmd *commands.CancelServiceHandler,
	listQuery *queries.ListServicesForCustomerHandler,
	subscriber events.EventSubscriber,
	publisher events.EventPublisher,
) *ServiceHandlers {
	return &ServiceHandlers{
		assignCmd:     assignCmd,
		suspendCmd:    suspendCmd,
		reactivateCmd: reactivateCmd,
		cancelCmd:     cancelCmd,
		listQuery:     listQuery,
		subscriber:    subscriber,
		publisher:     publisher,
	}
}

func (h *ServiceHandlers) WithAuditLogger(l audit.Logger) *ServiceHandlers {
	h.auditLogger = l
	return h
}

func (h *ServiceHandlers) audit(r *http.Request, action, resourceID string) {
	if h.auditLogger == nil {
		return
	}
	user := authhttp.UserFromContext(r.Context())
	actorID, actorName := "", ""
	if user != nil {
		actorID = user.ID
		actorName = user.Username
	}
	go func() { _ = h.auditLogger.Log(context.Background(), actorID, actorName, action, "service", resourceID, nil) }()
}

func (h *ServiceHandlers) RegisterRoutes(r chi.Router) {
	r.Get("/sse/customers/{id}/services", h.listSSE)
	r.Post("/api/customers/{id}/services", h.assignSSE)
	r.Put("/api/services/{serviceID}/suspend", h.suspendSSE)
	r.Put("/api/services/{serviceID}/reactivate", h.reactivateSSE)
	r.Delete("/api/services/{serviceID}", h.cancelSSE)
}

func (h *ServiceHandlers) listSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	// Subscribe before initial render so no event is missed.
	ch, cancel := h.subscriber.ChanSubscription(events.ServiceAll.String())
	defer cancel()

	q := queries.ListServicesForCustomerQuery{CustomerID: customerID}

	current, err := h.listQuery.Handle(r.Context(), q)
	if err != nil {
		log.Printf("service handler: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(ServiceTable(customerID, current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listQuery.Handle(r.Context(), q)
			if err != nil {
				log.Printf("service handler: listSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(ServiceTable(customerID, next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *ServiceHandlers) assignSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		ProductID    string `json:"productid"`
		ProductName  string `json:"productname"`
		PriceAmount  string `json:"priceamount"`
		Currency     string `json:"currency"`
		BillingCycle string `json:"billingcycle"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("service handler: assignSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	priceAmount, err := parsePriceCents(signals.PriceAmount)
	if err != nil {
		log.Printf("service handler: assignSSE parse price: %v", err)
		sse.PatchElementTempl(serviceFormError("Invalid price amount"))
		return
	}

	currency := signals.Currency
	if currency == "" {
		currency = "EUR"
	}

	svc, err := h.assignCmd.Handle(r.Context(), commands.AssignServiceCommand{
		CustomerID:   customerID,
		ProductID:    signals.ProductID,
		ProductName:  signals.ProductName,
		PriceAmount:  priceAmount,
		Currency:     currency,
		StartDate:    time.Now().UTC(),
		BillingCycle: signals.BillingCycle,
	})
	if err != nil {
		log.Printf("service handler: assignSSE Handle: %v", err)
		sse.PatchElementTempl(serviceFormError("internal server error"))
		return
	}
	h.audit(r, "service.assigned", svc.ID)

	cleared, _ := json.Marshal(map[string]any{
		"_assignOpen":  false,
		"productid":    "",
		"productname":  "",
		"priceamount":  "",
		"billingcycle": "",
	})
	sse.PatchSignals(cleared)
}

func (h *ServiceHandlers) suspendSSE(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if serviceID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.suspendCmd.Handle(r.Context(), commands.SuspendServiceCommand{ID: serviceID}); err != nil {
		log.Printf("service handler: suspendSSE Handle: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, "service.suspended", serviceID)
	// Re-render is handled by the listSSE subscription picking up isp.service.suspended.
	w.WriteHeader(http.StatusOK)
}

func (h *ServiceHandlers) reactivateSSE(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if serviceID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.reactivateCmd.Handle(r.Context(), commands.ReactivateServiceCommand{ID: serviceID}); err != nil {
		log.Printf("service handler: reactivateSSE Handle: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, "service.reactivated", serviceID)
	w.WriteHeader(http.StatusOK)
}

func (h *ServiceHandlers) cancelSSE(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	if serviceID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.cancelCmd.Handle(r.Context(), commands.CancelServiceCommand{ID: serviceID}); err != nil {
		log.Printf("service handler: cancelSSE Handle: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, "service.cancelled", serviceID)
	w.WriteHeader(http.StatusOK)
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
