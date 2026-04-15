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

### Phase 1 — Infrastructure: typed subscription helper — status: completed

1. [x] Add `ChanSubscriptionOf[T any]` to `internal/infrastructure/nats/subscriber.go`
   - signature: `func ChanSubscriptionOf[T any](s *Subscriber, subject string) (<-chan T, func())`
   - unmarshals `event.Data` (not the envelope) into T
   - same drop-on-slow-consumer semantics as `ChanSubscription`
   - => 3 tests: typed payload, wildcard subject, cancel closes channel

### Phase 2 — Invoice: write side publishes full read model — status: completed

2. [x] `GetReadModel` not needed — map from domain object in-memory
   - => `domainToReadModel(*domain.Invoice) queries.InvoiceReadModel` in commands/readmodel.go
   - => avoids DB round-trip and cross-layer import

3. [x] `CreateInvoiceHandler.Handle` — publishes `domainToReadModel(invoice)` in `event.Data`
   - replaces `{id, number}` minimal payload
   - => test: `TestDomainToReadModel_MapsFieldsCorrectly` asserts all fields including TotalAmount

4. [x] Same for `FinalizeInvoiceHandler` and `VoidInvoiceHandler`
   - => finalized/voided status reflected in payload

5. [x] Add json tags (snake_case) to `InvoiceReadModel`
   - => consistent event JSON; no GORM impact

### Phase 3 — Invoice: SSE list handler goes live — status: completed

6. [x] `InvoiceRow` component already existed with `id={fmt.Sprintf("invoice-%s", inv.ID)}`
   - `InvoiceTable` already calls `@InvoiceRow` per row — nothing to refactor

7. [x] Add `ChanSubscription` to `EventSubscriber` interface (shared/events/event.go)
   - => concrete `*nats.Subscriber` already implemented it

8. [x] Update `listSSE` to subscribe BEFORE initial render, then loop on channel:
   - subscribe to `isp.invoice.*` (wildcard catches created/finalized/voided)
   - initial render → `InvoiceTable`
   - per event: unmarshal `event.Data` → `InvoiceRow(item)` patch
   - exits on `r.Context().Done()` (client disconnect)
   - => no re-query on live update path

### Phase 4 — Rollout to other modules — status: open

9. [ ] Customer module — same pattern: json tags on read model, `domainToReadModel`, publish full model, listSSE stays alive
10. [ ] Product module
11. [ ] Recurring module

## Verification

- Create invoice → list view shows new row without page refresh
- Finalize invoice → row status badge updates in real time across open tabs
- No additional DB query triggered by live update (verify: `listQuery.Handle` not called in live path)
- All existing tests pass

## Progress Log

- 2604150940 — Plan created; spec [[spec - nats - per item event payload streaming]] written first
- 2604151100 — Phase 1-3 complete: ChanSubscriptionOf[T], full read model events, live listSSE for invoice
