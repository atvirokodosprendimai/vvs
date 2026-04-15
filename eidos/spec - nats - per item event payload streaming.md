---
tldr: NATS events carry full read-model payload so live UI updates patch individual rows without re-querying
category: core
---

# NATS Per-Item Event Payload Streaming

## Intent

Events are the data. When a write happens, the command publishes the affected entity's read model as the event payload. SSE list handlers receive a complete item and patch only that row — no re-query of the full list.

## Problem with Current Pattern

Current live-update loop:
1. Write → publish `DomainEvent{Data: {id, number}}` (minimal notification)
2. SSE handler receives event → re-queries ALL invoices → re-renders whole table

Two problems:
- Unnecessary full re-query on every live update
- Business logic on read side has to be duplicated in both initial render and live-update paths

## Streaming Pattern

### Write side

After saving, the command reads back (or constructs) the full read model and puts it in `event.Data`:

```go
// Before: minimal notification
data, _ := json.Marshal(map[string]string{"id": invoice.ID, "number": invoice.InvoiceNumber})

// After: full read model
rm, _ := h.repo.GetReadModel(ctx, invoice.ID)
data, _ := json.Marshal(rm)
```

### Subscribe side — typed helper

`Subscriber.ChanSubscriptionOf[T]` auto-unmarshals `event.Data` into T:

```go
ch, cancel := nats.ChanSubscriptionOf[queries.InvoiceReadModel](h.sub, "isp.invoice.created")
defer cancel()
for item := range ch {
    sse.PatchElementTempl(w, ctx, InvoiceTableRow(item))
}
```

### SSE handler — per-row patch

Initial render: unchanged — full query → render whole table.

Live update: subscribe → receive item → render that row's templ component → patch by row ID.

```
#invoice-row-{id}    ← morph just this row
#invoice-table-empty ← remove empty state if was showing
```

## Conventions

- Subject carries item type: `isp.invoice.created`, `isp.invoice.updated`, `isp.invoice.voided`
- `event.Data` is always the current read model JSON for the affected entity
- SSE handlers use `ChanSubscriptionOf[T]` — never `ChanSubscription` for list live-updates
- Row element ID: `{module}-row-{id}` (e.g. `invoice-row-abc123`)
- For deletes/voids: patch the row with a new status, or remove element by ID

## Behaviour

- Live list updates show instantly without full page re-render
- Multiple open browser tabs all update in real time
- Business logic (display formatting, status badge) lives in one templ component, called from both initial render and live update
- Delete/void: commander publishes `{id, status}` only (no full model needed to remove a row)

## Infrastructure

```
internal/infrastructure/nats/subscriber.go
  + ChanSubscriptionOf[T any](s *Subscriber, subject string) (<-chan T, func())
```

Uses Go generics — auto-unmarshals `event.Data` (not the envelope itself) into T.

## Rollout Order

1. Invoice (most complex, validates the pattern)
2. Customer
3. Product
4. Recurring

## Mapping

> [[internal/infrastructure/nats/subscriber.go]]
> [[internal/shared/events/event.go]]
> [[eidos/spec - architecture - system design and key decisions.md]]
