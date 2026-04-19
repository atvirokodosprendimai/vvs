package http

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/payment/app/commands"
	authdomain "github.com/vvs/isp/internal/modules/auth/domain"
)

// Handlers wires the payment import HTTP endpoints.
type Handlers struct {
	previewCmd *commands.PreviewImportHandler
	confirmCmd *commands.ConfirmImportHandler
}

// NewHandlers constructs a Handlers value.
func NewHandlers(preview *commands.PreviewImportHandler, confirm *commands.ConfirmImportHandler) *Handlers {
	return &Handlers{previewCmd: preview, confirmCmd: confirm}
}

// RegisterRoutes registers all payment import routes with the chi router.
// Auth is handled by the global middleware in the router — no per-module middleware needed.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/payments/import", h.showImportPage)
	// previewImport is a regular POST (not SSE) because multipart/form-data is required.
	r.Post("/payments/import/preview", h.previewImport)
	// confirmImport is a Datastar SSE endpoint: reads selectedIds signal, marks invoices paid.
	r.Post("/payments/import/confirm", h.confirmImport)
}

// showImportPage renders the CSV upload form.
func (h *Handlers) showImportPage(w http.ResponseWriter, r *http.Request) {
	PaymentImportPage().Render(r.Context(), w)
}

// previewImport handles the CSV upload (regular multipart POST, not Datastar SSE).
// It parses the file, runs match logic, and returns the full preview page.
func (h *Handlers) previewImport(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 5 << 20 // 5 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(1 << 20); err != nil {
		log.Printf("payment handler: previewImport: ParseMultipartForm: %v", err)
		http.Error(w, "could not parse upload (file may be too large)", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("csv_file")
	if err != nil {
		log.Printf("payment handler: previewImport: FormFile: %v", err)
		http.Error(w, "no file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		log.Printf("payment handler: previewImport: ReadAll: %v", err)
		http.Error(w, "could not read file", http.StatusInternalServerError)
		return
	}

	results, err := h.previewCmd.Handle(r.Context(), commands.PreviewImportCommand{CSVData: data})
	if err != nil {
		log.Printf("payment handler: previewImport: Handle: %v", err)
		http.Error(w, "could not parse CSV file", http.StatusBadRequest)
		return
	}

	PaymentImportPreviewPage(results).Render(r.Context(), w)
}

// confirmImport is a Datastar SSE handler.
// It reads the `selectedIds` signal (comma-separated invoice IDs), marks them paid,
// then SSE-patches a success fragment and redirects.
func (h *Handlers) confirmImport(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	var signals struct {
		SelectedIds string `json:"selectedIds"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("payment handler: confirmImport: ReadSignals: %v", err)
		sse.PatchElementTempl(paymentImportError("Could not read selection."))
		return
	}

	var ids []string
	for _, id := range strings.Split(signals.SelectedIds, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}

	result, err := h.confirmCmd.Handle(r.Context(), commands.ConfirmImportCommand{InvoiceIDs: ids})
	if err != nil {
		log.Printf("payment handler: confirmImport: Handle: %v", err)
		sse.PatchElementTempl(paymentImportError(err.Error()))
		return
	}

	if len(result.Errors) > 0 {
		log.Printf("payment handler: confirmImport: partial errors: %v", result.Errors)
		sse.PatchElementTempl(paymentImportPartialSuccess(len(result.MarkedPaid), result.Errors))
		return
	}
	sse.PatchElementTempl(PaymentImportSuccess(len(result.MarkedPaid)))
	sse.Redirect("/payments/import")
}

func (h *Handlers) ModuleName() authdomain.Module { return authdomain.ModulePayments }
