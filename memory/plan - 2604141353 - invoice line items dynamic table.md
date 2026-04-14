---
tldr: Replace fixed 3-row product inputs with a dynamic table — add-line row at bottom (product searchable like customer), remove-row button per line, backend-driven via SSE helpers
status: active
---

# Plan: Invoice line items — dynamic table

## Context

- Builds on: [[plan - 2604141322 - invoice form autocomplete backend driven.md]]
- Invoice form at `internal/modules/invoice/adapters/http/`
- Pattern: Datastar backend-driven — all state lives in backend; form manipulation
  endpoints update the `lines` signal array and re-render the table via SSE

**Current state:**
- 3 fixed line rows with `line0Query`/`line1Query`/`line2Query` search inputs
- `lines` signal array with 3 pre-populated empty slots

**Target state:**
- Line items shown as a `<table id="invoice-lines">` — renders actual added lines
- Single "add line" row at bottom with: product search (like customer autocomplete) + description + qty + unit price + Add button
- Each table row has a Remove button
- No pre-populated empty lines — start with `lines: []`
- `POST /api/invoices/lines` — backend appends new line to signal array, re-renders table
- `DELETE /api/invoices/lines?idx=N` — backend removes line at index N, re-renders table

**New signal design:**
```
{
  customerSearch: '',  customerId: '',  customerName: '',
  issueDate: 'YYYY-MM-DD',  dueDate: 'YYYY-MM-DD',  taxRate: 21,
  lines: [],                              // accumulated line items (submitted)
  newLineSearch: '',                      // product autocomplete search
  newLineProductId: '',
  newLineProductName: '',
  newLineDescription: '',
  newLineQty: 1,
  newLineUnitPrice: '',
}
```

**Product autocomplete update:**
- Old: `line{N}Query` signal + `/api/autocomplete/products?line=N` per row
- New: `newLineSearch` signal + `/api/autocomplete/products` (no line param) + `#new-line-product-ac` dropdown
- Select: sets `newLineProductId`, `newLineProductName`, `newLineUnitPrice`; clears dropdown

## Phases

### Phase 1 — Add/remove line endpoints — status: open

1. [ ] Write tests for `POST /api/invoices/lines` (add-line handler)
   - test: adds new line to `lines` signal array and returns updated `InvoiceLineTable`
   - test: empty productName returns validation error
   - test: existing lines are preserved when adding

2. [ ] Implement `POST /api/invoices/lines`
   - reads signals: `lines` (current), `newLineProductId/Name/Description/Qty/UnitPrice`
   - appends new line to `lines` array
   - returns `MarshalAndPatchSignals({lines: updated, newLineSearch:'', newLineProductId:'', newLineProductName:'', newLineDescription:'', newLineQty:1, newLineUnitPrice:''})` + `PatchElementTempl(InvoiceLineTable(lines))`

3. [ ] Write tests for `DELETE /api/invoices/lines?idx=N` (remove-line handler)
   - test: removes correct index, re-renders table
   - test: invalid idx is a no-op (returns current table)

4. [ ] Implement `DELETE /api/invoices/lines?idx=N`
   - reads `lines` signal array, removes index N
   - returns `MarshalAndPatchSignals({lines: updated})` + `PatchElementTempl(InvoiceLineTable(updatedLines))`

5. [ ] Add `InvoiceLineTable` templ component to `fragments.templ`
   - `id="invoice-lines"` outer div
   - table with columns: Product, Description, Qty, Unit Price, Total, (remove button)
   - shows "No line items yet" when empty
   - register routes in `RegisterRoutes`

### Phase 2 — Update product autocomplete for new-line row — status: open

6. [ ] Update `productAutocompleteSSE` to read `newLineSearch` signal
   - remove `line{N}Query` / `?line=N` logic
   - reads `newLineSearch`; returns `ProductSuggestions(rows, 0)` targeting `#new-line-product-ac`

7. [ ] Update `productSelectSSE` to set `newLine*` signals
   - remove the per-line lines[] merge logic
   - sets `newLineProductId`, `newLineProductName`, `newLineUnitPrice`, `newLineSearch`
   - clears `#new-line-product-ac`

8. [ ] Update product autocomplete tests
   - replace `line0Query`/`line1Query` signal tests with `newLineSearch`
   - add test: select fills `newLineProductName`, `newLineUnitPrice`

### Phase 3 — Invoice form template refactor — status: open

9. [ ] Update `InvoiceFormPage` template
   - remove the 3 fixed line rows and `line0Query`/`line1Query`/`line2Query` signals
   - add `<div id="invoice-lines">` (initial empty table via `InvoiceLineTable(nil)`)
   - add "new line" row: product search input + `#new-line-product-ac` + description + qty + unit price + Add button
   - Add button: `data-on:click="@post('/api/invoices/lines')"`

10. [ ] Update `invoiceFormSignals` function
    - remove `line0Query`, `line1Query`, `line2Query`, pre-populated lines
    - add `newLineSearch`, `newLineProductId`, `newLineProductName`, `newLineDescription`, `newLineQty`, `newLineUnitPrice`
    - `lines` starts as empty array `[]`

## Verification

- `/invoices/new` → form shows empty line items table
- Type "Inter" in product search → dropdown with matching products
- Click product → unit price auto-filled, product name shown in search input
- Fill description + qty → click Add → line appears in table, add form clears
- Click Remove on a row → row disappears from table
- Submit → invoice created with the accumulated line items

## Progress Log

