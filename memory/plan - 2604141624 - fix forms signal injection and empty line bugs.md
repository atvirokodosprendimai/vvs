---
tldr: Fix three form bugs — single-quote JS injection in formSignals, parseMoneyInput empty string, recurring empty-line submission
status: active
---

# Plan: Fix Forms — Signal Injection + Empty Lines + Unit Price

## Context

- Builds on: [[plan - 2604141423 - fix invalid unit price on invoice create.md]] (unit price parse — execute that first or fold in here)
- All form signal builders (`formSignals`, `prodFormSignals`, `recurringFormSignals`, `invoiceFormSignals`) use `fmt.Sprintf` with `'%s'` to build JS object literals
- Any entity with `'` (apostrophe) in a field breaks the JS signal → form fails to load / signals corrupt on edit page

## Bugs

### Bug 1 — Single-quote injection in `formSignals` (CRITICAL)

All `*FormSignals` functions do:
```go
fmt.Sprintf(`{companyName:'%s',...}`, c.CompanyName, ...)
```

`c.CompanyName = "O'Brien Ltd"` → `{companyName:'O'Brien Ltd'}` → JS parse error → form broken.

**Affected:**
- `formSignals(c)` — `customer/adapters/http/templates.templ:233`
- `prodFormSignals(p)` — `product/adapters/http/templates.templ:290`
- `recurringFormSignals(inv)` — `recurring/adapters/http/templates.templ:365`
- `invoiceFormSignals(inv)` — `invoice/adapters/http/templates.templ:341`

**Fix:** Use `json.Marshal` to build signal struct → proper JSON string with escaped values. `data-signals` accepts JSON.

### Bug 2 — `parseMoneyInput("")` panics with parse error (invoice create)

- `parseMoneyInput` in `invoice/adapters/http/handlers.go:253` doesn't handle `""`
- `parsePriceCents` (product handlers) already handles it: `if s == "" { return 0, nil }`
- `parseAmountCents` (recurring handlers) silently returns 0 for empty/invalid
- Fix: add `if s == "" { return 0, nil }` to `parseMoneyInput` — same as `parsePriceCents`

### Bug 3 — Recurring form submits empty lines

- `recurring/adapters/http/handlers.go:135` iterates all `LineProductNames` including empty ones
- Creates recurring invoice lines with `ProductName: ""` (invalid)
- Fix: add `if signals.LineProductNames[i] == "" { continue }` — same as invoice form

## Phases

### Phase 1 — Fix parseMoneyInput (invoice unit price) — status: open

1. [ ] Fix `parseMoneyInput` in `invoice/adapters/http/handlers.go:253`
   - Add `if s == "" { return 0, nil }` before `strconv.ParseFloat`
   - Write test: `TestCreateInvoice_EmptyUnitPrice` — line with `unitPrice:""` → accepted, price = 0
   - Write test: `TestCreateInvoice_NumericUnitPrice` — line with `unitPrice:29.99` (JSON number) → check type coercion

2. [ ] Mark [[plan - 2604141423 - fix invalid unit price on invoice create.md]] as completed

### Phase 2 — Fix formSignals injection — status: open

3. [ ] Fix `formSignals(c)` in `customer/adapters/http/templates.templ:233`
   - Replace `fmt.Sprintf` with `json.Marshal` of a struct
   - Test: create/edit customer with apostrophe in name → form loads, saves correctly

4. [ ] Fix `prodFormSignals(p)` in `product/adapters/http/templates.templ:290`
   - Same approach: marshal struct to JSON

5. [ ] Fix `recurringFormSignals(inv)` in `recurring/adapters/http/templates.templ:365`
   - Same approach: marshal struct to JSON
   - Note: parallel array structure `lineProductNames:['x','y','z']` must be preserved

6. [ ] Fix `invoiceFormSignals(inv)` in `invoice/adapters/http/templates.templ:341`
   - Same approach: marshal struct to JSON

### Phase 3 — Fix recurring empty line submission — status: open

7. [ ] Fix `createSSE` in `recurring/adapters/http/handlers.go:135`
   - Add `if i >= len(signals.LineProductNames) || signals.LineProductNames[i] == "" { continue }`

8. [ ] Fix `updateSSE` in `recurring/adapters/http/handlers.go:198`
   - Same empty-line skip

## Verification

- Customer with `O'Brien Ltd` → edit page loads without JS error, signals correct
- Product with `D'Angelo` in name → edit page loads correctly
- Invoice → add line without price → submit → no error, price = 0
- Recurring → create with 1 of 3 rows filled → only 1 line saved

## Progress Log

- 2604141624 — Plan created
