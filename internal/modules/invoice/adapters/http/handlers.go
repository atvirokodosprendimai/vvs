package http

import (
	"context"
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
	"github.com/vvs/isp/internal/shared/audit"
	"github.com/vvs/isp/internal/shared/events"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
)

// CustomerSearchResult is returned by CustomerSearcher for the autocomplete dropdown.
type CustomerSearchResult struct {
	ID          string
	Code        string
	CompanyName string
}

// CustomerSearcher is a port for searching customers from the invoice module.
type CustomerSearcher interface {
	SearchCustomers(ctx context.Context, query string, limit int) ([]CustomerSearchResult, error)
}

type Handlers struct {
	createCmd      *commands.CreateInvoiceHandler
	finalizeCmd    *commands.FinalizeInvoiceHandler
	markPaidCmd    *commands.MarkPaidHandler
	voidCmd        *commands.VoidInvoiceHandler
	addLineCmd     *commands.AddLineItemHandler
	updateLineCmd  *commands.UpdateLineItemHandler
	removeLineCmd  *commands.RemoveLineItemHandler
	generateCmd    *commands.GenerateFromSubscriptionsHandler
	listAllQuery   *queries.ListAllInvoicesHandler
	getQuery       *queries.GetInvoiceHandler
	listForCustQ   *queries.ListInvoicesForCustomerHandler
	subscriber     events.EventSubscriber
	custSearch     CustomerSearcher
	defaultVATRate int
	auditLogger    audit.Logger
}

func NewHandlers(
	createCmd *commands.CreateInvoiceHandler,
	finalizeCmd *commands.FinalizeInvoiceHandler,
	markPaidCmd *commands.MarkPaidHandler,
	voidCmd *commands.VoidInvoiceHandler,
	addLineCmd *commands.AddLineItemHandler,
	updateLineCmd *commands.UpdateLineItemHandler,
	removeLineCmd *commands.RemoveLineItemHandler,
	listAllQuery *queries.ListAllInvoicesHandler,
	getQuery *queries.GetInvoiceHandler,
	listForCustQ *queries.ListInvoicesForCustomerHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		createCmd:      createCmd,
		finalizeCmd:    finalizeCmd,
		markPaidCmd:    markPaidCmd,
		voidCmd:        voidCmd,
		addLineCmd:     addLineCmd,
		updateLineCmd:  updateLineCmd,
		removeLineCmd:  removeLineCmd,
		listAllQuery:   listAllQuery,
		getQuery:       getQuery,
		listForCustQ:  listForCustQ,
		subscriber:    subscriber,
	}
}

// WithGenerateCmd adds the generate-from-subscriptions command handler.
func (h *Handlers) WithGenerateCmd(cmd *commands.GenerateFromSubscriptionsHandler) *Handlers {
	h.generateCmd = cmd
	return h
}

// WithCustomerSearch adds the customer search capability.
func (h *Handlers) WithCustomerSearch(cs CustomerSearcher) *Handlers {
	h.custSearch = cs
	return h
}

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
	go func() { _ = h.auditLogger.Log(context.Background(), actorID, actorName, action, "invoice", resourceID, nil) }()
}

// WithDefaultVATRate sets the default VAT rate for new line items.
func (h *Handlers) WithDefaultVATRate(rate int) *Handlers {
	h.defaultVATRate = rate
	return h
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/invoices", h.listPage)
	r.Get("/invoices/new", h.createPage)
	r.Get("/invoices/{id}", h.detailPage)

	r.Get("/sse/invoices", h.listSSE)
	r.Get("/sse/invoices/{id}", h.detailSSE)
	r.Get("/sse/invoices/customers/search", h.customerSearch)

	r.Post("/api/invoices", h.create)
	r.Put("/api/invoices/{id}/finalize", h.finalize)
	r.Put("/api/invoices/{id}/pay", h.markPaid)
	r.Put("/api/invoices/{id}/void", h.voidInvoice)
	r.Post("/api/invoices/{id}/lines", h.addLine)
	r.Put("/api/invoices/{id}/lines/{lineID}", h.updateLine)
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
	vatRate := h.defaultVATRate
	if vatRate <= 0 {
		vatRate = 21
	}
	CreateInvoicePage(vatRate).Render(r.Context(), w)
}

// --- SSE handlers ---

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.InvoiceAll.String())
	defer cancel()

	var signals struct {
		StatusFilter string `json:"statusFilter"`
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
		CustomerID       string `json:"customerID"`
		CustomerName     string `json:"customerName"`
		CustomerCode     string `json:"customerCode"`
		IssueDate        string `json:"issueDate"`
		DueDate          string `json:"dueDate"`
		Notes            string `json:"notes"`
		LineProduct1     string `json:"lineProduct1"`
		LineDescription1 string `json:"lineDescription1"`
		LineQty1         string `json:"lineQty1"`
		LinePrice1       string `json:"linePrice1"`
		LineVATRate1     string `json:"lineVATRate1"`
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

	var lineItems []commands.LineItemInput
	if signals.LineProduct1 != "" || signals.LineDescription1 != "" {
		qty, _ := strconv.Atoi(signals.LineQty1)
		if qty <= 0 {
			qty = 1
		}
		vatRate := parseVATRate(signals.LineVATRate1, h.defaultVATRate)
		lineItems = append(lineItems, commands.LineItemInput{
			ProductName:    signals.LineProduct1,
			Description:    signals.LineDescription1,
			Quantity:       qty,
			UnitPriceGross: parseValueCents(signals.LinePrice1),
			VATRate:        vatRate,
		})
	}

	inv, err := h.createCmd.Handle(r.Context(), commands.CreateInvoiceCommand{
		CustomerID:   signals.CustomerID,
		CustomerName: signals.CustomerName,
		CustomerCode: signals.CustomerCode,
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
	h.audit(r, "invoice.created", inv.ID)

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
	h.audit(r, "invoice.finalized", id)

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
	h.audit(r, "invoice.paid", id)

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
	h.audit(r, "invoice.voided", id)

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
		VATRate     string `json:"lineVATRate"`
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
	vatRate := parseVATRate(signals.VATRate, h.defaultVATRate)

	_, err := h.addLineCmd.Handle(r.Context(), commands.AddLineItemCommand{
		InvoiceID:      id,
		ProductName:    signals.ProductName,
		Description:    signals.Description,
		Quantity:       qty,
		UnitPriceGross: price,
		VATRate:        vatRate,
	})
	if err != nil {
		log.Printf("invoice handler: addLine: %v", err)
		sse.PatchElementTempl(invoiceFormError(err.Error()))
		return
	}

	sse.Redirect("/invoices/" + id)
}

func (h *Handlers) updateLine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	lineID := chi.URLParam(r, "lineID")
	if id == "" || lineID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		ProductName string `json:"editProductName"`
		Description string `json:"editDescription"`
		Quantity    string `json:"editQuantity"`
		UnitPrice   string `json:"editUnitPrice"`
		VATRate     string `json:"editVATRate"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("invoice handler: updateLine ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	qty, _ := strconv.Atoi(signals.Quantity)
	if qty <= 0 {
		qty = 1
	}
	price := parseValueCents(signals.UnitPrice)
	vatRate := parseVATRate(signals.VATRate, h.defaultVATRate)

	_, err := h.updateLineCmd.Handle(r.Context(), commands.UpdateLineItemCommand{
		InvoiceID:      id,
		LineItemID:     lineID,
		ProductName:    signals.ProductName,
		Description:    signals.Description,
		Quantity:       qty,
		UnitPriceGross: price,
		VATRate:        vatRate,
	})
	if err != nil {
		log.Printf("invoice handler: updateLine: %v", err)
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

// parseVATRate parses a VAT rate string. Returns defaultRate if empty/invalid.
func parseVATRate(s string, defaultRate int) int {
	if s == "" {
		return defaultRate
	}
	rate, err := strconv.Atoi(s)
	if err != nil {
		return defaultRate
	}
	return rate
}

// customerSearch returns customer search results as SSE HTML fragments.
func (h *Handlers) customerSearch(w http.ResponseWriter, r *http.Request) {
	if h.custSearch == nil {
		http.Error(w, "not configured", http.StatusNotImplemented)
		return
	}

	var signals struct {
		Search string `json:"customerSearch"`
	}
	_ = datastar.ReadSignals(r, &signals)
	query := strings.TrimSpace(signals.Search)

	sse := datastar.NewSSE(w, r)

	if len(query) < 2 {
		sse.PatchElementTempl(customerSearchResults(nil))
		return
	}

	results, err := h.custSearch.SearchCustomers(r.Context(), query, 10)
	if err != nil {
		log.Printf("invoice handler: customerSearch: %v", err)
		sse.PatchElementTempl(customerSearchResults(nil))
		return
	}

	sse.PatchElementTempl(customerSearchResults(results))
}
