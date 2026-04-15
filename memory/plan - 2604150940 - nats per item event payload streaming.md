---
tldr: Events carry full read-model payload; SSE handlers subscribe and patch individual rows — no re-query on live update
status: active
---

# NATS Per-Item Event Payload Streaming

## Context

Spec: [[spec - nats - per item event payload streaming]]
Arch: [[spec - architecture - system design and key decisions]]

Currently `listSSE` renders once (no NATS live updates at all). Commands publish minimal `{id, number}` in `event.Data`. Goal: events carry full read model → SSE handlers stay alive, patch rows as items arrive.

## Phases

### Phase 1 — Infrastructure: typed subscription helper — status: open

1. [ ] Add `ChanSubscriptionOf[T any]` to `internal/infrastructure/nats/subscriber.go`
   - signature: `func ChanSubscriptionOf[T any](s *Subscriber, subject string) (<-chan T, func())`
   - unmarshals `event.Data` (not the envelope) into T
   - same drop-on-slow-consumer semantics as `ChanSubscription`
   - write test: publish event with `Data: json.Marshal(struct{Name string}{Name:"x"})`, assert typed chan receives it

### Phase 2 — Invoice: write side publishes full read model — status: open

2. [ ] Add `GetReadModel(ctx, id) (InvoiceReadModel, error)` to `InvoiceRepository` interface + GORM impl
   - queries `invoices` table by id, returns `queries.InvoiceReadModel`

3. [ ] `CreateInvoiceHandler.Handle` — after `repo.Save`, call `GetReadModel`, marshal to `event.Data`
   - replaces current `{id, number}` minimal payload
   - test: assert published event.Data unmarshals to `InvoiceReadModel` with correct TotalAmount

4. [ ] Same for `FinalizeInvoiceHandler` and `VoidInvoiceHandler`
   - finalized: status = "finalized" in payload
   - voided: status = "void" in payload

### Phase 3 — Invoice: SSE list handler goes live — status: open

5. [ ] Add `InvoiceTableRow(item queries.InvoiceReadModel) templ.Component` to `fragments.templ`
   - id: `invoice-row-{item.ID}` — allows morph of single row
   - same columns as existing table row in `InvoiceTable`

6. [ ] Refactor `InvoiceTable` to call `@InvoiceTableRow(inv)` per row — remove duplicated row HTML

7. [ ] Update `listSSE` to subscribe to `isp.invoice.*` after initial render:
   ```go
   ch, cancel := nats.ChanSubscriptionOf[queries.InvoiceReadModel](h.sub, "isp.invoice.*")
   defer cancel()
   // initial render
   result, _ := h.listQuery.Handle(...)
   sse.PatchElementTempl(InvoiceTable(result))
   // live updates
   for item := range ch {
       sse.PatchElementTempl(InvoiceTableRow(item))
   }
   ```
   - uses wildcard subject to catch created/finalized/voided
   - test: open listSSE, publish typed event, assert row patch emitted without re-query

### Phase 4 — Rollout to other modules — status: open

8. [ ] Customer module — same pattern: `ChanSubscriptionOf[CustomerReadModel]`, publish full model, per-row patch in listSSE
9. [ ] Product module
10. [ ] Recurring module

## Verification

- Create invoice → list view shows new row without page refresh
- Finalize invoice → row status badge updates in real time across open tabs
- No additional DB query triggered by live update (verify by checking handler — `listQuery.Handle` not called in live path)
- All existing tests pass

## Progress Log

- 2604150940 — Plan created; spec [[spec - nats - per item event payload streaming]] written first
