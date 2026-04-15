You are expert level golang engineer with hexagonal architecture mindset and very good at UX/UI in dark mode with orange accents

# VVS ISP Manager - Agent Guide

## What This Is

ISP business management system. Single Go binary with embedded SQLite + NATS. Reactive UI via Datastar SSE + Templ + Tailwind CSS v4 CDN. Dark mode with orange accents.

## Architecture

**Hexagonal Architecture** + **DDD** + **CQRS** (single writer, many readers)

```
cmd/server/main.go          # Entry point (urfave/cli v3 flags: --db, --addr)
internal/
  app/                      # Composition root - wires all modules
    app.go                  # Creates repos, handlers, routes, starts server
    config.go               # Config struct
  shared/                   # Shared kernel (DO NOT add module-specific code here)
    domain/                 # Money, CompanyCode, Pagination, errors
    events/                 # DomainEvent, EventPublisher, EventSubscriber interfaces
    cqrs/                   # CommandHandler[C], QueryHandler[Q,R] generic interfaces
  infrastructure/           # Adapters for cross-cutting concerns
    database/               # SQLite setup, WriteSerializer, Goose migrations
    nats/                   # Embedded NATS server, Publisher, Subscriber
    http/                   # Server, middleware, router, dashboard templ
    notifications/          # DB-backed per-user notifications (store, worker, SSE handler)
    chat/                   # DB-backed team chat (store, SSE handler, message history)
    arista/                 # Arista EOS eAPI client (RouterProvisioner)
    mikrotik/               # MikroTik API client (RouterProvisioner)
  modules/                  # Each module is a bounded context
    {module}/
      domain/               # Aggregate root, value objects, repository port (interface)
      app/commands/          # Write-side handlers (use WriteSerializer)
      app/queries/           # Read-side handlers (use reader DB directly)
      adapters/persistence/  # GORM repository implementation
      adapters/http/         # Datastar SSE handlers + templ templates
      adapters/importers/    # (payment only) File import adapters
      migrations/            # Goose SQL migrations (embedded via //go:embed)
specs/                      # SDD specification documents per module
```

## Modules

| Module | Prefix | Key Entity | Notes |
|--------|--------|-----------|-------|
| customer | CLI-00001 | Customer | Unique company codes, atomic sequence |
| product | - | Product | Types: internet/voip/hosting/custom |
| invoice | INV-2026-00001 | Invoice + InvoiceLine | Status: Draft->Finalized->Paid/Void |
| recurring | - | RecurringInvoice | Schedule: Monthly/Quarterly/Yearly on day X |
| payment | - | Payment | SEPA CSV import, invoice matching |
| network | - | Router | RouterType: mikrotik/arista; provisionerDispatcher in app.go picks client at runtime |

## Cross-cutting Infrastructure

| Package | Purpose |
|---------|---------|
| `infrastructure/notifications` | Worker subscribes `isp.>`, creates DB rows for notable events; SSE handler pushes badge+list per user |
| `infrastructure/chat` | Team chat with DB history; SSE streams history then appends new messages in real-time |
| `infrastructure/arista` | Arista EOS eAPI (JSON-RPC over HTTPS); implements `domain.RouterProvisioner` |
| `infrastructure/mikrotik` | MikroTik API; implements `domain.RouterProvisioner` |

**Multi-vendor provisioner pattern (composition root):**
`provisionerDispatcher` in `app.go` implements `RouterProvisioner` and routes calls based on `RouterConn.RouterType`:
```go
func (d *provisionerDispatcher) pick(conn networkdomain.RouterConn) networkdomain.RouterProvisioner {
    if conn.RouterType == networkdomain.RouterTypeArista {
        return d.arista
    }
    return d.mikrotik
}
```
Neither infrastructure package imports the other — routing lives at the composition root only.

## Key Patterns

### CQRS Flow (Single Writer, Many Readers)

**Architecture:** One writer goroutine (WriteSerializer) serializes all DB writes. Many readers use a separate read-only `*gorm.DB` (WAL mode). After a write succeeds, the writer publishes a NATS event. Read-side SSE handlers subscribe to NATS and re-render when notified.

**Write path:**
```
HTTP POST/PUT/DELETE -> ReadSignals -> Command -> CommandHandler -> WriteSerializer -> SQLite -> Publish NATS event
```

**Read path (initial load):**
```
HTTP GET (data-init="@get(...)") -> SSE connection opens -> QueryHandler -> Reader DB -> templ -> PatchElementTempl
```

**Read path (live update via NATS):**
```
NATS event arrives -> SSE handler's subscription fires -> QueryHandler -> Reader DB -> templ -> PatchElementTempl
```
The SSE handler keeps the connection open, subscribes to relevant NATS subjects, and re-renders the read model whenever a write-side event arrives. This is how the UI stays reactive without polling.

**Full round-trip example:**
```
1. User clicks "Create Customer" -> @post('/api/customers')
2. Handler reads signals, builds CreateCustomerCommand
3. CommandHandler validates, calls WriteSerializer.Exec(func(tx) { ... })
4. WriteSerializer executes in single goroutine -> SQLite write
5. CommandHandler publishes event: nats.Publish("isp.customer.created", payload)
6. Meanwhile, another user has the customer list open (SSE connection from data-init)
7. That SSE handler has nats.Subscribe("isp.customer.*", callback)
8. Callback fires -> re-queries customer list from Reader DB -> PatchElementTempl
9. Datastar morphs the customer table in the browser
```

**SSE handler pattern (subscribe to NATS for live updates):**
```go
func listHandler(querySvc *queries.ListHandler, sub events.EventSubscriber) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        sse := datastar.NewSSE(w, r)
        // 1. Initial render
        result, _ := querySvc.Handle(r.Context(), query)
        sse.PatchElementTempl(ListTemplate(result))
        // 2. Subscribe to NATS for live updates — ChanSubscription returns (chan DomainEvent, cancelFunc)
        ch, cancel := sub.ChanSubscription("isp.customer.*")
        defer cancel()
        for {
            select {
            case <-r.Context().Done():
                return
            case event, ok := <-ch:
                if !ok { return }
                result, _ = querySvc.Handle(r.Context(), query)
                sse.PatchElementTempl(ListTemplate(result))
            }
        }
    }
}
```

**SSE live updates — always re-render the full component:**
Datastar is backend-driven UI. The browser holds no state — it renders whatever HTML the backend sends.
On any NATS event (create, update, delete), re-query the current state and send the full component HTML.
Datastar's morphing algorithm reconciles the DOM: adds new rows, removes deleted ones, updates changed ones.

```go
// CORRECT: re-render full list on every event
case _, ok := <-ch:
    if !ok { return }
    result, err = querySvc.Handle(r.Context(), query)
    if err != nil { continue }
    sse.PatchElementTempl(ListTemplate(result))
```

❌ **WRONG: selector-based remove**
```go
// DON'T do this — if element is absent from DOM, Datastar fails;
// also violates "backend is source of truth" principle
sse.PatchElements("",
    datastar.WithSelector("#item-"+event.AggregateID),
    datastar.WithMode(datastar.ElementPatchModeRemove),
)
```

**`PatchElements` append — only for append-only streams (e.g. chat history):**
```go
// OK for chat/log — content only grows, re-sending full history is expensive
var buf bytes.Buffer
MyItemTempl(item).Render(ctx, &buf)
sse.PatchElements(buf.String(),
    datastar.WithSelector("#container-id"),
    datastar.WithMode(datastar.ElementPatchModeAppend),
)
```

**`ExecuteScript` for imperative DOM ops:**
```go
sse.ExecuteScript(`(function(){var el=document.getElementById('x');if(el)el.scrollTop=el.scrollHeight})()`)
```

### WriteSerializer
ALL writes go through `database.WriteSerializer` (channel-based single goroutine). This avoids SQLite locking. Never write to the DB outside the WriteSerializer — it will cause "database is locked" errors. Readers use a separate `*gorm.DB` in WAL read-only mode and can read concurrently.

### NATS Events
Subject format: `isp.{module}.{event}` (e.g., `isp.customer.created`, `isp.invoice.paid`)
Events are published AFTER successful write, inside the command handler. Subscribers react asynchronously.
The write side MUST publish an event after every mutation — this is how the read side knows to re-render. If you add a new command and forget to publish, the UI won't update in real-time.

### Templ Templates
All HTML is rendered via templ components. Generate with `templ generate ./internal/...`.
Layout: `internal/infrastructure/http/templates/layout.templ`
Components: `internal/infrastructure/http/templates/components.templ`

### Datastar v1 RC.8 (IMPORTANT — read carefully)

**Go SDK:** `github.com/starfederation/datastar-go/datastar` (NOT the old `datastar/sdk/go`)

**Attribute syntax — v1 uses COLONS, not hyphens:**
```
data-on:click        ✅ correct (colon separates plugin from key)
data-on-click        ❌ wrong (old v0.x syntax)
data-bind:fieldName  ✅ correct
data-bind-fieldName  ❌ wrong
data-on:submit__prevent  ✅ modifiers use double underscore
data-on:input__debounce.500ms  ✅ modifier tags use dot
```

**Element initialization — use `data-init`, NOT `data-on:load`:**
```html
data-init="@get('/api/items')"     ✅ fires when element enters DOM
data-on:load="@get('/api/items')"  ❌ "load" is a DOM event, only fires on window/img/script
```
`data-init` runs its expression on page load OR when the element is patched into the DOM.

**SSE flow (how Datastar talks to the backend):**
1. `data-init="@get('/api/...')"` — opens SSE connection on element init
2. `data-on:submit__prevent="@post('/api/...')"` — sends signals as JSON POST
3. `data-on:click="@delete('/api/items/123')"` — action on click
4. ALL `@get`/`@post`/`@put`/`@delete` requests send the full signal state as JSON
5. Server reads signals with `datastar.ReadSignals(r, &struct{})` and unmarshals into a Go struct

**Server-side response methods:**
```go
sse := datastar.NewSSE(w, r)
sse.PatchElementTempl(component)    // morph DOM element by matching id=""
sse.Redirect("/path")               // client-side redirect
sse.ConsoleError(err)               // log error to browser console
```

**Signals and forms (the Datastar way):**
- Define signals: `data-signals="{name:'',email:'',page:1}"`
- Bind inputs: `data-bind:name` (binds to signal "name")
- On submit, `@post('/api/...')` sends ALL signals as JSON body
- Backend parses with `ReadSignals(r, &signals)` — struct tags map to signal names
- No `<form>` name/value attributes needed — signals ARE the form state

**Long-lived SSE (e.g., clock, live feed):**
```go
func handler(w http.ResponseWriter, r *http.Request) {
    sse := datastar.NewSSE(w, r)
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-r.Context().Done():
            return
        case t := <-ticker.C:
            if sse.IsClosed() { return }
            sse.PatchElementTempl(MyFragment(t.Format("15:04:05")))
        }
    }
}
```
Datastar morphs DOM by element `id`. The fragment templ must render an element with a stable `id`.

**Chaining expressions in data-on — use `&&`, NEVER `;`:**
```html
data-on:click="@post('/api/x') && ($signal='')"         ✅ correct
data-on:click="@post('/api/x'); $signal=''"             ❌ wrong — Datastar uses regexp on &&, semicolons break parsing
```
For signal assignment before action, wrap in parens to avoid precedence issues:
```html
data-on:click="($page=1) && @get('/api/items')"         ✅
```

**DOM event object is `evt`, NOT `$event`:**
```html
data-on:keydown="evt.key==='Enter' && ..."    ✅
data-on:keydown="$event.key==='Enter' && ..."  ❌ — $event does not exist; $x is only for signals
```
Signals use `$signalName`. The native DOM event is always `evt`.

**`evt.preventDefault()` in a `&&` chain:**
`preventDefault()` returns `undefined` (falsy), which would short-circuit the chain.
Use negation: `!evt.preventDefault()` evaluates to `true`, so the chain continues:
```html
data-on:keydown="evt.key==='Enter' && !evt.shiftKey && !evt.preventDefault() && @post('/api/x') && ($msg='')"
```

**Signal names for `data-bind` must be lowercase:**
HTML attribute names are lowercased by browsers. `data-bind:chatMsg` becomes `data-bind:chatmsg` in the DOM,
so Datastar binds to signal `chatmsg`, not `chatMsg`. If you define `data-signals="{chatMsg:''}"` (camelCase)
and bind with `data-bind:chatMsg`, they refer to different signals — clearing `$chatMsg` won't update the input.
**Rule:** use all-lowercase signal names for any signal you intend to bind with `data-bind:`.

**`data-init` — `el` refers to the current element:**
```html
<div data-init="el.scrollIntoView()"></div>    ✅ — el is the element itself
<div data-init="@get('/sse/chat')"></div>       ✅ — opens SSE stream on init
```

**Common mistakes to avoid:**
- ❌ `NewSSE` before `ReadSignals` — `NewSSE` closes the request body; always call `ReadSignals` first, then `NewSSE`. Use `http.Error` for errors before SSE is created:
  ```go
  // CORRECT order:
  var signals struct { Name string `json:"name"` }
  if err := datastar.ReadSignals(r, &signals); err != nil {
      http.Error(w, err.Error(), http.StatusBadRequest)
      return
  }
  sse := datastar.NewSSE(w, r) // always AFTER ReadSignals
  ```
- ❌ `data-on-load` → use `data-init` instead
- ❌ `data-bind-x` → use `data-bind:x` (colon)
- ❌ `MergeFragmentTempl()` → renamed to `PatchElementTempl()` in new SDK
- ❌ `datastar/sdk/go` → use `datastar-go/datastar`
- ❌ Using `r.PathValue("id")` → use `chi.URLParam(r, "id")` (we use chi router)

## How to Add a New Module

1. Create directory structure: `internal/modules/{name}/{domain,app/commands,app/queries,adapters/persistence,adapters/http,migrations}`
2. **Domain first** (TDD): Write `domain/{name}.go` with aggregate, `domain/{name}_test.go`, `domain/repository.go` interface
3. **Commands**: Write handlers in `app/commands/`, each publishes NATS event after write
4. **Queries**: Write handlers in `app/queries/` with read models (GORM structs with `TableName()`)
5. **Persistence**: GORM models in `adapters/persistence/models.go`, repository in `gorm_repository.go`
6. **Migration**: SQL in `migrations/001_create_{name}.sql` with goose Up/Down, embed.go
7. **HTTP**: Handlers in `adapters/http/handlers.go`, templ in `adapters/http/templates.templ`
8. **Wire**: Add to `internal/app/app.go` - create repo, handlers, routes; add migration entry

## Tech Stack

| Component | Package |
|-----------|---------|
| Database | `github.com/glebarez/sqlite` (no CGo) + `gorm.io/gorm` |
| Migrations | `github.com/pressly/goose/v3` (per-module version tables) |
| Frontend | `github.com/starfederation/datastar-go/datastar` (SSE reactivity) |
| Router | `github.com/go-chi/chi/v5` |
| Templates | `github.com/a-h/templ` (type-safe HTML) |
| CSS | Tailwind CSS v4 CDN |
| Events | `github.com/nats-io/nats-server/v2` (embedded) + `nats.go` |
| CLI | `github.com/urfave/cli/v3` |
| IDs | `github.com/google/uuid` (UUIDv7) |
| Testing | `github.com/stretchr/testify` |

## Commands

```bash
make build          # Build single binary -> bin/vvs
make run            # Run with defaults (./data/vvs.db, :8080)
make test           # Run all tests
make test-unit      # Domain tests only (fast)
make generate       # Regenerate templ files
./bin/vvs --db ./data/vvs.db --addr :8080  # Run with flags
```

## Conventions

- Money is `int64` cents + currency string. Never use floats for money.
- Customer codes: `CLI-{NNNNN}`, Invoice numbers: `INV-{YEAR}-{NNNNN}`
- All timestamps in UTC
- Domain aggregates are pure Go structs with behavior methods
- GORM models are separate structs in `adapters/persistence/models.go` with `toModel`/`toDomain` mappers
- HTTP handlers read Datastar signals, construct commands/queries, call handlers, render templ
- Templ files must be regenerated after changes: `templ generate ./internal/...`
- Each module's migrations use a separate goose version table: `goose_{module}`

## Parallel Development

Modules are fully independent bounded contexts. Multiple agents can work on different modules simultaneously. The only shared touchpoint is `internal/app/app.go` (composition root) which should be updated last.
