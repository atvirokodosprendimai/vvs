---
tldr: Fix three form bugs — single-quote JS injection in formSignals, parseMoneyInput empty string, recurring empty-line submission
status: completed
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

### Phase 1 — Fix parseMoneyInput (invoice unit price) — status: completed

1. [x] Fix `parseMoneyInput` in `invoice/adapters/http/handlers.go:253`
   - => Added `if s == "" { return 0, nil }` — matches `parsePriceCents` pattern in product handlers
   - => Tests: `TestParseMoneyInput_EmptyString`, `TestParseMoneyInput_ValidFloat`, `TestParseMoneyInput_Integer`, `TestParseMoneyInput_Zero` in `handlers_test.go`
   - => JSON number coercion: not tested — datastar uses JSON body for POST, `string` field rejects number type; will surface only if observed at runtime

2. [x] Mark [[plan - 2604141423 - fix invalid unit price on invoice create.md]] as completed

### Phase 2 — Fix formSignals injection — status: completed

3. [x] Fix `formSignals(c)` in `customer/adapters/http/templates.templ:233`
   - => replaced with `json.Marshal` of `sig` struct; `encoding/json` import added

4. [x] Fix `prodFormSignals(p)` in `product/adapters/http/templates.templ:290`
   - => same pattern; preserved default values (productType: "internet", priceCurrency: "EUR", billingPeriod: "monthly")

5. [x] Fix `recurringFormSignals(inv)` in `recurring/adapters/http/templates.templ:365`
   - => parallel arrays now `[]string` fields marshaled as JSON arrays; `strconv` import added

6. [x] Fix `invoiceFormSignals(inv)` in `invoice/adapters/http/templates.templ:303`
   - => full struct with `lines:[]any{}` (empty slice → `[]`), `newLineQty:1` preserved

### Phase 3 — Fix recurring empty line submission — status: completed

7. [x] Fix `createSSE` in `recurring/adapters/http/handlers.go:135`
   - => `if signals.LineProductNames[i] == "" { continue }` added at top of loop

8. [x] Fix `updateSSE` in `recurring/adapters/http/handlers.go:198`
   - => same skip added

## Verification

- Customer with `O'Brien Ltd` → edit page loads without JS error, signals correct
- Product with `D'Angelo` in name → edit page loads correctly
- Invoice → add line without price → submit → no error, price = 0
- Recurring → create with 1 of 3 rows filled → only 1 line saved

## Progress Log

- 2604141624 — Plan created
- 2604141640 — Phase 1 done: parseMoneyInput empty string fix + 4 tests; plan 2604141423 closed
- 2604141650 — Phase 2+3 done: all formSignals use json.Marshal; recurring empty lines skipped
