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

// GET /api/autocomplete/products?line=N
// Reads line{N}Query signal; returns PatchElementTempl(ProductSuggestions) for the given line.
func (h *Handlers) productAutocompleteSSE(w http.ResponseWriter, r *http.Request) {
	lineStr := r.URL.Query().Get("line")
	lineIdx := 0
	fmt.Sscanf(lineStr, "%d", &lineIdx)

	var signals struct {
		Line0Query string `json:"line0Query"`
		Line1Query string `json:"line1Query"`
		Line2Query string `json:"line2Query"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	var query string
	switch lineIdx {
	case 0:
		query = signals.Line0Query
	case 1:
		query = signals.Line1Query
	case 2:
		query = signals.Line2Query
	}

	if query == "" {
		sse.PatchElementTempl(ProductSuggestions(nil, lineIdx))
		return
	}

	search := "%" + query + "%"
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
	sse.PatchElementTempl(ProductSuggestions(suggestions, lineIdx))
}

// GET /api/autocomplete/products/select?id=X&line=N
// Reads all current line signals; updates lines[N].productName + unitPrice and line{N}Query; clears dropdown.
func (h *Handlers) productSelectSSE(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	lineStr := r.URL.Query().Get("line")
	if id == "" || lineStr == "" {
		http.Error(w, "missing id or line", http.StatusBadRequest)
		return
	}

	lineIdx := 0
	fmt.Sscanf(lineStr, "%d", &lineIdx)

	type lineSignal struct {
		ProductID   string `json:"productId"`
		ProductName string `json:"productName"`
		Description string `json:"description"`
		Quantity    int    `json:"quantity"`
		UnitPrice   string `json:"unitPrice"`
	}
	var signals struct {
		Line0Query string       `json:"line0Query"`
		Line1Query string       `json:"line1Query"`
		Line2Query string       `json:"line2Query"`
		Lines      []lineSignal `json:"lines"`
	}
	// ReadSignals returns nil when datastar param is absent — safe to ignore error here
	_ = datastar.ReadSignals(r, &signals)

	// Pad Lines so lineIdx is in range
	for len(signals.Lines) <= lineIdx {
		signals.Lines = append(signals.Lines, lineSignal{Quantity: 1})
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

	unitPriceStr := fmt.Sprintf("%.2f", float64(p.PriceAmount)/100)
	signals.Lines[lineIdx].ProductID = p.ID
	signals.Lines[lineIdx].ProductName = p.Name
	signals.Lines[lineIdx].UnitPrice = unitPriceStr

	switch lineIdx {
	case 0:
		signals.Line0Query = p.Name
	case 1:
		signals.Line1Query = p.Name
	case 2:
		signals.Line2Query = p.Name
	}

	sse := datastar.NewSSE(w, r)
	sse.MarshalAndPatchSignals(signals)
	sse.PatchElementTempl(ProductSuggestions(nil, lineIdx))
}
