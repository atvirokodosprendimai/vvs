package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/deal/app/commands"
	"github.com/vvs/isp/internal/modules/deal/app/queries"
	"github.com/vvs/isp/internal/modules/deal/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// CustomerNameResolver resolves customer ID → name.
type CustomerNameResolver interface {
	ResolveCustomerName(ctx context.Context, id string) string
}

type Handlers struct {
	addCmd       *commands.AddDealHandler
	updateCmd    *commands.UpdateDealHandler
	deleteCmd    *commands.DeleteDealHandler
	advanceCmd   *commands.AdvanceDealHandler
	listQuery    *queries.ListDealsForCustomerHandler
	listAllQuery *queries.ListAllDealsHandler
	subscriber   events.EventSubscriber
	custNames    CustomerNameResolver
}

func NewHandlers(
	addCmd *commands.AddDealHandler,
	updateCmd *commands.UpdateDealHandler,
	deleteCmd *commands.DeleteDealHandler,
	advanceCmd *commands.AdvanceDealHandler,
	listQuery *queries.ListDealsForCustomerHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		addCmd:     addCmd,
		updateCmd:  updateCmd,
		deleteCmd:  deleteCmd,
		advanceCmd: advanceCmd,
		listQuery:  listQuery,
		subscriber: subscriber,
	}
}

// WithListAll sets the list-all query handler for the standalone /deals page.
func (h *Handlers) WithListAll(q *queries.ListAllDealsHandler) { h.listAllQuery = q }

// WithCustomerNames sets the customer name resolver.
func (h *Handlers) WithCustomerNames(r CustomerNameResolver) { h.custNames = r }

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Customer-scoped deal routes
	r.Get("/sse/customers/{id}/deals", h.listSSE)
	r.Post("/api/customers/{id}/deals", h.addSSE)
	r.Put("/api/deals/{dealID}", h.updateSSE)
	r.Put("/api/deals/{dealID}/advance", h.advanceSSE)
	r.Delete("/api/deals/{dealID}", h.deleteSSE)

	// Standalone deals page
	r.Get("/deals", func(w http.ResponseWriter, r *http.Request) {
		DealsPage().Render(r.Context(), w)
	})
	r.Get("/sse/deals", h.listAllSSE)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	// Subscribe before initial render so no event is missed.
	ch, cancel := h.subscriber.ChanSubscription(events.DealAll.String())
	defer cancel()

	q := queries.ListDealsForCustomerQuery{CustomerID: customerID}

	current, err := h.listQuery.Handle(r.Context(), q)
	if err != nil {
		log.Printf("deal handler: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(DealList(customerID, current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listQuery.Handle(r.Context(), q)
			if err != nil {
				log.Printf("deal handler: listSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(DealList(customerID, next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) addSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		DealTitle    string `json:"dealTitle"`
		DealValue    string `json:"dealValue"`
		DealCurrency string `json:"dealCurrency"`
		DealNotes    string `json:"dealNotes"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("deal handler: addSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	value, err := parseValueCents(signals.DealValue)
	if err != nil {
		log.Printf("deal handler: addSSE parse value: %v", err)
		sse.PatchElementTempl(dealFormError("Invalid deal value"))
		return
	}

	currency := signals.DealCurrency
	if currency == "" {
		currency = "EUR"
	}

	_, err = h.addCmd.Handle(r.Context(), commands.AddDealCommand{
		CustomerID: customerID,
		Title:      signals.DealTitle,
		Value:      value,
		Currency:   currency,
		Notes:      signals.DealNotes,
	})
	if err != nil {
		if errors.Is(err, domain.ErrTitleRequired) {
			sse.PatchElementTempl(dealFormError("Title is required"))
			return
		}
		log.Printf("deal handler: addSSE Handle: %v", err)
		sse.PatchElementTempl(dealFormError("Internal server error"))
		return
	}

	cleared, _ := json.Marshal(map[string]any{
		"_dealModalOpen": false,
		"_dealId":        "",
		"dealTitle":      "",
		"dealValue":      "",
		"dealNotes":      "",
	})
	sse.PatchSignals(cleared)
}

func (h *Handlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	dealID := chi.URLParam(r, "dealID")
	if dealID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		DealTitle    string `json:"dealTitle"`
		DealValue    string `json:"dealValue"`
		DealCurrency string `json:"dealCurrency"`
		DealNotes    string `json:"dealNotes"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("deal handler: updateSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	value, err := parseValueCents(signals.DealValue)
	if err != nil {
		log.Printf("deal handler: updateSSE parse value: %v", err)
		sse.PatchElementTempl(dealFormError("Invalid deal value"))
		return
	}

	currency := signals.DealCurrency
	if currency == "" {
		currency = "EUR"
	}

	err = h.updateCmd.Handle(r.Context(), commands.UpdateDealCommand{
		ID:       dealID,
		Title:    signals.DealTitle,
		Value:    value,
		Currency: currency,
		Notes:    signals.DealNotes,
	})
	if err != nil {
		if errors.Is(err, domain.ErrTitleRequired) {
			sse.PatchElementTempl(dealFormError("Title is required"))
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Printf("deal handler: updateSSE Handle: %v", err)
		sse.PatchElementTempl(dealFormError("Internal server error"))
		return
	}

	sse2 := datastar.NewSSE(w, r)
	cleared, _ := json.Marshal(map[string]any{
		"_dealModalOpen": false,
		"_dealId":        "",
		"dealTitle":      "",
		"dealValue":      "",
		"dealNotes":      "",
	})
	sse2.PatchSignals(cleared)
}

func (h *Handlers) advanceSSE(w http.ResponseWriter, r *http.Request) {
	dealID := chi.URLParam(r, "dealID")
	if dealID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		DealAdvanceAction string `json:"dealAdvanceAction"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("deal handler: advanceSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	err := h.advanceCmd.Handle(r.Context(), commands.AdvanceDealCommand{
		ID:     dealID,
		Action: signals.DealAdvanceAction,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrAlreadyClosed) {
			sse.PatchElementTempl(dealFormError("Deal is already closed"))
			return
		}
		log.Printf("deal handler: advanceSSE Handle: %v", err)
		sse.PatchElementTempl(dealFormError("Internal server error"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	dealID := chi.URLParam(r, "dealID")
	if dealID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.deleteCmd.Handle(r.Context(), commands.DeleteDealCommand{ID: dealID}); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Printf("deal handler: deleteSSE Handle: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) listAllSSE(w http.ResponseWriter, r *http.Request) {
	if h.listAllQuery == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}

	var signals struct {
		Search      string `json:"dealSearch"`
		StageFilter string `json:"dealStageFilter"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("deal handler: listAllSSE ReadSignals: %v", err)
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.DealAll.String())
	defer cancel()

	render := func() {
		all, err := h.listAllQuery.Handle(r.Context(), queries.ListAllDealsQuery{})
		if err != nil {
			log.Printf("deal handler: listAllSSE: %v", err)
			return
		}

		items := h.filterDeals(r.Context(), all, signals.Search, signals.StageFilter)
		sse.PatchElementTempl(DealsPageContent(items, signals.StageFilter))
	}

	render()

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			render()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) filterDeals(ctx context.Context, all []queries.DealReadModel, search, stageFilter string) []DealPageItem {
	var items []DealPageItem
	search = strings.ToLower(strings.TrimSpace(search))

	for _, d := range all {
		// Stage filter
		if stageFilter != "" && stageFilter != "all" && d.Stage != stageFilter {
			continue
		}

		custName := ""
		if h.custNames != nil {
			custName = h.custNames.ResolveCustomerName(ctx, d.CustomerID)
		}

		// Search filter
		if search != "" {
			match := strings.Contains(strings.ToLower(d.Title), search) ||
				strings.Contains(strings.ToLower(custName), search) ||
				strings.Contains(strings.ToLower(d.Notes), search)
			if !match {
				continue
			}
		}

		items = append(items, DealPageItem{
			ID:           d.ID,
			Title:        d.Title,
			Value:        d.Value,
			Currency:     d.Currency,
			Stage:        d.Stage,
			CustomerID:   d.CustomerID,
			CustomerName: custName,
			Notes:        d.Notes,
			CreatedAt:    d.CreatedAt.Format("2006-01-02"),
		})
	}
	return items
}

func parseValueCents(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f * 100), nil
}
