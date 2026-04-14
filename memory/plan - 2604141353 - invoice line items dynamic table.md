---
tldr: Replace fixed 3-row product inputs with a dynamic table — add-line row at bottom (product searchable like customer), remove-row button per line, backend-driven via SSE helpers
status: completed
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

### Phase 1 — Add/remove line endpoints — status: completed

1. [x] Write tests for `POST /api/invoices/lines` (add-line handler)
   - => TestAddLine_ValidLine, TestAddLine_EmptyProductName, TestAddLine_PreservesExistingLines, TestAddLine_DefaultsQtyToOne

2. [x] Implement `POST /api/invoices/lines`
   - => `line_items.go`: addLineSSE reads lineFormSignals, appends new lineItem, resets newLine* signals, returns MarshalAndPatchSignals + PatchElementTempl(InvoiceLineTable)
   - => `lineItem` type defined in `autocomplete.go` with JSON tags matching signal keys

3. [x] Write tests for `DELETE /api/invoices/lines?idx=N` (remove-line handler)
   - => TestRemoveLine_ValidIndex, TestRemoveLine_LastLine, TestRemoveLine_InvalidIndex, TestRemoveLine_RendersTableTarget

4. [x] Implement `DELETE /api/invoices/lines?idx=N`
   - => `line_items.go`: removeLineSSE reads lines, splices index N (no-op if out of range), returns updated signals + table

5. [x] Add `InvoiceLineTable` templ component to `fragments.templ`
   - => `id="invoice-lines"` outer div; shows "No line items yet" empty-state or table with Remove buttons
   - => Remove button: `@delete('/api/invoices/lines?idx=N')` per row
   - => routes registered in `RegisterRoutes`

### Phase 2 — Update product autocomplete for new-line row — status: completed

6. [x] Update `productAutocompleteSSE` to read `newLineSearch` signal
   - => removed `line{N}Query` / `?line=N` logic; reads `newLineSearch` only

7. [x] Update `productSelectSSE` to set `newLine*` signals
   - => sets `newLineSearch`, `newLineProductId`, `newLineProductName`, `newLineUnitPrice`; clears `#new-line-product-ac`
   - => `ProductSuggestions` component: `id="new-line-product-ac"`, no line param

8. [x] Update product autocomplete tests
   - => replaced `line0Query`/`line1Query` with `newLineSearch`; added TestProductAutocomplete_DropdownTarget
   - => TestProductSelect_FillsNewLineSignals checks newLineProductName/UnitPrice/Id signals

### Phase 3 — Invoice form template refactor — status: completed

9. [x] Update `InvoiceFormPage` template
   - => replaced 3 fixed line rows with `@InvoiceLineTable(nil)` + add-line row
   - => add-line row: newLineSearch input + #new-line-product-ac + description + qty + unitPrice + Add button (`@post('/api/invoices/lines')`)

10. [x] Update `invoiceFormSignals` function
    - => removed line0Query/line1Query/line2Query and pre-populated lines
    - => lines starts as `[]`, added newLine* fields

## Verification

- `/invoices/new` → form shows empty line items table ("No line items yet")
- Type "Inter" in product search → dropdown with matching products
- Click product → unit price auto-filled, product name shown in search input
- Fill description + qty → click Add → line appears in table, add form clears
- Click Remove on a row → row disappears from table
- Submit → invoice created with the accumulated line items

## Progress Log

- 2604141353 — Plan created
- 2604141402 — All 10 actions completed in one commit (4334a5d); 19 tests pass
