---
tldr: Invoice list shows amount 0 — lineItem.UnitPrice is string, Datastar may coerce to JSON number on submit, Go string field rejects JSON number → empty → 0 price
status: completed
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

### Phase 1 — Change lineItem.UnitPrice to int64 — status: completed

1. [x] Update `lineItem` struct in `autocomplete.go`: `UnitPrice string` → `UnitPrice int64`
   - => added comment explaining why (JSON string/number coercion)

2. [x] Update `addLineSSE` in `line_items.go`:
   - => `price, _ := parseMoneyInput(signals.NewLineUnitPrice)` → `UnitPrice: price`
   - => `signals.NewLineUnitPrice = ""` reset unchanged (it's the input signal)

3. [x] Update `InvoiceLineTable` in `fragments.templ`:
   - => `fmt.Sprintf("%.2f", float64(line.UnitPrice)/100)`

4. [x] Update `createSSE` anonymous struct in `handlers.go`:
   - => `UnitPrice int64` — uses `l.UnitPrice` directly (already cents)
   - => removed `parseMoneyInput` call; also removed unused `i` from loop

5. [x] Updated tests — `"unitPrice":"X.XX"` → `"unitPrice":CENTS` in all signal bodies
   - => Added `TestAddLine_ValidLine` assertion: `"unitPrice":2999` present
   - => Added `TestAddLine_UnitPriceStoredAsCents`: 49.99 → 4999, not a JSON string

## Verification

- Create invoice with one line (product at €29.99) → list shows total ~€36.28 (with 21% tax)
- `go test ./internal/modules/invoice/adapters/http/...` all pass

## Progress Log

- 2604141659 — Plan created
- 2604141712 — All 5 actions done; 23 tests pass
