package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/payment/app/commands"
	"github.com/vvs/isp/internal/modules/payment/app/queries"
	"github.com/vvs/isp/internal/shared/events"
)

type Handlers struct {
	recordCmd  *commands.RecordPaymentHandler
	importCmd  *commands.ImportPaymentsHandler
	matchCmd   *commands.MatchPaymentHandler
	listQuery  *queries.ListPaymentsHandler
	getQuery   *queries.GetPaymentHandler
	unmatchedQ *queries.UnmatchedPaymentsHandler
	subscriber events.EventSubscriber
}

func NewHandlers(
	recordCmd *commands.RecordPaymentHandler,
	importCmd *commands.ImportPaymentsHandler,
	matchCmd *commands.MatchPaymentHandler,
	listQuery *queries.ListPaymentsHandler,
	getQuery *queries.GetPaymentHandler,
	unmatchedQ *queries.UnmatchedPaymentsHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		recordCmd:  recordCmd,
		importCmd:  importCmd,
		matchCmd:   matchCmd,
		listQuery:  listQuery,
		getQuery:   getQuery,
		unmatchedQ: unmatchedQ,
		subscriber: subscriber,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/payments", h.listPage)
	r.Get("/payments/import", h.importPage)
	r.Get("/payments/{id}", h.detailPage)

	r.Get("/api/payments", h.listSSE)
	r.Post("/api/payments", h.createSSE)
	r.Post("/api/payments/import", h.importSSE)
	r.Put("/api/payments/{id}/match", h.matchSSE)
	r.Get("/api/payments/unmatched", h.unmatchedSSE)
}

func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	PaymentListPage().Render(r.Context(), w)
}

func (h *Handlers) importPage(w http.ResponseWriter, r *http.Request) {
	PaymentImportPage().Render(r.Context(), w)
}

func (h *Handlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	payment, err := h.getQuery.Handle(r.Context(), queries.GetPaymentQuery{ID: id})
	if err != nil {
		http.Error(w, "Payment not found", http.StatusNotFound)
		return
	}
	PaymentDetailPage(payment).Render(r.Context(), w)
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

	result, err := h.listQuery.Handle(r.Context(), queries.ListPaymentsQuery{
		Search:   signals.Search,
		Status:   signals.Status,
		Page:     signals.Page,
		PageSize: signals.PageSize,
	})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.PatchElementTempl(PaymentTable(result))
}

func (h *Handlers) createSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Amount      string `json:"amount"`
		Reference   string `json:"reference"`
		PayerName   string `json:"payerName"`
		PayerIBAN   string `json:"payerIBAN"`
		BookingDate string `json:"bookingDate"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	amountCents := parseAmountCents(signals.Amount)

	bookingDate, err := time.Parse("2006-01-02", signals.BookingDate)
	if err != nil {
		sse.PatchElementTempl(formError("Invalid booking date format (use YYYY-MM-DD)"))
		return
	}

	_, err = h.recordCmd.Handle(r.Context(), commands.RecordPaymentCommand{
		AmountCents: amountCents,
		Currency:    "EUR",
		Reference:   signals.Reference,
		PayerName:   signals.PayerName,
		PayerIBAN:   signals.PayerIBAN,
		BookingDate: bookingDate,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/payments")
}

func (h *Handlers) importSSE(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	format := r.FormValue("format")
	if format == "" {
		format = "sepa_csv"
	}

	payments, err := h.importCmd.Handle(r.Context(), commands.ImportPaymentsCommand{
		Format: format,
		Reader: file,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_ = payments
	http.Redirect(w, r, "/payments", http.StatusSeeOther)
}

func (h *Handlers) matchSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var signals struct {
		InvoiceID  string `json:"invoiceID"`
		CustomerID string `json:"customerID"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	err := h.matchCmd.Handle(r.Context(), commands.MatchPaymentCommand{
		PaymentID:  id,
		InvoiceID:  signals.InvoiceID,
		CustomerID: signals.CustomerID,
	})
	if err != nil {
		sse.PatchElementTempl(formError(err.Error()))
		return
	}

	sse.Redirect("/payments/" + id)
}

func (h *Handlers) unmatchedSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	payments, err := h.unmatchedQ.Handle(r.Context(), queries.UnmatchedPaymentsQuery{})
	if err != nil {
		sse.ConsoleError(err)
		return
	}

	sse.PatchElementTempl(UnmatchedPaymentsList(payments))
}

func parseAmountCents(s string) int64 {
	f, _ := strconv.ParseFloat(s, 64)
	return int64(f * 100)
}
