---
tldr: FakeRouterProvisioner mock unblocks tests; wire service suspension → ARP disable automation
status: completed
---

# Plan: Network Provisioning Mock + Automation

## Context

Consilium 2026-04-19 priority #4. Largest operational gap — new subscriber activations and service suspensions require manual router access today. QA blocked: no mock `RouterProvisioner` for tests.

- Existing domain: `networkdomain.RouterProvisioner` interface (SetARPStatic, DisableARP, GetARPEntry)
- Existing: `provisionerDispatcher` in `internal/app/app.go` dispatches MikroTik vs Arista
- Service suspension already fires `isp.service.suspended` NATS event — domain side ready, provisioner never called
- Consilium backlog: [[project_consilium_backlog_priority]] — #4

## Phases

### Phase 1 — FakeRouterProvisioner Mock — status: completed

1. [x] Create `FakeRouterProvisioner` in testutil
   - => `internal/testutil/fake_provisioner.go`
   - => records `SetARPStaticCalls []ARPCall`, `DisableARPCalls []DisableARPCall`, `GetARPEntryCalls []string`
   - => `SetError(method, err)` for error injection; `Reset()` to clear

2. [x] Subscriber integration tests using fake
   - => `internal/modules/network/app/subscribers/arp_worker_test.go`
   - => 3 tests: service.suspended → DisableARP, service.reactivated → SetARPStatic, service.cancelled → DisableARP
   - => uses testutil.NewTestNATS for embedded NATS; 50ms sleep for subscription setup

### Phase 2 — Service Suspension → ARP Automation — status: completed (pre-existing)

- ARPWorker.handleServiceEvent already handles service lifecycle events → SyncCustomerARPCommand
- SyncCustomerARPHandler calls provisioner.DisableARP / SetARPStatic
- Wire in wire_network.go already done (ARPWorker registered)

### Phase 3 — Customer Activation Flow — status: completed (pre-existing)

- ARPWorker.handleARPRequested handles manual ARP enable/disable from UI
- ARPWorker.handleCustomerEvent handles customer status changes

## Verification

```bash
go test ./internal/testutil/... ./internal/modules/network/... -v
go build ./...
```

## Adjustments

2026-04-19: Phase 2 + 3 were already implemented in a prior session.
Main work was Phase 1 (FakeRouterProvisioner + subscriber tests).

## Progress Log

2026-04-19: All phases complete. commit f9cf15b.
