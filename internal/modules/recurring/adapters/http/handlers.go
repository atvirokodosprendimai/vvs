package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/recurring/app/commands"
	"github.com/vvs/isp/internal/modules/recurring/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

type Handlers struct {
	createCmd  *commands.CreateRecurringHandler
	updateCmd  *commands.UpdateRecurringHandler
	toggleCmd  *commands.ToggleRecurringHandler
	listQuery  *queries.ListRecurringHandler
	getQuery   *queries.GetRecurringHandler
	subscriber events.EventSubscriber
}

func NewHandlers(
	createCmd *commands.CreateRecurringHandler,
	updateCmd *commands.UpdateRecurringHandler,
	toggleCmd *commands.ToggleRecurringHandler,
	listQuery *queries.ListRecurringHandler,
	getQuery *queries.GetRecurringHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		createCmd:  createCmd,
		updateCmd:  updateCmd,
		toggleCmd:  toggleCmd,
		listQuery:  listQuery,
		getQuery:   getQuery,
		subscriber: subscriber,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/recurring", h.listPage)
	r.Get("/recurring/new", h.createPage)
	r.Get("/recurring/{id}", h.detailPage)
	r.Get("/recurring/{id}/edit", h.editPage)

	r.Get("/api/recurring", h.listSSE)
	r.Post("/api/recurring", h.createSSE)
	r.Put("/api/recurring/{id}", h.updateSSE)
	r.Put("/api/recurring/{id}/toggle", h.toggleSSE)
	r.Delete("/api/recurring/{id}", h.deleteSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	RecurringListPage().Render(r.Context(), w)
}

func (h *Handlers) createPage(w http.ResponseWriter, r *http.Request) {
	RecurringFormPage(nil).Render(r.Context(), w)
}

func (h *Handlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	invoice, err := h.getQuery.Handle(r.Context(), queries.GetRecurringQuery{ID: id})
	if err != nil {
		http.Error(w, "Recurring invoice not found", http.StatusNotFound)
		return
	}
	RecurringDetailPage(invoice).Render(r.Context(), w)
}

func (h *Handlers) editPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	invoice, err := h.getQuery.Handle(r.Context(), queries.GetRecurringQuery{ID: id})
	if err != nil {
		http.Error(w, "Recurring invoice not found", http.StatusNotFound)
		return
	}
	RecurringFormPage(invoice).Render(r.Context(), w)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Search   string `json:"search"`
		Status   string `json:"status"`
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

	result, err := h.listQuery.Handle(r.Context(), queries.ListRecurringQuery{
		Search:   signals.Search,
		Status:   signals.Status,
		Page:     signals.Page,
		PageSize: signals.PageSize,
	})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.PatchElementTempl(RecurringTable(result))
}

func (h *Handlers) createSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		CustomerID   string `json:"customerID"`
		CustomerName string `json:"customerName"`
		Frequency    string `json:"frequency"`
		DayOfMonth   string `json:"dayOfMonth"`
		// Lines as parallel arrays
		LineProductIDs   []string `json:"lineProductIDs"`
		LineProductNames []string `json:"lineProductNames"`
		LineDescriptions []string `json:"lineDescriptions"`
		LineQuantities   []string `json:"lineQuantities"`
		LineUnitPrices   []string `json:"lineUnitPrices"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	dayOfMonth, _ := strconv.Atoi(signals.DayOfMonth)

	var lines []commands.RecurringLineInput
	for i := 0; i < len(signals.LineProductNames); i++ {
		qty, _ := strconv.Atoi(signals.LineQuantities[i])
		if qty < 1 {
			qty = 1
		}
		price := parseAmountCents(signals.LineUnitPrices[i])
		productID := ""
		if i < len(signals.LineProductIDs) {
			productID = signals.LineProductIDs[i]
		}
		desc := ""
		if i < len(signals.LineDescriptions) {
			desc = signals.LineDescriptions[i]
		}
		lines = append(lines, commands.RecurringLineInput{
			ProductID:   productID,
			ProductName: signals.LineProductNames[i],
			Description: desc,
			Quantity:    qty,
			UnitPrice:   price,
			Currency:    "EUR",
		})
	}

	_, err := h.createCmd.Handle(r.Context(), commands.CreateRecurringCommand{
		CustomerID:   signals.CustomerID,
		CustomerName: signals.CustomerName,
		Frequency:    signals.Frequency,
		DayOfMonth:   dayOfMonth,
		Lines:        lines,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/recurring")
}

func (h *Handlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var signals struct {
		CustomerID   string `json:"customerID"`
		CustomerName string `json:"customerName"`
		Frequency    string `json:"frequency"`
		DayOfMonth   string `json:"dayOfMonth"`
		// Lines as parallel arrays
		LineProductIDs   []string `json:"lineProductIDs"`
		LineProductNames []string `json:"lineProductNames"`
		LineDescriptions []string `json:"lineDescriptions"`
		LineQuantities   []string `json:"lineQuantities"`
		LineUnitPrices   []string `json:"lineUnitPrices"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	dayOfMonth, _ := strconv.Atoi(signals.DayOfMonth)

	var lines []commands.RecurringLineInput
	for i := 0; i < len(signals.LineProductNames); i++ {
		qty, _ := strconv.Atoi(signals.LineQuantities[i])
		if qty < 1 {
			qty = 1
		}
		price := parseAmountCents(signals.LineUnitPrices[i])
		productID := ""
		if i < len(signals.LineProductIDs) {
			productID = signals.LineProductIDs[i]
		}
		desc := ""
		if i < len(signals.LineDescriptions) {
			desc = signals.LineDescriptions[i]
		}
		lines = append(lines, commands.RecurringLineInput{
			ProductID:   productID,
			ProductName: signals.LineProductNames[i],
			Description: desc,
			Quantity:    qty,
			UnitPrice:   price,
			Currency:    "EUR",
		})
	}

	err := h.updateCmd.Handle(r.Context(), commands.UpdateRecurringCommand{
		ID:           id,
		CustomerID:   signals.CustomerID,
		CustomerName: signals.CustomerName,
		Frequency:    signals.Frequency,
		DayOfMonth:   dayOfMonth,
		Lines:        lines,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/recurring/" + id)
}

func (h *Handlers) toggleSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	err := h.toggleCmd.Handle(r.Context(), commands.ToggleRecurringCommand{ID: id})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.Redirect("/recurring/" + id)
}

func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	_, err := h.getQuery.Handle(r.Context(), queries.GetRecurringQuery{ID: id})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.Redirect("/recurring")
}

func parseAmountCents(s string) int64 {
	n, _ := strconv.ParseFloat(s, 64)
	return int64(n * 100)
}
