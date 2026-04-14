package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/debt/app/commands"
	"github.com/vvs/isp/internal/modules/debt/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

// eventNotifier is implemented by *infranats.Subscriber.
// Returns a channel of events and a cancel func (caller must call cancel to unsubscribe).
type eventNotifier interface {
	ChanSubscription(subject string) (<-chan events.DomainEvent, func())
}

type Handlers struct {
	syncCmd   *commands.SyncDebtorsHandler
	listQuery *queries.ListDebtStatusesHandler
	notifier  eventNotifier
}

func NewHandlers(
	syncCmd *commands.SyncDebtorsHandler,
	listQuery *queries.ListDebtStatusesHandler,
	notifier eventNotifier,
) *Handlers {
	return &Handlers{
		syncCmd:   syncCmd,
		listQuery: listQuery,
		notifier:  notifier,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/debt", h.listPage)
	r.Get("/api/debt", h.listSSE)
	r.Post("/api/debt/sync", h.syncSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	DebtListPage().Render(r.Context(), w)
}

// listSSE is a long-lived SSE handler. It renders the debt table on connect
// and re-renders it every time a debt.synced NATS event arrives.
func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	rows, err := h.listQuery.Handle(r.Context())
	if err != nil {
		sse.ConsoleError(err)
		return
	}
	sse.PatchElementTempl(DebtTable(rows))

	ch, cancel := h.notifier.ChanSubscription("isp.debt.synced")
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			if sse.IsClosed() {
				return
			}
			rows, err = h.listQuery.Handle(r.Context())
			if err != nil {
				sse.ConsoleError(err)
				continue
			}
			sse.PatchElementTempl(DebtTable(rows))
		}
	}
}

// syncSSE triggers a full sync from itax.lt. The NATS event published by the
// command will cause all open listSSE connections to re-render automatically.
func (h *Handlers) syncSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	if err := h.syncCmd.Handle(r.Context(), commands.SyncDebtorsCommand{}); err != nil {
		sse.ConsoleError(err)
		return
	}
}
