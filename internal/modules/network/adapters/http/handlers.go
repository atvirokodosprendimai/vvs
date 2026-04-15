package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/network/app/commands"
	"github.com/vvs/isp/internal/modules/network/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

type Handlers struct {
	createCmd  *commands.CreateRouterHandler
	updateCmd  *commands.UpdateRouterHandler
	deleteCmd  *commands.DeleteRouterHandler
	listQuery  *queries.ListRoutersHandler
	getQuery   *queries.GetRouterHandler
	subscriber events.EventSubscriber
}

func NewHandlers(
	createCmd *commands.CreateRouterHandler,
	updateCmd *commands.UpdateRouterHandler,
	deleteCmd *commands.DeleteRouterHandler,
	listQuery *queries.ListRoutersHandler,
	getQuery *queries.GetRouterHandler,
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
	r.Get("/routers", h.listPage)
	r.Get("/routers/new", h.createPage)
	r.Get("/routers/{id}", h.detailPage)
	r.Get("/routers/{id}/edit", h.editPage)

	r.Get("/api/routers", h.listSSE)
	r.Post("/api/routers", h.createSSE)
	r.Put("/api/routers/{id}", h.updateSSE)
	r.Delete("/api/routers/{id}", h.deleteSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	RouterListPage().Render(r.Context(), w)
}

func (h *Handlers) createPage(w http.ResponseWriter, r *http.Request) {
	RouterFormPage(nil).Render(r.Context(), w)
}

func (h *Handlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	router, err := h.getQuery.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "Router not found", http.StatusNotFound)
		return
	}
	RouterDetailPage(router).Render(r.Context(), w)
}

func (h *Handlers) editPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	router, err := h.getQuery.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "Router not found", http.StatusNotFound)
		return
	}
	RouterFormPage(&router).Render(r.Context(), w)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription("isp.network.router.*")
	defer cancel()

	routers, err := h.listQuery.Handle(r.Context())
	if err != nil {
		sse.ConsoleError(err)
		return
	}
	sse.PatchElementTempl(RouterTable(routers))

	// Live updates: re-render full table on any event — Datastar morph handles add/update/delete
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			routers, err = h.listQuery.Handle(r.Context())
			if err != nil {
				continue
			}
			sse.PatchElementTempl(RouterTable(routers))
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) createSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Name       string `json:"name"`
		RouterType string `json:"router_type"`
		Host       string `json:"host"`
		Port       string `json:"port"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		Notes      string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	port := parsePort(signals.Port, signals.RouterType)

	_, err := h.createCmd.Handle(r.Context(), commands.CreateRouterCommand{
		Name:       signals.Name,
		RouterType: signals.RouterType,
		Host:       signals.Host,
		Port:       port,
		Username:   signals.Username,
		Password:   signals.Password,
		Notes:      signals.Notes,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}
	sse.Redirect("/routers")
}

func (h *Handlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var signals struct {
		Name       string `json:"name"`
		RouterType string `json:"router_type"`
		Host       string `json:"host"`
		Port       string `json:"port"`
		Username   string `json:"username"`
		Password   string `json:"password"` // empty = keep existing
		Notes      string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	port := parsePort(signals.Port, signals.RouterType)

	_, err := h.updateCmd.Handle(r.Context(), commands.UpdateRouterCommand{
		ID:         id,
		Name:       signals.Name,
		RouterType: signals.RouterType,
		Host:       signals.Host,
		Port:       port,
		Username:   signals.Username,
		Password:   signals.Password,
		Notes:      signals.Notes,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}
	sse.Redirect("/routers/" + id)
}

func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	if err := h.deleteCmd.Handle(r.Context(), id); err != nil {
		sse.ConsoleError(err)
		return
	}
	sse.Redirect("/routers")
}

func parsePort(s, routerType string) int {
	if s != "" {
		if p, err := strconv.Atoi(s); err == nil && p > 0 {
			return p
		}
	}
	if routerType == "arista" {
		return 443
	}
	return 8728
}
