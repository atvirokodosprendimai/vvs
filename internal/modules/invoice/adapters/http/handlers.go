package http

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/invoice/app/commands"
	"github.com/vvs/isp/internal/modules/invoice/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

type Handlers struct {
	createCmd    *commands.CreateInvoiceHandler
	finalizeCmd  *commands.FinalizeInvoiceHandler
	voidCmd      *commands.VoidInvoiceHandler
	listQuery    *queries.ListInvoicesHandler
	getQuery     *queries.GetInvoiceHandler
	subscriber   events.EventSubscriber
}

func NewHandlers(
	createCmd *commands.CreateInvoiceHandler,
	finalizeCmd *commands.FinalizeInvoiceHandler,
	voidCmd *commands.VoidInvoiceHandler,
	listQuery *queries.ListInvoicesHandler,
	getQuery *queries.GetInvoiceHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		createCmd:   createCmd,
		finalizeCmd: finalizeCmd,
		voidCmd:     voidCmd,
		listQuery:   listQuery,
		getQuery:    getQuery,
		subscriber:  subscriber,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/invoices", h.listPage)
	r.Get("/invoices/new", h.createPage)
	r.Get("/invoices/{id}", h.detailPage)
	r.Get("/invoices/{id}/edit", h.editPage)

	r.Get("/api/invoices", h.listSSE)
	r.Post("/api/invoices", h.createSSE)
	r.Put("/api/invoices/{id}/finalize", h.finalizeSSE)
	r.Put("/api/invoices/{id}/void", h.voidSSE)
	r.Delete("/api/invoices/{id}", h.deleteSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	InvoiceListPage().Render(r.Context(), w)
}

func (h *Handlers) createPage(w http.ResponseWriter, r *http.Request) {
	InvoiceFormPage(nil).Render(r.Context(), w)
}

func (h *Handlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	invoice, err := h.getQuery.Handle(r.Context(), queries.GetInvoiceQuery{ID: id})
	if err != nil {
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}
	InvoiceDetailPage(invoice).Render(r.Context(), w)
}

func (h *Handlers) editPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	invoice, err := h.getQuery.Handle(r.Context(), queries.GetInvoiceQuery{ID: id})
	if err != nil {
		http.Error(w, "Invoice not found", http.StatusNotFound)
		return
	}
	InvoiceFormPage(invoice).Render(r.Context(), w)
}

func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	var signals struct {
		Search   string `json:"search"`
		Status   string `json:"status"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		sse.ConsoleError(err)
		return
	}

	if signals.PageSize == 0 {
		signals.PageSize = 25
	}

	result, err := h.listQuery.Handle(r.Context(), queries.ListInvoicesQuery{
		Search:   signals.Search,
		Status:   signals.Status,
		Page:     signals.Page,
		PageSize: signals.PageSize,
	})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.PatchElementTempl(InvoiceTable(result))
}

func (h *Handlers) createSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	var signals struct {
		CustomerID   string `json:"customerId"`
		CustomerName string `json:"customerName"`
		IssueDate    string `json:"issueDate"`
		DueDate      string `json:"dueDate"`
		TaxRate      int    `json:"taxRate"`
		Lines        []struct {
			ProductID   string `json:"productId"`
			ProductName string `json:"productName"`
			Description string `json:"description"`
			Quantity    int    `json:"quantity"`
			UnitPrice   string `json:"unitPrice"`
		} `json:"lines"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		sse.ConsoleError(err)
		return
	}

	issueDate, err := time.Parse("2006-01-02", signals.IssueDate)
	if err != nil {
		sse.PatchElementTempl(formError("Invalid issue date format"))
		return
	}

	dueDate, err := time.Parse("2006-01-02", signals.DueDate)
	if err != nil {
		sse.PatchElementTempl(formError("Invalid due date format"))
		return
	}

	lines := make([]commands.CreateInvoiceLineInput, len(signals.Lines))
	for i, l := range signals.Lines {
		unitPrice, err := parseMoneyInput(l.UnitPrice)
		if err != nil {
			sse.PatchElementTempl(formError("Invalid unit price for line " + strconv.Itoa(i+1)))
			return
		}
		qty := l.Quantity
		if qty <= 0 {
			qty = 1
		}
		lines[i] = commands.CreateInvoiceLineInput{
			ProductID:   l.ProductID,
			ProductName: l.ProductName,
			Description: l.Description,
			Quantity:    qty,
			UnitPrice:   unitPrice,
		}
	}

	taxRate := signals.TaxRate
	if taxRate == 0 {
		taxRate = 21
	}

	_, err = h.createCmd.Handle(r.Context(), commands.CreateInvoiceCommand{
		CustomerID:   signals.CustomerID,
		CustomerName: signals.CustomerName,
		IssueDate:    issueDate,
		DueDate:      dueDate,
		TaxRate:      taxRate,
		Lines:        lines,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/invoices")
}

func (h *Handlers) finalizeSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	err := h.finalizeCmd.Handle(r.Context(), commands.FinalizeInvoiceCommand{ID: id})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.Redirect("/invoices/" + id)
}

func (h *Handlers) voidSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	err := h.voidCmd.Handle(r.Context(), commands.VoidInvoiceCommand{ID: id})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.Redirect("/invoices/" + id)
}

func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")

	invoice, err := h.getQuery.Handle(r.Context(), queries.GetInvoiceQuery{ID: id})
	if err != nil {
		sse.ConsoleError(err)
		return
	}
	if invoice.Status != "draft" {
		sse.ConsoleError(fmt.Errorf("only draft invoices can be deleted"))
		return
	}

	err = h.voidCmd.Handle(r.Context(), commands.VoidInvoiceCommand{ID: id})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.Redirect("/invoices")
}

func parseMoneyInput(s string) (int64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f * 100), nil
}
