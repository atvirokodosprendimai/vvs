---
tldr: Customer portal VM self-service — VM plan catalog, Stripe checkout, balance top-up, VM provisioned on payment webhook
status: active
---

# Plan: Portal VM Self-Service with Stripe and Balance

## Context

- Spec: [[spec - portal - customer self-service access]] — existing portal auth, NATS RPC bridge, two-binary split
- Spec: [[spec - architecture - system design and key decisions]] — hexagonal arch, NATS RPC, no DB in portal binary
- Related module: `internal/modules/proxmox/` — VM domain + CreateVM command (just built)
- Related module: `internal/modules/portal/` — portal binary, NATS bridge, existing invoice/ticket/service pages

### Architecture decisions baked in

The portal binary (`cmd/portal`) is public-facing with no direct DB access. All writes go via NATS RPC to core. Stripe is wired into the portal binary because:
- Portal is already internet-facing (Stripe webhooks need a public endpoint)
- Stripe SDK needs no DB — just API keys and HTTP
- Core receives NATS events from portal after webhook validation → executes DB writes

**Note:** No spec exists yet for the billing/Stripe domain. Consider `/eidos:spec portal - vm self-service billing with stripe` before implementing.

### Two Stripe flows

1. **VM purchase** — customer selects a VM plan → Stripe Checkout → webhook → core provisions VM
2. **Balance top-up** — customer adds credit → Stripe Checkout → webhook → core credits balance

Balance shortcut: if customer balance ≥ plan price → deduct immediately, skip Stripe entirely.

---

## Phases

### Phase 1 — VM Plan catalog (admin) — status: open

Admin-configurable preset VM plans: name, description, specs (cores/RAM/disk), monthly price, Proxmox node + template.

1. [ ] Domain: `VMPlan` entity + repository port
   - fields: id, name, description, cores int, memoryMB int, diskGB int, storage string, templateVMID int, nodeID string, priceMonthlyEuroCents int64, enabled bool, notes string
   - `NewVMPlan(...)` with validation (cores>0, memory>0, disk>0, price≥0, templateVMID>0)
   - `VMPlanRepository`: Save, FindByID, FindAll, FindEnabled, Delete
   - file: `internal/modules/proxmox/domain/vm_plan.go`

2. [ ] Migration: `proxmox/migrations/003_create_vm_plans.sql`
   - `vm_plans` table with all domain fields, `enabled BOOLEAN DEFAULT 1`

3. [ ] Persistence: `GormVMPlanRepository`
   - file: `internal/modules/proxmox/adapters/persistence/gorm_vm_plan_repository.go`
   - `VMPlanModel` in `models.go`

4. [ ] NATS subjects: add `ProxmoxVMPlanAll`, `ProxmoxVMPlanCreated`, `ProxmoxVMPlanUpdated`, `ProxmoxVMPlanDeleted`
   - file: `internal/shared/events/subjects.go`

5. [ ] Commands: CreateVMPlan, UpdateVMPlan, DeleteVMPlan
   - file: `internal/modules/proxmox/app/commands/vm_plan_commands.go`
   - publish `ProxmoxVMPlanAll` on each mutation

6. [ ] Queries: `VMPlanReadModel`, ListVMPlansHandler, GetVMPlanHandler
   - add to `internal/modules/proxmox/app/queries/`

7. [ ] Admin HTTP: VM plan CRUD
   - routes: GET/POST `/proxmox/plans`, `/proxmox/plans/new`, GET/PUT `/proxmox/plans/{id}/edit`, DELETE `/proxmox/plans/{id}`
   - templ: PlanListPage, PlanTable, PlanRow, PlanFormPage, PlanForm, PlanFormError
   - add to `proxmox/adapters/http/handlers.go` + `RegisterRoutes`

8. [ ] Wire: inject `VMPlanRepository` + plan commands/queries into `wire_proxmox.go` and `proxmox/adapters/http/handlers.go`

### Phase 2 — Customer balance ledger — status: open

Prepaid credit per customer. Cached balance + append-only ledger for audit. New `billing` module.

1. [ ] Billing module skeleton
   - create `internal/modules/billing/domain/`, `app/commands/`, `app/queries/`, `adapters/persistence/`, `migrations/`
   - embed.go for migrations

2. [ ] Domain: `BalanceLedgerEntry` + balance operations
   - file: `internal/modules/billing/domain/balance.go`
   - entry types const: `EntryTypeTopUp`, `EntryTypeVMPurchase`, `EntryTypeRefund`, `EntryTypeAdjustment`
   - `BalanceRepository` port: `GetBalance`, `Credit`, `Deduct`

3. [ ] Migration: `billing/migrations/001_create_balance.sql`
   - `customer_balance(customer_id PK, balance_cents INT NOT NULL DEFAULT 0, updated_at)`
   - `balance_ledger(id, customer_id, type, amount_cents, description, stripe_session_id, created_at)`
   - index on `balance_ledger(customer_id)`

4. [ ] Persistence: `GormBalanceRepository`
   - `Credit` — atomic: INSERT ledger + UPDATE/INSERT balance in one WriteTX
   - `Deduct` — atomic: check balance ≥ amount, deduct, insert ledger; return `ErrInsufficientBalance` if not enough
   - file: `internal/modules/billing/adapters/persistence/gorm_balance_repository.go`

5. [ ] Commands: `TopUpBalanceCommand`, `DeductBalanceCommand`
   - file: `internal/modules/billing/app/commands/balance_commands.go`
   - TopUp: Credit + publish `BillingBalanceCredited` NATS event
   - Deduct: Deduct + publish `BillingBalanceDebited` (or return `ErrInsufficientBalance`)

6. [ ] NATS subjects: `BillingBalanceCredited`, `BillingBalanceDebited` in `subjects.go`

7. [ ] Query: `GetCustomerBalanceHandler`
   - file: `internal/modules/billing/app/queries/balance_queries.go`

8. [ ] Admin UI: balance display on customer detail left-column card
   - inject `GetCustomerBalanceHandler` into customer handlers (same `With*Query` pattern)
   - show balance in customer detail page with manual-adjust button (admin only)
   - POST `/api/customers/{id}/balance/adjust` with `amountCents + description` signals

9. [ ] Wire: `internal/app/wire_billing.go`
   - `billingWired{balanceRepo, topUpCmd, deductCmd, getBalanceQuery}`
   - add to `builder.go` module chain + `allMigrations()`

### Phase 3 — Stripe infrastructure — status: open

Stripe Go SDK + adapter. Used by both core (webhook validation, session creation on behalf of portal) and portal binary.

1. [ ] Add Stripe dependency: `go get github.com/stripe/stripe-go/v82`

2. [ ] StripeClient adapter
   - file: `internal/infrastructure/stripe/client.go`
   - `type Client struct { secretKey, webhookSecret string }`
   - `New(secretKey, webhookSecret string) *Client`
   - `CreateCheckoutSession(ctx, params CheckoutParams) (sessionID, url string, err error)`
     - `CheckoutParams{Mode, SuccessURL, CancelURL string, LineItems []LineItem, Metadata map[string]string, CustomerEmail string}`
     - use `stripe.CheckoutSessionNew` with `payment_method_types: ["card"]`
   - `ConstructEvent(payload []byte, sigHeader string) (stripe.Event, error)` — validates webhook signature

3. [ ] Config additions
   - `internal/app/config.go`: `StripeSecretKey`, `StripeWebhookSecret`, `StripePublishableKey`
   - `cmd/server/main.go`: `--stripe-*` CLI flags + `VVS_STRIPE_*` env vars
   - `cmd/portal/main.go`: same 3 flags (portal creates sessions + validates webhooks)

### Phase 4 — NATS RPC bridge extensions — status: open

New subjects so portal binary can request VM plans, balance info, and trigger provisioning/top-up on core.

1. [ ] New subjects in `portal/adapters/nats/bridge.go`
   ```
   SubjectVMPlansList          = "isp.portal.rpc.vm.plans.list"
   SubjectVMsList              = "isp.portal.rpc.vms.list"          // customer's VMs
   SubjectVMProvision          = "isp.portal.rpc.vm.provision"      // post-webhook
   SubjectBalanceGet           = "isp.portal.rpc.balance.get"
   SubjectBalanceTopupComplete = "isp.portal.rpc.balance.topup"     // post-webhook
   SubjectBalanceDeductVM      = "isp.portal.rpc.balance.deduct.vm" // balance-shortcut buy
   ```

2. [ ] Core PortalBridge handlers (new subscriptions in `bridge.go`)
   - `handleVMPlansList` → ListVMPlansHandler (enabled=true only)
   - `handleVMsList(customerID)` → ListVMsForCustomerHandler
   - `handleVMProvision(customerID, planID, stripeSessionID)`:
     1. GetVMPlan → validate enabled
     2. Credit check for duplicate (ledger lookup by stripeSessionID — idempotent)
     3. Call `CreateVMHandler.Handle` with plan specs
     4. Return `{vmID}`
   - `handleBalanceGet(customerID)` → GetCustomerBalanceHandler
   - `handleBalanceTopupComplete(customerID, amountCents, stripeSessionID)` → TopUpBalanceCommand (idempotent via stripeSessionID)
   - `handleBalanceDeductVM(customerID, planID)` → DeductBalance + CreateVM (balance-path purchase)

3. [ ] Portal NATS client extensions (`portal/adapters/nats/client.go`)
   - `ListVMPlans(ctx) ([]VMPlanDTO, error)`
   - `ListCustomerVMs(ctx, customerID) ([]VMDTO, error)`
   - `GetBalance(ctx, customerID) (balanceCents int64, error)`
   - `ProvisionVM(ctx, customerID, planID, stripeSessionID string) (vmID string, error)`
   - `CompleteBalanceTopup(ctx, customerID string, amountCents int64, stripeSessionID string) error`
   - `BuyVMWithBalance(ctx, customerID, planID string) (vmID string, error)`

### Phase 5 — Stripe webhook handler (portal binary) — status: open

Portal receives and validates Stripe webhooks, dispatches to core.

1. [ ] Webhook endpoint in portal handlers
   - `POST /stripe/webhook`
   - Read raw body BEFORE any other parsing (critical for Stripe signature validation)
   - `stripeClient.ConstructEvent(body, r.Header.Get("Stripe-Signature"))` → `stripe.Event`
   - Switch `event.Type = "checkout.session.completed"`:
     - Unmarshal `CheckoutSession`, read `Metadata["type"]`
     - `"vm_purchase"` → `natsClient.ProvisionVM(customerID, planID, session.ID)`
     - `"balance_topup"` → parse `Metadata["amountCents"]` → `natsClient.CompleteBalanceTopup(...)`
   - Respond 200 immediately; errors are logged (Stripe retries)

2. [ ] Route registered in `RegisterPublicRoutes` (before cookie auth middleware)

### Phase 6 — Portal VM purchase pages — status: open

1. [ ] Portal page: GET `/portal/plans`
   - `natsClient.ListVMPlans` → render plan cards
   - templ: `PortalPlansPage`, `VMPlanCard(plan VMPlanDTO, customerID string)`
   - Plan card shows: name, specs, price/month, "Buy with Stripe" + "Buy from balance (€X)" buttons if balance ≥ price

2. [ ] Checkout initiation: POST `/portal/checkout/vm`
   - Signal: `{planId string}`
   - Check balance via `natsClient.GetBalance`
   - If balance ≥ plan price → `natsClient.BuyVMWithBalance` → redirect `/portal/vms?provisioning=1`
   - Else → `stripeClient.CreateCheckoutSession` with metadata `{type:vm_purchase, customerId, planId}` → SSE redirect to Stripe URL

3. [ ] Portal pages: GET `/portal/checkout/success` and `/portal/checkout/cancel`
   - Success: "Payment received! Your VM is being provisioned." + link to `/portal/vms`
   - Cancel: "Checkout cancelled." + link to `/portal/plans`

4. [ ] Portal page: GET `/portal/vms`
   - `natsClient.ListCustomerVMs` → render VM table
   - templ: `PortalVMsPage`, `PortalVMTable(vms []VMDTO)`
   - Columns: name, status badge, IP, specs (cores/RAM/disk)

### Phase 7 — Portal balance pages — status: open

1. [ ] Portal page: GET `/portal/balance`
   - Shows balance in EUR (`balance_cents / 100`)
   - Preset top-up buttons: €5 / €10 / €20 / €50, custom input
   - POST `/portal/balance/topup` on submit
   - templ: `PortalBalancePage`

2. [ ] Top-up initiation: POST `/portal/balance/topup`
   - Signal: `{amountCents int}`
   - Validate: amountCents ∈ [100, 100_000] (€1–€1000)
   - `stripeClient.CreateCheckoutSession` with metadata `{type:balance_topup, customerId, amountCents}` → redirect to Stripe URL

### Phase 8 — Portal nav + final wiring — status: open

1. [ ] Portal nav links: "My VMs" (`/portal/vms`), "Plans" (`/portal/plans`), "Balance" (`/portal/balance`)
   - Update portal layout template nav section

2. [ ] Portal binary wiring: `cmd/portal/main.go`
   - Parse Stripe flags → `stripeinfra.New(secretKey, webhookSecret)`
   - Pass `stripeClient` to portal HTTP handlers
   - Pass `stripePublishableKey` to handlers (needed for JS Stripe embed, if any)

3. [ ] Core binary wiring
   - `wire_billing.go` billing module into `PortalBridge` (inject topUpCmd, deductCmd, getBalanceQuery)
   - `wire_proxmox.go` inject `listVMPlansQuery` + `createVMCmd` into bridge

4. [ ] Integration smoke test (manual)
   - Admin creates VM plan at `/proxmox/plans/new`
   - Portal: `/portal/plans` shows the plan
   - Test Stripe CLI webhook: `stripe trigger checkout.session.completed` with metadata → VM appears in proxmox_vms
   - Portal balance top-up flow end to end

---

## Verification

- [ ] `go test ./internal/modules/billing/...` — balance Credit/Deduct/idempotency
- [ ] Admin creates VM plan; plan appears in `/proxmox/plans` table
- [ ] Portal `/portal/plans` lists only enabled plans
- [ ] Stripe Checkout Session created with correct metadata
- [ ] Stripe webhook `checkout.session.completed` → VM row in `proxmox_vms` with `status=creating`
- [ ] Duplicate webhook for same `stripe_session_id` is idempotent (no double VM)
- [ ] Customer balance top-up: `balance_cents` increases by correct amount
- [ ] Balance purchase path: no Stripe involved, balance deducted, VM created
- [ ] `/portal/vms` shows customer's VMs (via NATS)
- [ ] `/portal/balance` shows correct balance

## Adjustments

<!-- none yet -->

## Progress Log

<!-- Updated after every completed action -->
