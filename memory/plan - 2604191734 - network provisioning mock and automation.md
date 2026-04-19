---
tldr: FakeRouterProvisioner mock unblocks tests; wire service suspension → ARP disable automation
status: active
---

# Plan: Network Provisioning Mock + Automation

## Context

Consilium 2026-04-19 priority #4. Largest operational gap — new subscriber activations and service suspensions require manual router access today. QA blocked: no mock `RouterProvisioner` for tests.

- Existing domain: `networkdomain.RouterProvisioner` interface (SetARPStatic, DisableARP, GetARPEntry)
- Existing: `provisionerDispatcher` in `internal/app/app.go` dispatches MikroTik vs Arista
- Existing: MikroTik adapter (`internal/infrastructure/mikrotik/`), Arista client (`internal/infrastructure/arista/`)
- Service suspension already fires `isp.service.suspended` NATS event — domain side ready, provisioner never called
- Consilium backlog: [[project_consilium_backlog_priority]] — #4

## Phases

### Phase 1 — FakeRouterProvisioner Mock — status: open

1. [ ] Create `FakeRouterProvisioner` in testutil
   - file: `internal/testutil/fake_provisioner.go`
   - implements `networkdomain.RouterProvisioner` interface
   - records calls: `SetARPStaticCalls []ARPCall`, `DisableARPCalls []string`, `GetARPEntryCalls []string`
   - configurable error injection: `SetError(method string, err error)`
   - `ARPCall` struct: `{IP, MAC, CustomerID string}`

2. [ ] Write provisioner dispatch unit tests using fake
   - file: `internal/app/provisioner_dispatch_test.go` or near `provisionerDispatcher`
   - test: RouterTypeArista routes to Arista provisioner, else MikroTik
   - test: error from provisioner propagates

3. [ ] Write Arista adapter smoke test
   - file: `internal/infrastructure/arista/provisioner_test.go`
   - unit test the command-generation logic using recorded calls

### Phase 2 — Service Suspension → ARP Automation — status: open

1. [ ] Create/extend NATS subscriber: `isp.service.suspended` → `provisioner.DisableARP`
   - check `internal/modules/network/app/subscribers/arp_worker.go`
   - on `ServiceSuspended` event: look up customer's router + IP → call `provisioner.DisableARP`
   - on `ServiceReactivated` event: look up → call `provisioner.SetARPStatic`

2. [ ] Wire provisioner subscriber in `wire_network.go`
   - pass provisioner dispatcher to subscriber
   - register subscriber with NATS on startup (only when network module enabled)

3. [ ] Integration test: service suspend triggers ARP disable
   - use `FakeRouterProvisioner` + testutil NATS
   - setup: customer + router + service → suspend → assert `DisableARPCalls[0].IP == customer.IPAddress`

### Phase 3 — Customer Activation Flow — status: open

1. [ ] Verify/wire `isp.network.arp_requested` → `SetARPStatic`
   - check existing `arp_worker.go` for `NetworkARPRequested` — confirm it actually calls provisioner
   - if missing: add provisioner call

2. [ ] Integration test: ARP enable button → SetARPStatic with correct IP/MAC

3. [ ] `go build ./... && go test ./internal/modules/network/... ./internal/testutil/...`

## Verification

```bash
go test ./internal/testutil/... -v
go test ./internal/infrastructure/arista/... -v
go test ./internal/modules/network/... -v
go build ./...
# Assign service → suspend → verify provisioner called (via fake or log)
# Reactivate → verify SetARPStatic called
```

## Adjustments

## Progress Log
