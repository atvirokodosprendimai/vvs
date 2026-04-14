package http

import (
	"fmt"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
)

type lineFormSignals struct {
	Lines              []lineItem `json:"lines"`
	NewLineSearch      string     `json:"newLineSearch"`
	NewLineProductID   string     `json:"newLineProductId"`
	NewLineProductName string     `json:"newLineProductName"`
	NewLineDescription string     `json:"newLineDescription"`
	NewLineQty         int        `json:"newLineQty"`
	NewLineUnitPrice   string     `json:"newLineUnitPrice"`
}

// POST /api/invoices/lines
// Appends a new line item to the `lines` signal array and re-renders the table.
// Resets the new-line form fields after adding.
func (h *Handlers) addLineSSE(w http.ResponseWriter, r *http.Request) {
	var signals lineFormSignals
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	if signals.NewLineProductName == "" {
		sse.PatchElementTempl(formError("Product name is required"))
		return
	}

	qty := signals.NewLineQty
	if qty <= 0 {
		qty = 1
	}

	signals.Lines = append(signals.Lines, lineItem{
		ProductID:   signals.NewLineProductID,
		ProductName: signals.NewLineProductName,
		Description: signals.NewLineDescription,
		Quantity:    qty,
		UnitPrice:   signals.NewLineUnitPrice,
	})

	// Reset the add-line form
	signals.NewLineSearch = ""
	signals.NewLineProductID = ""
	signals.NewLineProductName = ""
	signals.NewLineDescription = ""
	signals.NewLineQty = 1
	signals.NewLineUnitPrice = ""

	sse.MarshalAndPatchSignals(signals)
	sse.PatchElementTempl(InvoiceLineTable(signals.Lines))
}

// DELETE /api/invoices/lines?idx=N
// Removes the line item at the given index from the `lines` signal array and re-renders the table.
// Out-of-range index is a no-op.
func (h *Handlers) removeLineSSE(w http.ResponseWriter, r *http.Request) {
	idxStr := r.URL.Query().Get("idx")
	idx := -1
	if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil {
		idx = -1
	}

	var signals struct {
		Lines []lineItem `json:"lines"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	if idx >= 0 && idx < len(signals.Lines) {
		signals.Lines = append(signals.Lines[:idx], signals.Lines[idx+1:]...)
	}

	sse.MarshalAndPatchSignals(signals)
	sse.PatchElementTempl(InvoiceLineTable(signals.Lines))
}
