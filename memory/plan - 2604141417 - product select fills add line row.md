---
tldr: When a product is selected from autocomplete, replace the entire add-line row HTML with all fields pre-filled (product name, unit price) and patch signals
status: completed
---

# Plan: Product select fills add-line row

## Context

- Builds on: [[plan - 2604141353 - invoice line items dynamic table.md]]
- Invoice form at `internal/modules/invoice/adapters/http/`
- Pattern: Datastar backend-driven — `productSelectSSE` currently only patches signals and clears the dropdown

**Current state:**
- `productSelectSSE` sets `newLineSearch`, `newLineProductId`, `newLineProductName`, `newLineUnitPrice` signals and clears `#new-line-product-ac`
- Add-line row is inline HTML in `InvoiceFormPage` — no distinct element id for the whole row
- After product select: only the search input updates (via signal bind), price field stays blank until user sees it

**Target state:**
- Extract add-line row into a `AddLineRow(search, productId, productName, description string, qty int, unitPrice string)` templ component with `id="new-line-row"`
- `productSelectSSE` reads current `newLineDescription` and `newLineQty` from signals (GET request → `?datastar=<json>`)
- `productSelectSSE` returns:
  1. `MarshalAndPatchSignals` with all newLine* fields set
  2. `PatchElementTempl(AddLineRow(...))` — replaces entire row HTML with fields filled in
  3. `PatchElementTempl(ProductSuggestions(nil))` — clears dropdown
- The row HTML re-render makes all fields visually update simultaneously

**Signal flow:**
- User types in `newLineSearch` → product dropdown appears
- User clicks a product → `productSelectSSE` GET fires with full signal state
- Handler reads `newLineDescription` and `newLineQty` from signals (to preserve user-entered values)
- Handler emits: patch signals (all newLine* fields) + patch `#new-line-row` HTML + clear dropdown

## Phases

### Phase 1 — AddLineRow templ component — status: completed

1. [x] Extract add-line row into `AddLineRow` component in `fragments.templ`
   - => `id="new-line-row"` wrapping div; params: `search, description string, qty int, unitPrice string`
   - => all inputs have both `data-bind:*` and `value={param}` so both signal-binding and HTML pre-fill work
   - => `#new-line-product-ac` div included inside the component

2. [x] Update `InvoiceFormPage` to render `@AddLineRow("", "", 1, "")` instead of inline HTML
   - => replaced ~40 lines of inline HTML with one line

### Phase 2 — Update productSelectSSE — status: completed

3. [x] Update `productSelectSSE` in `autocomplete.go`
   - => reads `newLineDescription` and `newLineQty` via `datastar.ReadSignals` (GET query param — safe to call again)
   - => defaults qty to 1 if zero/missing
   - => emits: `PatchSignals` (newLine* fields) + `PatchElementTempl(AddLineRow(...))` + `PatchElementTempl(ProductSuggestions(nil))`

4. [x] Update tests — added `TestProductSelect_PatchesAddLineRow` and `TestProductSelect_PreservesDescriptionAndQty`
   - => `PatchesAddLineRow`: verifies `new-line-row` id and filled values in HTML
   - => `PreservesDescriptionAndQty`: sends `newLineDescription` + `newLineQty` in signals, verifies they appear in patched row

## Verification

- `/invoices/new` → type in product search → click a product
- All fields fill: product name in search input, unit price in price field
- Description and qty fields retain whatever the user typed before selecting
- Dropdown clears
- Click Add → line appears in table; add row resets

## Progress Log

- 2604141417 — Plan created
- 2604141430 — All 4 actions completed in one commit (c7ba7df); 21 tests pass
