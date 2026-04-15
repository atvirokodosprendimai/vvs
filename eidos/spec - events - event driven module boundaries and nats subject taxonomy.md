---
tldr: Modules communicate exclusively via NATS events; no cross-module package imports; defines all canonical subjects and payloads
---

# Event-Driven Module Boundaries

Each module is a bounded context. The only allowed cross-module communication mechanism is NATS publish/subscribe. Modules must not call into each other's domain, command, or query layers directly.

## Target

Enforce bounded-context isolation so modules can be:
- developed, tested, and deployed independently
- selectively enabled via `VVS_MODULES` without code changes
- run in separate processes connected to an external NATS server

## Behaviour

### Import rules

- A package in `internal/modules/X` MUST NOT import any package from `internal/modules/Y`
- Allowed imports: `internal/shared/`, `internal/infrastructure/`
- **Exception**: `internal/app/app.go` is the sole composition root — it imports all modules for wiring and is the only file where cross-module references are allowed

### Cross-module reads

- Modules may read from SQLite tables owned by another module using the shared reader `*gorm.DB`
- This is done with raw SQL or GORM against the reader directly — not by calling the other module's query handlers
- Example: customer form needs a router list → reads the `routers` table directly, no import of `network/app/queries`

### Cross-module write side effects

- When a write in module X should trigger a side effect in module Y, module X publishes a NATS event
- Module Y owns a subscriber worker that consumes the event and executes the side effect
- The worker lives in `internal/modules/Y/app/subscribers/` and is registered during module Y's startup

### Subscriber workers

- A worker is a long-running goroutine started once per process lifetime
- It subscribes to one or more NATS subjects and dispatches to command handlers within its own module
- Workers are started only when their module is enabled
- All subscription channels must be cancelled on shutdown (`defer cancel()`)

## Design

### NATS Subject Taxonomy

Pattern: `isp.{module}.{verb}` where verb is past tense for domain facts, present tense for requests.

#### Customer module — publishes

| Subject | Trigger | Data payload |
|---------|---------|--------------|
| `isp.customer.created` | CreateCustomer command | `CustomerPayload` |
| `isp.customer.updated` | UpdateCustomer command | `CustomerPayload` |
| `isp.customer.deleted` | DeleteCustomer command | `CustomerPayload` |

`CustomerPayload`:
```json
{
  "id": "uuid",
  "code": "CLI-00001",
  "name": "Acme Corp",
  "status": "active|suspended|churned",
  "router_id": "uuid|null",
  "ip_address": "10.0.0.1",
  "mac_address": "aa:bb:cc:dd:ee:ff"
}
```

#### Product module — publishes

| Subject | Trigger | Data payload |
|---------|---------|--------------|
| `isp.product.created` | CreateProduct command | `ProductPayload` |
| `isp.product.updated` | UpdateProduct command | `ProductPayload` |
| `isp.product.deleted` | DeleteProduct command | `ProductPayload` |

#### Network module — publishes

| Subject | Trigger | Data payload |
|---------|---------|--------------|
| `isp.network.router.created` | CreateRouter command | `RouterPayload` |
| `isp.network.router.updated` | UpdateRouter command | `RouterPayload` |
| `isp.network.arp_changed` | SyncCustomerARP command | `ARPChangedPayload` |

`ARPChangedPayload`:
```json
{
  "customer_id": "uuid",
  "action": "enable|disable",
  "ip_address": "10.0.0.1",
  "success": true
}
```

#### Network module — subscribes

| Subject | Published by | Handler |
|---------|-------------|---------|
| `isp.customer.*` | customer module | auto-sync ARP on status change |
| `isp.network.arp_requested` | customer UI handler | manual ARP enable/disable |

`ARPRequestedPayload`:
```json
{
  "customer_id": "uuid",
  "action": "enable|disable"
}
```

**Rule**: `isp.network.arp_requested` replaces the direct `SyncCustomerARPHandler` injection into customer HTTP handlers. The customer ARP endpoint publishes this event; the network module's ARPWorker handles it.

#### Invoice module — planned

| Subject | Trigger |
|---------|---------|
| `isp.invoice.created` | CreateInvoice |
| `isp.invoice.finalized` | FinalizeInvoice |
| `isp.invoice.paid` | MarkInvoicePaid |
| `isp.invoice.voided` | VoidInvoice |

#### Recurring module — planned

| Subject | Trigger |
|---------|---------|
| `isp.recurring.invoice_generated` | Scheduler generates invoice |

#### Payment module — planned

| Subject | Trigger |
|---------|---------|
| `isp.payment.imported` | ImportPayments batch |
| `isp.payment.matched` | MatchPayment to invoice |

### Module interface

Each module exposes a `Register` function that wires its internals and is called from `app.go` only when the module is enabled:

```go
// internal/app/module.go
type AppDeps struct {
    DB, Reader *gorm.DB
    Writer     *database.WriteSerializer
    Publisher  events.EventPublisher
    Subscriber events.EventSubscriber
    Router     *infrahttp.Router
    Config     Config
}

type Module interface {
    Name() string
    Register(ctx context.Context, deps AppDeps) error
}
```

Module `Register` is responsible for:
1. Wiring its repository, commands, and queries from `AppDeps`
2. Mounting HTTP routes onto `deps.Router`
3. Starting subscriber workers as goroutines

### app.go role

`app.go` is the composition root:
- Builds `AppDeps` (DB, NATS, Writer, Router)
- Runs migrations for all modules (schema consistency regardless of enable flag)
- Calls `m.Register(ctx, deps)` for each enabled module
- Contains no business logic, no event subscriptions, no domain knowledge

## Verification

- `grep -rn "modules/" internal/modules/` shows no cross-module imports (only `shared/` and `infrastructure/`)
- `grep -rn "modules/" internal/app/app.go` is the only file with cross-module imports
- Network ARP worker subscribes to both `isp.customer.*` and `isp.network.arp_requested`
- Customer HTTP handlers contain no imports from `internal/modules/network/`
- `go test ./... -race` passes

## Friction

- Cross-module reads via raw SQL on the shared reader bypass the type system — callers must know the table schema. Acceptable trade-off for now; if schemas drift, a shared read model type can be extracted to `internal/shared/`.
- `isp.network.arp_requested` is fire-and-forget — the customer UI has no way to surface ARP errors synchronously. Error visibility requires a follow-up `isp.network.arp_changed` event with `success: false` and a toast mechanism.

## Interactions

- Depends on [[spec - architecture - system design and key decisions]]
- Enables [[plan - 2604151142 - event driven nats ipc and module isolation flags]]

## Mapping

> [[internal/app/app.go]]
> [[internal/shared/events/event.go]]
> [[internal/infrastructure/nats/subscriber.go]]
> [[internal/infrastructure/nats/publisher.go]]
> [[internal/modules/network/app/commands/sync_customer_arp.go]]
