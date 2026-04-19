---
tldr: Portal shows customer's active services, status, billing cycle, next billing date, and network info — answers "what am I paying for?"
status: completed
---

# Plan: Customer Portal — Service Status + Usage Summary

## Context

Consilium 2026-04-19 priority #12. Portal today: invoices + PDF only. Customers cannot see their active services, IP address, or when they'll be billed next — driving inbound support calls.

**Available data:**
- `Service{ID, CustomerID, ProductName, PriceAmount, Status, BillingCycle, NextBillingDate}` — active/suspended/cancelled
- `Customer{IPAddress, MACAddress, RouterID, NetworkZone}` — connection info (already fetched in `SubjectCustomerGet`)
- `servicedomain.ListForCustomer(ctx, customerID)` → `[]*Service`
- Bridge pattern: `isp.portal.rpc.*` subjects in `bridge.go` + `client.go`

Scope: **read-only** service data. No service management (suspend/cancel) from portal — staff-only operations.

- Consilium backlog: [[project_consilium_backlog_priority]] — #12

## Phases

### Phase 1 — Bridge: services list subject (core side) — status: open

1. [ ] Add subject constant to `bridge.go`
   - `SubjectServicesList = "isp.portal.rpc.services.list"`

2. [ ] Add service lister interface + field to `PortalBridge`
   - `serviceLister bridgeServiceLister` — minimal interface: `ListForCustomer(ctx, customerID) ([]*servicedomain.Service, error)`
   - Or use a thin `PortalService` read struct to avoid leaking domain types over wire

3. [ ] Implement `handleServicesList`
   - req: `{customerID}` — mandatory, `errForbidden` if empty
   - calls `serviceLister.ListForCustomer(ctx, req.CustomerID)`
   - returns list of `PortalService{ID, ProductName, PriceAmountCents, Status, BillingCycle, NextBillingDate}`
   - include suspended services (customer should see why they have no connectivity)

4. [ ] Register handler in `PortalBridge.Register()`

5. [ ] Unit test in `bridge_test.go`
   - returns only that customer's services
   - empty customerID → forbidden
   - includes suspended services

### Phase 2 — Also extend `SubjectCustomerGet` with network info — status: open

1. [ ] Extend `BridgeCustomer` struct to include connection fields
   - add `IPAddress`, `NetworkZone` to `BridgeCustomer` (already partially returned)
   - check what `handleCustomerGet` currently returns — add IP/zone if missing

2. [ ] Update `PortalNATSClient.GetCustomer` to decode new fields

### Phase 3 — Client: `PortalNATSClient.ListServices` — status: open

1. [ ] Add `PortalService` struct to `client.go`
   ```go
   type PortalService struct {
       ID              string
       ProductName     string
       PriceAmountCents int64
       Status          string
       BillingCycle    string
       NextBillingDate *time.Time
   }
   ```

2. [ ] Add `ListServices(ctx context.Context, customerID string) ([]PortalService, error)` to `PortalNATSClient`

3. [ ] Unit test: happy path + empty list + error propagation

### Phase 4 — Portal HTTP + Template — status: open

1. [ ] Add services page route to portal HTTP handlers
   - `GET /portal/services` → list services for authenticated customer
   - fetch services + customer (for IP/zone)

2. [ ] Create `portal_services.templ`
   - **Active services** section: card per service showing product name, price/cycle, next billing date
   - **Connection info** section: IP address, network zone (only if IP is non-empty)
   - **Suspended services**: amber badge, reason placeholder ("contact support to reactivate")
   - Empty state: "No active services — contact us to get started"
   - Status badge colors: active=green, suspended=amber, cancelled=neutral

3. [ ] Add "Services" nav link to portal layout (alongside Invoices and Support)

### Phase 5 — Wiring — status: open

1. [ ] Pass `serviceRepo` to `PortalBridge` in `wire_infra.go`
   - `serviceRepo` is available from `wire_service.go`'s `serviceWired.repo`

2. [ ] Pass `PortalNATSClient.ListServices` to portal HTTP handlers in `cmd/portal/main.go`

3. [ ] `go build ./... && go test ./internal/modules/portal/...`

## Verification

```bash
go test ./internal/modules/portal/adapters/nats/... -v
go build ./cmd/portal/ ./cmd/vvs-core/
# Log in to portal as customer
# /portal/services → see active service(s) with product name, price, next billing date
# See IP address and network zone if set on customer
# Suspended service → shows amber badge
# No services → shows empty state
```

## Adjustments

## Progress Log
