---
tldr: Control customer internet access by managing MikroTik ARP entries, with NetBox as IPAM source of truth
category: core
---

# Network Provisioning — MikroTik ARP + NetBox

## Intent

VVS manages Layer 2 access control for ISP/hosting customers via MikroTik ARP entries.
When a customer is active: static ARP entry (IP+MAC) → internet works.
When suspended or churned: ARP entry disabled → no L2 connectivity → access cut.
NetBox holds the authoritative IP/MAC assignments; VVS reads from it and applies to routers.

## Behaviour

- Suspending a customer automatically disables their ARP entry on the assigned router
- Reactivating a customer automatically re-enables (static) ARP entry
- Operator can manually trigger enable/disable from the customer detail page
- Customer detail shows current ARP status fetched live from MikroTik
- Each customer is assigned to exactly one router; the system talks only to that router
- NetBox is queried for IP/MAC on each provisioning operation (not cached)

## Design

### Network Module

New bounded context `internal/modules/network/` with the standard hexagonal shape:

```
domain/         ← Router entity, RouterRepository
app/commands/   ← SyncCustomerARP (enable|disable)
app/queries/    ← ListRouters, GetRouter
adapters/
  persistence/  ← GormRouterRepository (stores router connection details)
  http/         ← Router CRUD UI + manual ARP trigger endpoint
migrations/
```

### Router Entity

```go
type Router struct {
    ID       string
    Name     string   // human label, e.g. "Edge-01"
    Host     string   // IP or hostname
    Port     int      // default 8728
    Username string
    Password string   // stored encrypted at rest
}
```

### Customer Extension

Customer domain and DB table gain three fields:
- `RouterID *string` — FK to router; nil = no network provisioning
- `IPAddress string` — e.g. "10.0.1.55"
- `MACAddress string` — e.g. "AA:BB:CC:DD:EE:FF"

These may be auto-populated from NetBox or entered manually.

### Router Provisioner Port

Commands depend on a `RouterProvisioner` port (interface), not a concrete client.
Swap MikroTik → Arista (or any vendor) by changing one line in `app.go`.

```go
// internal/modules/network/domain/provisioner.go
type RouterProvisioner interface {
    SetARPStatic(ctx context.Context, host string, ip, mac, customerID string) error
    DisableARP(ctx context.Context, host string, ip string) error
    GetARPEntry(ctx context.Context, host string, ip string) (*ARPEntry, error)
}
```

Concrete impls:
- `internal/infrastructure/mikrotik/` — current
- `internal/infrastructure/arista/` — future (EOS eAPI or NAPALM)

### MikroTik Infrastructure

`internal/infrastructure/mikrotik/client.go` — implements `RouterProvisioner`
- Uses `github.com/go-routeros/routeros` (RouterOS binary API, port 8728/8729)
- One TCP connection per router, pooled by router ID
- Operations: `SetARPStatic(ip, mac)`, `DisableARP(ip)`, `GetARPEntry(ip)`

RouterOS commands:
```
# make static (enable access)
/ip arp add address={ip} mac-address={mac} interface=bridge comment=vvs-{customerID}

# disable (suspend access)
/ip arp set [find address={ip}] disabled=yes

# enable existing
/ip arp set [find address={ip}] disabled=no static=yes
```

### NetBox Infrastructure

`internal/infrastructure/netbox/client.go`
- HTTP REST client against NetBox API
- Auth: API token (configured in env/config)
- Operations:
  - `GetIPByCustomer(customerCode) (ip, mac, error)` — searches IP addresses by description/tag matching customer code
  - `UpdateARPStatus(ipID, status string) error` — writes custom field `arp_status` back to NetBox IP record

### SyncCustomerARP Command

```go
type SyncCustomerARPCommand struct {
    CustomerID string
    Action     string // "enable" | "disable"
}
```

Flow:
1. Load customer (get RouterID, IPAddress, MACAddress)
2. If IPAddress empty: query NetBox `GetIPByCustomer` → populate
3. Load router connection details
4. Call MikroTik: `SetARPStatic` or `DisableARP`
5. Write ARP status back to NetBox via `UpdateARPStatus`
6. Publish `isp.network.arp_changed` event with customer ID + new status

### Auto-trigger

Network module subscribes to `isp.customer.*` on startup.
On receiving event:
- If `status == "suspended"` or `status == "churned"` → `SyncCustomerARP{Action: "disable"}`
- If `status == "active"` → `SyncCustomerARP{Action: "enable"}`
- Only fires if customer has a `RouterID` set

### UI

Customer detail page:
- ARP status badge (live from MikroTik or last-known)
- "Enable Access" / "Disable Access" button → POST `/api/customers/{id}/arp`

Router management at `/routers`:
- List, add, edit, delete routers
- Test connection button

## Config

```
NETBOX_URL=https://netbox.example.com
NETBOX_TOKEN=abc123
```

Routers stored in DB (Host, Port, Username, Password encrypted).

## Mapping

> [[internal/modules/network/]]
> [[internal/infrastructure/mikrotik/]]
> [[internal/infrastructure/netbox/]]
> [[internal/modules/customer/domain/customer.go]]
> [[eidos/spec - architecture - system design and key decisions.md]]
