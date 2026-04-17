package http

import (
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/invoice/app/commands"
	"github.com/vvs/isp/internal/modules/invoice/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

type Handlers struct {
	createCmd     *commands.CreateInvoiceHandler
	finalizeCmd   *commands.FinalizeInvoiceHandler
	markPaidCmd   *commands.MarkPaidHandler
	voidCmd       *commands.VoidInvoiceHandler
	addLineCmd    *commands.AddLineItemHandler
	removeLineCmd *commands.RemoveLineItemHandler
	generateCmd   *commands.GenerateFromSubscriptionsHandler
	listAllQuery  *queries.ListAllInvoicesHandler
	getQuery      *queries.GetInvoiceHandler
	listForCustQ  *queries.ListInvoicesForCustomerHandler
	subscriber    events.EventSubscriber
}

func NewHandlers(
	createCmd *commands.CreateInvoiceHandler,
	finalizeCmd *commands.FinalizeInvoiceHandler,
	markPaidCmd *commands.MarkPaidHandler,
	voidCmd *commands.VoidInvoiceHandler,
	addLineCmd *commands.AddLineItemHandler,
	removeLineCmd *commands.RemoveLineItemHandler,
	listAllQuery *queries.ListAllInvoicesHandler,
	getQuery *queries.GetInvoiceHandler,
	listForCustQ *queries.ListInvoicesForCustomerHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		createCmd:     createCmd,
		finalizeCmd:   finalizeCmd,
		markPaidCmd:   markPaidCmd,
		voidCmd:       voidCmd,
		addLineCmd:    addLineCmd,
		removeLineCmd: removeLineCmd,
		listAllQuery:  listAllQuery,
		getQuery:      getQuery,
		listForCustQ:  listForCustQ,
		subscriber:    subscriber,
	}
}

// WithGenerateCmd adds the generate-from-subscriptions command handler.
func (h *Handlers) WithGenerateCmd(cmd *commands.GenerateFromSubscriptionsHandler) *Handlers {
	h.generateCmd = cmd
	return h
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/invoices", h.listPage)
	r.Get("/invoices/new", h.createPage)
	r.Get("/invoices/{id}", h.detailPage)

	r.Get("/sse/invoices", h.listSSE)
	r.Get("/sse/invoices/{id}", h.detailSSE)

	r.Post("/api/invoices", h.create)
	r.Put("/api/invoices/{id}/finalize", h.finalize)
	r.Put("/api/invoices/{id}/pay", h.markPaid)
	r.Put("/api/invoices/{id}/void", h.voidInvoice)
	r.Post("/api/invoices/{id}/lines", h.addLine)
	r.Delete("/api/invoices/{id}/lines/{lineID}", h.removeLine)
}

// --- Page handlers ---

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	InvoiceListPage().Render(r.Context(), w)
}

func (h *Handlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	inv, err := h.getQuery.Handle(r.Context(), id)
	if err != nil {
		log.Printf("invoice handler: detailPage: %v", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	InvoiceDetailPage(*inv).Render(r.Context(), w)
}

func (h *Handlers) createPage(w http.ResponseWriter, r *http.Request) {
	CreateInvoicePage().Render(r.Context(), w)
}

// --- SSE handlers ---

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.InvoiceAll.String())
	defer cancel()

	var signals struct {
		StatusFilter string `json:"_statusFilter"`
	}
	_ = datastar.ReadSignals(r, &signals)
	statusFilter := strings.TrimSpace(signals.StatusFilter)

	current, err := h.listAllQuery.Handle(r.Context(), queries.ListAllInvoicesQuery{Status: statusFilter})
	if err != nil {
		log.Printf("invoice handler: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(InvoiceTable(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listAllQuery.Handle(r.Context(), queries.ListAllInvoicesQuery{Status: statusFilter})
			if err != nil {
				log.Printf("invoice handler: listSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(InvoiceTable(next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) detailSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.InvoiceAll.String())
	defer cancel()

	inv, err := h.getQuery.Handle(r.Context(), id)
	if err != nil {
		log.Printf("invoice handler: detailSSE: %v", err)
		return
	}
	sse.PatchElementTempl(invoiceDetailContent(*inv))
	sse.PatchElementTempl(invoiceStatusActions(*inv))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.getQuery.Handle(r.Context(), id)
			if err != nil {
				log.Printf("invoice handler: detailSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(inv, next) {
				sse.PatchElementTempl(invoiceDetailContent(*next))
				sse.PatchElementTempl(invoiceStatusActions(*next))
				inv = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// --- Command handlers ---

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		CustomerID   string `json:"customerID"`
		CustomerName string `json:"customerName"`
		IssueDate    string `json:"issueDate"`
		DueDate      string `json:"dueDate"`
		Notes        string `json:"notes"`
		// Line items — simple static rows
		LineProduct1     string `json:"lineProduct1"`
		LineDescription1 string `json:"lineDescription1"`
		LineQty1         string `json:"lineQty1"`
		LinePrice1       string `json:"linePrice1"`
		LineProduct2     string `json:"lineProduct2"`
		LineDescription2 string `json:"lineDescription2"`
		LineQty2         string `json:"lineQty2"`
		LinePrice2       string `json:"linePrice2"`
		LineProduct3     string `json:"lineProduct3"`
		LineDescription3 string `json:"lineDescription3"`
		LineQty3         string `json:"lineQty3"`
		LinePrice3       string `json:"linePrice3"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("invoice handler: create ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	if signals.CustomerID == "" || signals.CustomerName == "" {
		sse.PatchElementTempl(invoiceFormError("Customer is required"))
		return
	}

	issueDate, err := time.Parse("2006-01-02", signals.IssueDate)
	if err != nil {
		issueDate = time.Now().UTC()
	}
	dueDate, err := time.Parse("2006-01-02", signals.DueDate)
	if err != nil {
		dueDate = issueDate.AddDate(0, 0, 30)
	}

	// Collect line items from static rows
	var lineItems []commands.LineItemInput
	type lineRow struct {
		product     string
		description string
		qty         string
		price       string
	}
	rows := []lineRow{
		{signals.LineProduct1, signals.LineDescription1, signals.LineQty1, signals.LinePrice1},
		{signals.LineProduct2, signals.LineDescription2, signals.LineQty2, signals.LinePrice2},
		{signals.LineProduct3, signals.LineDescription3, signals.LineQty3, signals.LinePrice3},
	}
	for _, row := range rows {
		if row.description == "" && row.product == "" {
			continue
		}
		qty, _ := strconv.Atoi(row.qty)
		if qty <= 0 {
			qty = 1
		}
		price := parseValueCents(row.price)
		lineItems = append(lineItems, commands.LineItemInput{
			ProductName: row.product,
			Description: row.description,
			Quantity:    qty,
			UnitPrice:   price,
		})
	}

	inv, err := h.createCmd.Handle(r.Context(), commands.CreateInvoiceCommand{
		CustomerID:   signals.CustomerID,
		CustomerName: signals.CustomerName,
		IssueDate:    issueDate,
		DueDate:      dueDate,
		Notes:        signals.Notes,
		LineItems:    lineItems,
	})
	if err != nil {
		log.Printf("invoice handler: create Handle: %v", err)
		sse.PatchElementTempl(invoiceFormError(err.Error()))
		return
	}

	sse.Redirect("/invoices/" + inv.ID)
}

func (h *Handlers) finalize(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	_, err := h.finalizeCmd.Handle(r.Context(), commands.FinalizeInvoiceCommand{InvoiceID: id})
	if err != nil {
		log.Printf("invoice handler: finalize: %v", err)
		sse.PatchElementTempl(invoiceFormError(err.Error()))
		return
	}

	sse.Redirect("/invoices/" + id)
}

func (h *Handlers) markPaid(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	_, err := h.markPaidCmd.Handle(r.Context(), commands.MarkPaidCommand{InvoiceID: id})
	if err != nil {
		log.Printf("invoice handler: markPaid: %v", err)
		sse.PatchElementTempl(invoiceFormError(err.Error()))
		return
	}

	sse.Redirect("/invoices/" + id)
}

func (h *Handlers) voidInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	_, err := h.voidCmd.Handle(r.Context(), commands.VoidInvoiceCommand{InvoiceID: id})
	if err != nil {
		log.Printf("invoice handler: void: %v", err)
		sse.PatchElementTempl(invoiceFormError(err.Error()))
		return
	}

	sse.Redirect("/invoices/" + id)
}

func (h *Handlers) addLine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		ProductName string `json:"lineProductName"`
		Description string `json:"lineDescription"`
		Quantity    string `json:"lineQuantity"`
		UnitPrice   string `json:"lineUnitPrice"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("invoice handler: addLine ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	qty, _ := strconv.Atoi(signals.Quantity)
	if qty <= 0 {
		qty = 1
	}
	price := parseValueCents(signals.UnitPrice)

	_, err := h.addLineCmd.Handle(r.Context(), commands.AddLineItemCommand{
		InvoiceID:   id,
		ProductName: signals.ProductName,
		Description: signals.Description,
		Quantity:    qty,
		UnitPrice:   price,
	})
	if err != nil {
		log.Printf("invoice handler: addLine: %v", err)
		sse.PatchElementTempl(invoiceFormError(err.Error()))
		return
	}

	sse.Redirect("/invoices/" + id)
}

func (h *Handlers) removeLine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	lineID := chi.URLParam(r, "lineID")
	if id == "" || lineID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	_, err := h.removeLineCmd.Handle(r.Context(), commands.RemoveLineItemCommand{
		InvoiceID:  id,
		LineItemID: lineID,
	})
	if err != nil {
		log.Printf("invoice handler: removeLine: %v", err)
		sse.PatchElementTempl(invoiceFormError(err.Error()))
		return
	}

	sse.Redirect("/invoices/" + id)
}

// parseValueCents parses a decimal string like "12.50" into cents (1250).
func parseValueCents(s string) int64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(f * 100)
}
