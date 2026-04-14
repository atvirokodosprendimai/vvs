---
tldr: Backend-driven autocomplete on invoice form — customer and product search suggestions, backend sets selected values via MergeSignals + clears dropdown via PatchElementTempl
status: completed
---

# Plan: Invoice form autocomplete — backend driven

## Context

- Spec: [[spec - invoice - lifecycle and calculation.md]] (to be created/updated)
- Related: invoice form at `modules/invoice/adapters/http/`
- Customer data lives in `modules/customer/`; product data in `modules/product/`

**Pattern (Datastar backend-driven):**
1. Input `data-on:input__debounce.50ms="@get('/api/autocomplete/customers')"` — Datastar sends all signals including the search term
2. Backend queries DB, returns `PatchElementTempl` on `#customer-ac` — renders dropdown with suggestions
3. User clicks suggestion → `data-on:click="@get('/api/autocomplete/customers/select?id=X')"` — backend reads customer by ID
4. Select handler responds: `PatchSignals({customerId, customerName, customerSearch})` + `PatchElementTempl(empty dropdown)` — dropdown gone, signals set
5. Same pattern for each product line field

**Signal design:**
- Customer: `customerSearch` (what user types), `customerId`, `customerName` — flat
- Line items: `line0Query`, `line1Query`, `line2Query` (search term per row); `lines[N].productName`, `lines[N].unitPrice` as submit signals
- On product select: `MarshalAndPatchSignals` sets full lines array (deep merge) with updated `lines[N].productName` + `lines[N].unitPrice` + `lineNQuery`

## Phases

### Phase 1 — Customer autocomplete — status: completed

1. [x] Write tests for `GET /api/autocomplete/customers` SSE handler
   - => `autocomplete_test.go`: TestCustomerAutocomplete_EmptyQuery, MatchingQuery, CaseInsensitive, MatchesByCode
   - => uses in-memory SQLite with customers table; sseGETRequest helper encodes ?datastar= param

2. [x] Implement `GET /api/autocomplete/customers` SSE handler
   - => `autocomplete.go`: reads `customerSearch` signal; queries `company_name LIKE ? OR code LIKE ?` with LIMIT 10
   - => `fragments.templ`: `CustomerSuggestions(rows)` component with `id="customer-ac"` outer div

3. [x] Write tests for `GET /api/autocomplete/customers/select` SSE handler
   - => TestCustomerSelect_SetsSignals, TestCustomerSelect_ClearsDropdown

4. [x] Implement `GET /api/autocomplete/customers/select` SSE handler
   - => `autocomplete.go`: fetches by id, calls `sse.PatchSignals(json)` + `PatchElementTempl(CustomerSuggestions(nil))`
   - => routes registered in `RegisterRoutes`

5. [x] Update `InvoiceFormPage` template
   - => `templates.templ`: single `customerSearch` input with `data-on:input__debounce.50ms`
   - => `<div id="customer-ac"></div>` below input; `data-show`+`data-text` for selected customer display
   - => `invoiceFormSignals`: added `customerSearch`, `customerId`, `customerName` (separate from search)

### Phase 2 — Product line autocomplete — status: completed

6. [x] Write tests for `GET /api/autocomplete/products` SSE handler
   - => TestProductAutocomplete_EmptyQuery, MatchingQuery, Line1Query

7. [x] Implement `GET /api/autocomplete/products` SSE handler
   - => reads `line{N}Query` based on `?line=N`; queries products LIKE; returns `ProductSuggestions(rows, line)`
   - => `fragments.templ`: `ProductSuggestions(rows, line)` with `id="product-ac-{line}"`

8. [x] Write tests for `GET /api/autocomplete/products/select`
   - => TestProductSelect_SetsSignals, TestProductSelect_ClearsDropdown

9. [x] Implement `GET /api/autocomplete/products/select`
   - => reads full signals (lines array), updates `lines[lineIdx].productName/unitPrice/productId`, sets `line{N}Query`
   - => `MarshalAndPatchSignals(signals)` + `PatchElementTempl(ProductSuggestions(nil, line))`

10. [x] Update `InvoiceFormPage` template line item rows
    - => 3 line rows, each: `lineNQuery` search input + `#product-ac-N` dropdown + description + qty + unitPrice
    - => `invoiceFormSignals`: includes `line0Query`, `line1Query`, `line2Query` + 3 empty lines
    - => `createSSE`: filters empty lines (productName == "")

### Phase 3 — Shared infrastructure — status: completed

11. [x] Extract customer + product read access for invoice adapter
    - => added `reader *gorm.DB` field to `Handlers`, `reader` param to `NewHandlers`
    - => `app.go`: passes `reader` to `invoicehttp.NewHandlers`

## Verification

- Type "Acme" in customer field on /invoices/new → dropdown appears with matching customers
- Click a customer → dropdown closes, customerId + customerName fields populated, no page reload
- Type "Inter" in first product line → dropdown with matching products appears
- Click a product → product name + unit price populated, dropdown closed
- Submit the invoice → form data includes the autocompleted values correctly

## Progress Log

- 2604141322 — Plan created
- 2604141457 — All 11 actions implemented in one commit (2b7249c); 11 tests pass
