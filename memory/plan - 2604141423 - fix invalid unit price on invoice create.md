---
tldr: Fix "Invalid unit price for line 1" error when submitting invoice form — handle empty string and JSON number/string type mismatch for unitPrice
status: active
---

# Plan: Fix "Invalid unit price for line 1"

## Context

- Error path: `createSSE` in `handlers.go:165` → `parseMoneyInput(l.UnitPrice)` → `strconv.ParseFloat("", 64)` → parse error → `formError("Invalid unit price for line 1")`
- Two causes:
  1. User adds a line without selecting from product autocomplete → `newLineUnitPrice` stays `""` → `lineItem.UnitPrice = ""` stored in `lines` signal → fails on submit
  2. Potential Datastar type coercion: Datastar JS may store `"29.99"` as number `29.99` in the signal store, sent back as JSON number → Go `string` field rejects JSON number → zero value `""` → same failure

## Root Cause

```go
// handlers.go:253
func parseMoneyInput(s string) (int64, error) {
    f, err := strconv.ParseFloat(s, 64)  // "" → error "invalid syntax"
    ...
}
```

And in `createSSE`, the `Lines[].UnitPrice` field is `string` — JSON number would deserialize as `""`.

## Fix

**Two-part fix:**

1. `parseMoneyInput`: treat `""` as 0 cents (valid, price of 0.00) — belt-and-suspenders
2. `lineItem` and `createSSE` struct: change `UnitPrice` type to handle both JSON string and JSON number via custom unmarshal. Store as string representation, accept either.

Actually, the cleanest fix: use `json.Number` for `UnitPrice` fields — it accepts both JSON strings and numbers, preserves string representation. But `json.Number` isn't a `string`, so display code and `parseMoneyInput` need adjusting.

**Simpler fix**: Keep `UnitPrice string` everywhere but:
1. Handle empty string in `parseMoneyInput` → 0
2. Add `allowNumber` to the JSON unmarshal by using a custom decoder or `json.RawMessage` inline in `createSSE`

**Chosen approach**: 
- Fix `parseMoneyInput` to handle `""` → 0 (fixes the most common case)
- In `createSSE` anonymous struct, use a helper type for `UnitPrice` that accepts JSON number too (fixes the type coercion case)
- OR: ensure Datastar always sends string by storing prices as strings only — already the case via `addLineSSE`, but need to verify

## Phases

### Phase 1 — Fix parseMoneyInput and unit price parsing — status: open

1. [ ] Fix `parseMoneyInput` in `handlers.go` to handle empty string → return 0, nil
   - Simplest fix: add early return for `s == ""`
   - Also handle JSON number coercion: change `UnitPrice` in the `createSSE` anonymous struct from `string` to a type that accepts both JSON string and JSON number
   - New helper type `flexPrice` with custom `UnmarshalJSON` in `handlers.go`: accepts `"29.99"` (string) or `29.99` (number), stores as string
   - Update `createSSE` to use `flexPrice` for `UnitPrice` and pass `string(l.UnitPrice)` to `parseMoneyInput`

2. [ ] Add tests for the fix in `autocomplete_test.go` (or a new `handlers_test.go`)
   - `TestCreateInvoice_EmptyUnitPrice` — line with `unitPrice:""` → no error, price = 0
   - `TestCreateInvoice_NumericUnitPrice` — line with `unitPrice:29.99` (JSON number) → parsed correctly as 2999 cents

## Verification

- Add line without selecting from autocomplete (leave price blank) → submit → no error
- Add line via autocomplete select → submit → no error
- Unit price shown correctly in created invoice

## Progress Log

- 2604141423 — Plan created
