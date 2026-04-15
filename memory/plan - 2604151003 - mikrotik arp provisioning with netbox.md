---
tldr: MikroTik ARP enable/disable per customer, triggered by status change and manual UI, NetBox as IPAM source of truth
status: active
---

# MikroTik ARP Provisioning with NetBox

## Context

Spec: [[spec - network - mikrotik arp provisioning with netbox]]
Arch: [[spec - architecture - system design and key decisions]]

ISP/hosting: customers need L2 access cut when suspended. MikroTik manages ARP entries. NetBox holds IP/MAC assignments. Multiple routers — each customer assigned to one.

## Phases

### Phase 1 — Research infrastructure libraries — status: open

1. [ ] `/eidos:research` MikroTik RouterOS Go API — `github.com/go-routeros/routeros` capabilities, connection lifecycle, ARP commands
2. [ ] `/eidos:research` NetBox REST API — IP address search by tag/description, custom field update endpoints, auth

### Phase 2 — Customer domain extension — status: open

3. [ ] Add `RouterID *string`, `IPAddress string`, `MACAddress string` to `domain.Customer`
   - migration: `ALTER TABLE customers ADD COLUMN router_id TEXT, ADD COLUMN ip_address TEXT, ADD COLUMN mac_address TEXT`

4. [ ] Update `UpdateCustomerCommand` to accept the new fields
   - update `customer.Update(...)` method signature

### Phase 3 — MikroTik infrastructure adapter — status: open

5. [ ] Define `RouterProvisioner` interface in `internal/modules/network/domain/provisioner.go`
   - `SetARPStatic`, `DisableARP`, `GetARPEntry` — vendor-agnostic port
   - MikroTik impl wired in `app.go`; future: swap for Arista by changing one line

6. [ ] Add `github.com/go-routeros/routeros` to go.mod
7. [ ] Implement `internal/infrastructure/mikrotik/client.go` — implements `RouterProvisioner`
   - `NewClient(host, port, user, pass) (*Client, error)` — dial and authenticate
   - `SetARPStatic(ctx, ip, mac, customerID string) error`
   - `DisableARP(ctx, ip string) error`
   - `GetARPEntry(ctx, ip string) (*ARPEntry, error)`
   - write unit tests with mock RouterOS responses

### Phase 4 — NetBox infrastructure adapter — status: open

7. [ ] Read `NETBOX_URL` and `NETBOX_TOKEN` from config
8. [ ] Implement `internal/infrastructure/netbox/client.go`
   - `GetIPByCustomerCode(ctx, code string) (ip, mac string, id int, error)`
   - `UpdateARPStatus(ctx, ipID int, status string) error`
   - write tests with `httptest` mock server

### Phase 5 — Network module: Router CRUD — status: open

9. [ ] Domain: `Router` struct + `RouterRepository` interface in `internal/modules/network/domain/`
10. [ ] GORM persistence + migration (table: `routers`)
11. [ ] Commands: `CreateRouter`, `UpdateRouter`, `DeleteRouter`
12. [ ] Query: `ListRouters`, `GetRouter`
13. [ ] HTTP handlers + templ templates: `/routers` list, `/routers/new`, `/routers/{id}/edit`
    - include "Test connection" button → GET `/api/routers/{id}/test`

### Phase 6 — SyncCustomerARP command — status: open

14. [ ] `internal/modules/network/app/commands/sync_customer_arp.go`
    - loads customer, router, queries NetBox if IP empty, calls MikroTik, writes back to NetBox
    - publishes `isp.network.arp_changed` event

15. [ ] Auto-trigger subscriber: on `isp.customer.*` event, if customer has RouterID, dispatch SyncCustomerARP
    - wired in `app.go` as a background goroutine subscriber

16. [ ] Wire in `app.go`: create MikroTik client pool, NetBox client, network module handlers

### Phase 7 — Customer UI: ARP status + manual trigger — status: open

17. [ ] Customer detail page: ARP status badge (active/disabled/unknown)
18. [ ] "Enable Access" / "Disable Access" POST button → `/api/customers/{id}/arp`
19. [ ] Customer form: add Router (dropdown), IP address, MAC address fields

## Verification

- Create customer, assign router + IP/MAC → status active → ARP static on MikroTik
- Suspend customer → ARP disabled automatically via NATS event
- Reactivate → ARP re-enabled
- Manual button on detail page triggers same flow
- NetBox IP record updated with ARP status after each operation
- Test connection button on router edit page confirms RouterOS auth works

## Progress Log

- 2604151003 — Plan created; spec [[spec - network - mikrotik arp provisioning with netbox]] written
