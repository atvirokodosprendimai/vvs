---
tldr: Invoice list shows amount 0 — lineItem.UnitPrice is string, Datastar may coerce to JSON number on submit, Go string field rejects JSON number → empty → 0 price
status: active
---

# Plan: Fix Invoice Amount 0

## Root Cause

`lineItem.UnitPrice string` → `json.Marshal` produces `"unitPrice":"29.99"` (JSON string) → Datastar stores in `$lines[0].unitPrice` as JS string `"29.99"`.

On form submit, Datastar serializes signals to JSON. JS may convert the numeric-looking string `"29.99"` to number `29.99` (or signal deserializer does). POST body: `"unitPrice":29.99` (JSON number).

`createSSE` reads `UnitPrice string` → `json.Unmarshal` of JSON number into string → type error → field left as `""` → `parseMoneyInput("")` → 0 → invoice created with 0 total.

(Before the `parseMoneyInput` empty-string fix in plan 2604141624, this manifested as "Invalid unit price for line 1" error.)

## Fix

Change `lineItem.UnitPrice` from `string` to `int64` (cents). Parsed once in `addLineSSE` from `NewLineUnitPrice string`, stored as JSON number in `$lines` signal → no string-number ambiguity.

## Context

- Related: [[plan - 2604141624 - fix forms signal injection and empty line bugs.md]] — parseMoneyInput empty string fix revealed this bug by silently accepting 0 instead of erroring

## Phases

### Phase 1 — Change lineItem.UnitPrice to int64 — status: open

1. [ ] Update `lineItem` struct in `autocomplete.go`: `UnitPrice string` → `UnitPrice int64`

2. [ ] Update `addLineSSE` in `line_items.go`:
   - Parse `signals.NewLineUnitPrice` to int64 cents: `price, _ := parseMoneyInput(signals.NewLineUnitPrice)` (reuse existing func)
   - Build `lineItem{..., UnitPrice: price}`
   - `signals.NewLineUnitPrice = ""` reset stays (it's the input signal, not lineItem)

3. [ ] Update `InvoiceLineTable` in `fragments.templ`:
   - `line.UnitPrice` is now `int64` — format for display: `fmt.Sprintf("%.2f", float64(line.UnitPrice)/100)`

4. [ ] Update `createSSE` anonymous struct in `handlers.go`:
   - Change `UnitPrice string` → `UnitPrice int64` (JSON number ← no parseMoneyInput needed)
   - Remove `parseMoneyInput(l.UnitPrice)` call → use `l.UnitPrice` directly as cents

5. [ ] Update tests in `autocomplete_test.go`:
   - Change all `"unitPrice":"X.XX"` → `"unitPrice":CENTS` (integer cents) in signal bodies
   - e.g. `"unitPrice":"10.00"` → `"unitPrice":1000`, `"unitPrice":"20.00"` → `"unitPrice":2000`
   - Add/update test asserting that `addLineSSE` correctly converts `newLineUnitPrice:"29.99"` → stored as `2999` in lines signal

## Verification

- Create invoice with one line (product at €29.99) → list shows total ~€36.28 (with 21% tax)
- `go test ./internal/modules/invoice/adapters/http/...` all pass

## Progress Log

- 2604141659 — Plan created
