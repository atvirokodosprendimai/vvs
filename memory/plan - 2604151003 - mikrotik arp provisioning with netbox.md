---
tldr: MikroTik ARP enable/disable per customer, triggered by status change and manual UI, NetBox as IPAM source of truth
status: completed
---

# MikroTik ARP Provisioning with NetBox

## Context

Spec: [[spec - network - mikrotik arp provisioning with netbox]]
Arch: [[spec - architecture - system design and key decisions]]

ISP/hosting: customers need L2 access cut when suspended. MikroTik manages ARP entries. NetBox holds IP/MAC assignments. Multiple routers — each customer assigned to one.

## Phases

### Phase 1 — Research infrastructure libraries — status: completed

1. [x] `/eidos:research` MikroTik RouterOS Go API — `github.com/go-routeros/routeros` capabilities, connection lifecycle, ARP commands
   - => go-routeros: `routeros.Dial(addr, user, pass)`, Run/RunArgs, Reply.Re[]*proto.Sentence
2. [x] `/eidos:research` NetBox REST API — IP address search by tag/description, custom field update endpoints, auth
   - => MAC lives on assigned_object (interface), not the IP record; PATCH custom_fields.arp_status

### Phase 2 — Customer domain extension — status: completed

3. [x] Add `RouterID *string`, `IPAddress string`, `MACAddress string` to `domain.Customer`
   - => migration 002_add_network_fields.sql created
4. [x] Update `UpdateCustomerCommand` to accept the new fields
   - => SetNetworkInfo called after Update in handler

### Phase 3 — MikroTik infrastructure adapter — status: completed

5. [x] Define `RouterProvisioner` interface in `internal/modules/network/domain/provisioner.go`
   - => ARPEntry + RouterConn value types (vendor-agnostic)
6. [x] Add `github.com/go-routeros/routeros` to go.mod
7. [x] Implement `internal/infrastructure/mikrotik/client.go` — implements `RouterProvisioner`
   - => internal routerosConn interface ([]map[string]string rows) for clean testing
   - => connection pool keyed by RouterID; evict on error
   - => 9 unit tests passing

### Phase 4 — NetBox infrastructure adapter — status: completed

8. [x] Read `NETBOX_URL` and `NETBOX_TOKEN` from config (CLI flags + env vars)
9. [x] Implement `internal/infrastructure/netbox/client.go`
   - => GetIPByCustomerCode follows assigned_object ref to get MAC from interface
   - => UpdateARPStatus PATCHes custom_fields.arp_status
   - => 6 tests with httptest mock server

### Phase 5 — Network module: Router CRUD — status: completed

10. [x] Domain: `Router` struct + `RouterRepository` interface
11. [x] GORM persistence + migration 001_create_routers.sql + goose_network table
12. [x] Commands: `CreateRouter`, `UpdateRouter`, `DeleteRouter`
13. [x] Query: `ListRouters`, `GetRouter`
14. [x] HTTP handlers + templ templates: `/routers` list, form, detail
    - => "Routers" nav link added to sidebar
    - => live list via NATS isp.network.router.*

### Phase 6 — SyncCustomerARP command — status: completed

15. [x] `internal/modules/network/app/commands/sync_customer_arp.go`
    - => loads customer → resolve IP via NetBox if empty → call MikroTik → write arp_status back
    - => publishes isp.network.arp_changed event
16. [x] Auto-trigger goroutine in app.go subscribes to isp.customer.*
17. [x] Wire: MikroTik client pool, NetBox client (optional), network handlers in app.go

### Phase 7 — Customer UI: ARP status + manual trigger — status: completed

18. [x] Customer detail: ARP section with Enable Access / Disable Access buttons → POST /api/customers/{id}/arp
19. [x] Customer form: Router dropdown + IP/MAC fields (shown when routers configured)

## Verification

- Create customer, assign router + IP/MAC → status active → ARP static on MikroTik
- Suspend customer → ARP disabled automatically via NATS event
- Reactivate → ARP re-enabled
- Manual button on detail page triggers same flow
- NetBox IP record updated with ARP status after each operation
- Test connection button on router edit page confirms RouterOS auth works

## Progress Log

- 2604151003 — Plan created; spec [[spec - network - mikrotik arp provisioning with netbox]] written
- 2604151003 — Phase 1 research completed via background agents
- 2604151003 — Phase 2: customer domain extension + migration
- 2604151003 — Phase 3: RouterProvisioner interface + MikroTik client (9 tests)
- 2604151003 — Phase 4: NetBox client with httptest tests (6 tests)
- 2604151003 — Phase 5: full network module Router CRUD + sidebar nav
- 2604151003 — Phase 6: SyncCustomerARP + auto-trigger subscriber + app.go wiring
- 2604151003 — Phase 7: customer form network fields + detail ARP buttons; plan completed
