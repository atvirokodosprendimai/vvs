package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
)

type customerSuggestion struct {
	ID   string
	Name string
	Code string
}

type productSuggestion struct {
	ID        string
	Name      string
	UnitPrice int64
	Currency  string
}

// lineItem is the shape of each element in the `lines` Datastar signal array.
// JSON tags must match the signal field names sent by the browser.
type lineItem struct {
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitPrice   string `json:"unitPrice"`
}

// GET /api/autocomplete/customers
// Reads customerSearch signal; returns PatchElementTempl(CustomerSuggestions) with matching rows.
func (h *Handlers) customerAutocompleteSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		CustomerSearch string `json:"customerSearch"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	if signals.CustomerSearch == "" {
		sse.PatchElementTempl(CustomerSuggestions(nil))
		return
	}

	search := "%" + signals.CustomerSearch + "%"
	type row struct {
		ID          string
		CompanyName string
		Code        string
	}
	var rows []row
	if err := h.reader.Raw(
		"SELECT id, company_name, code FROM customers WHERE (company_name LIKE ? OR code LIKE ?) AND status = 'active' LIMIT 10",
		search, search,
	).Scan(&rows).Error; err != nil {
		sse.ConsoleError(err)
		return
	}

	suggestions := make([]customerSuggestion, len(rows))
	for i, r := range rows {
		suggestions[i] = customerSuggestion{ID: r.ID, Name: r.CompanyName, Code: r.Code}
	}
	sse.PatchElementTempl(CustomerSuggestions(suggestions))
}

// GET /api/autocomplete/customers/select?id=X
// Fetches customer by ID; sets customerId/customerName/customerSearch signals and clears dropdown.
func (h *Handlers) customerSelectSSE(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	type row struct {
		ID          string
		CompanyName string
	}
	var c row
	if err := h.reader.Raw("SELECT id, company_name FROM customers WHERE id = ?", id).Scan(&c).Error; err != nil || c.ID == "" {
		http.Error(w, "customer not found", http.StatusNotFound)
		return
	}

	sse := datastar.NewSSE(w, r)

	signals := struct {
		CustomerID     string `json:"customerId"`
		CustomerName   string `json:"customerName"`
		CustomerSearch string `json:"customerSearch"`
	}{
		CustomerID:     c.ID,
		CustomerName:   c.CompanyName,
		CustomerSearch: c.CompanyName,
	}
	b, _ := json.Marshal(signals)
	sse.PatchSignals(b)
	sse.PatchElementTempl(CustomerSuggestions(nil))
}

// GET /api/autocomplete/products
// Reads newLineSearch signal; returns PatchElementTempl(ProductSuggestions) for the new-line row.
func (h *Handlers) productAutocompleteSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		NewLineSearch string `json:"newLineSearch"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	if signals.NewLineSearch == "" {
		sse.PatchElementTempl(ProductSuggestions(nil))
		return
	}

	search := "%" + signals.NewLineSearch + "%"
	type row struct {
		ID            string
		Name          string
		PriceAmount   int64
		PriceCurrency string
	}
	var rows []row
	if err := h.reader.Raw(
		"SELECT id, name, price_amount, price_currency FROM products WHERE name LIKE ? AND is_active = 1 LIMIT 10",
		search,
	).Scan(&rows).Error; err != nil {
		sse.ConsoleError(err)
		return
	}

	suggestions := make([]productSuggestion, len(rows))
	for i, r := range rows {
		suggestions[i] = productSuggestion{ID: r.ID, Name: r.Name, UnitPrice: r.PriceAmount, Currency: r.PriceCurrency}
	}
	sse.PatchElementTempl(ProductSuggestions(suggestions))
}

// GET /api/autocomplete/products/select?id=X
// Fetches product by ID; sets newLine* signals (search, productId, productName, unitPrice);
// replaces #new-line-row HTML with all fields pre-filled; clears dropdown.
// Preserves user-entered newLineDescription and newLineQty from current signals.
func (h *Handlers) productSelectSSE(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	type row struct {
		ID            string
		Name          string
		PriceAmount   int64
		PriceCurrency string
	}
	var p row
	if err := h.reader.Raw("SELECT id, name, price_amount, price_currency FROM products WHERE id = ?", id).Scan(&p).Error; err != nil || p.ID == "" {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	// Read current signals to preserve user-entered description and qty.
	var cur struct {
		NewLineDescription string `json:"newLineDescription"`
		NewLineQty         int    `json:"newLineQty"`
	}
	_ = datastar.ReadSignals(r, &cur) // missing signals are fine — zero values used as defaults
	if cur.NewLineQty <= 0 {
		cur.NewLineQty = 1
	}

	sse := datastar.NewSSE(w, r)

	unitPriceStr := fmt.Sprintf("%.2f", float64(p.PriceAmount)/100)
	signals := struct {
		NewLineSearch      string `json:"newLineSearch"`
		NewLineProductID   string `json:"newLineProductId"`
		NewLineProductName string `json:"newLineProductName"`
		NewLineUnitPrice   string `json:"newLineUnitPrice"`
	}{
		NewLineSearch:      p.Name,
		NewLineProductID:   p.ID,
		NewLineProductName: p.Name,
		NewLineUnitPrice:   unitPriceStr,
	}
	b, _ := json.Marshal(signals)
	sse.PatchSignals(b)
	sse.PatchElementTempl(AddLineRow(p.Name, cur.NewLineDescription, cur.NewLineQty, unitPriceStr))
	sse.PatchElementTempl(ProductSuggestions(nil))
}
