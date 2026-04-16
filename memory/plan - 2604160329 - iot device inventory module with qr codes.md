---
tldr: Generic IoT/hardware device inventory — QR code per device, customer assignment, warranty/lifecycle tracking, stock status
status: active
---

# Plan: IoT Device Inventory Module

## Context

Generic hardware device registry: routers, modems, ONTs, sensors, switches — anything with a serial number.
Primary use cases: know what's where, who has it, is it under warranty, scan QR to open in browser.
Fields intentionally minimal; "more fields added later" is the design principle — keep domain extensible.

- Architecture spec: [[spec - architecture - system design and key decisions]]
- Events spec: [[spec - events - event driven module boundaries and nats subject taxonomy]]

---

## Domain Design

**Module:** `internal/modules/device/` (standard hexagonal shape)

**Aggregate: `Device`**
```
ID            string    UUIDv7
Name          string    human label (e.g. "TP-Link EX220 #12")
SerialNumber  string    hardware serial (optional, unique when set)
DeviceType    string    modem | router | ont | switch | sensor | other
Status        string    in_stock | deployed | decommissioned
CustomerID    string    set when deployed, empty otherwise
Location      string    free text: address or warehouse bin
PurchasedAt   *time.Time
WarrantyExpiry *time.Time
Notes         string
CreatedAt     time.Time
UpdatedAt     time.Time
```

**Status machine:**
```
(none) ─register──▶ in_stock
                       │ ▲
                deploy │ │ return
                       ▼ │
                    deployed
                       │
              decommission│
                       ▼
                decommissioned  (terminal)
```

**Commands:** RegisterDevice, DeployDevice(customerID, location), ReturnDevice, DecommissionDevice, UpdateDevice
**Queries:** ListDevices(status?, customerID?, type?, search?, page), GetDevice(id)
**Events:** `isp.device.registered`, `isp.device.deployed`, `isp.device.returned`, `isp.device.decommissioned`

---

## QR Code

- Each device detail page at `/devices/{id}` includes a printable QR code
- QR encodes the full URL: `http(s)://{host}/devices/{id}`
- Generated server-side using `github.com/skip2/go-qrcode`, served as PNG at `GET /devices/{id}/qr.png`
- Template embeds `<img src="/devices/{id}/qr.png">` — works offline (same binary)

---

## Files to Create

```
internal/modules/device/
  domain/
    device.go          — aggregate, status consts, ErrNotFound, ErrInvalidTransition
    device_test.go     — status transition tests
    repository.go      — DeviceRepository interface
  app/
    commands/
      register.go      — RegisterDeviceHandler
      deploy.go        — DeployDeviceHandler
      return.go        — ReturnDeviceHandler
      decommission.go  — DecommissionDeviceHandler
      update.go        — UpdateDeviceHandler
    queries/
      list.go          — ListDevicesHandler + DeviceReadModel
      get.go           — GetDeviceHandler
  adapters/
    persistence/
      gorm_repository.go
      models.go
    http/
      handlers.go      — SSE handlers (list page, detail page)
      templates.templ  — DeviceListPage, DeviceDetailPage (with QR img)
      routes.go
      api.go           — REST JSON API (RegisterAPIRoutes)
  migrations/
    001_create_devices.sql
    embed.go
```

## Files to Modify

```
internal/app/app.go              — wire device module (repo, handlers, routes, RPC)
internal/app/direct.go           — wire device handlers for CLI direct mode
internal/infrastructure/http/router.go  — no change needed (APIRoutes interface already handles it)
internal/infrastructure/nats/rpc/server.go — add device.* subjects
cmd/server/cli_device.go         — NEW: vvs cli device {list,get,register,deploy,return,decommission,update}
```

---

## Phases

### Phase 1 — Domain + persistence + migration - status: open

1. [ ] Write domain aggregate `device.go` + `repository.go` + `device_test.go`
   - status transitions: register → in_stock → deployed ↔ in_stock → decommissioned
   - ErrInvalidTransition for illegal moves
   - ErrNotFound for missing devices

2. [ ] Write migration `001_create_devices.sql`
   ```sql
   CREATE TABLE devices (
     id             TEXT PRIMARY KEY,
     name           TEXT NOT NULL,
     serial_number  TEXT,
     device_type    TEXT NOT NULL DEFAULT 'other',
     status         TEXT NOT NULL DEFAULT 'in_stock'
                    CHECK(status IN ('in_stock','deployed','decommissioned')),
     customer_id    TEXT,
     location       TEXT,
     purchased_at   DATETIME,
     warranty_expiry DATETIME,
     notes          TEXT,
     created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
     updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
   );
   CREATE INDEX idx_devices_status      ON devices(status);
   CREATE INDEX idx_devices_customer    ON devices(customer_id);
   CREATE UNIQUE INDEX idx_devices_serial ON devices(serial_number)
     WHERE serial_number IS NOT NULL AND serial_number != '';
   ```

3. [ ] Write GORM persistence (`models.go` + `gorm_repository.go`)

### Phase 2 — Command + query handlers - status: open

4. [ ] Write command handlers (register, deploy, return, decommission, update) + tests

5. [ ] Write query handlers (`ListDevices`, `GetDevice`) + `DeviceReadModel`

### Phase 3 — HTTP UI - status: open

6. [ ] Write SSE handlers (`handlers.go`) + routes
   - `GET /devices` — list with status filter chips
   - `GET /devices/{id}` — detail page
   - `GET /devices/{id}/qr.png` — QR PNG (go-qrcode, encodes `{baseURL}/devices/{id}`)
   - `POST /api/devices` etc. (via api.go)

7. [ ] Write templ templates
   - `DeviceListPage` — table with status badge, type, customer, warranty expiry
   - `DeviceDetailPage` — all fields + QR code image + action buttons (deploy/return/decommission)

8. [ ] Add `github.com/skip2/go-qrcode` dependency

### Phase 4 — Wire + RPC + CLI - status: open

9. [ ] Wire into `app.go` and `direct.go`
   - run device migration
   - wire repo + commands + queries
   - register HTTP routes
   - add device subjects to natsrpc.Server

10. [ ] Add device subjects to `internal/infrastructure/nats/rpc/server.go`
    ```
    isp.rpc.device.list
    isp.rpc.device.get
    isp.rpc.device.register
    isp.rpc.device.deploy
    isp.rpc.device.return
    isp.rpc.device.decommission
    isp.rpc.device.update
    ```

11. [ ] Create `cmd/server/cli_device.go`
    - `vvs cli device list [--status] [--customer] [--type]`
    - `vvs cli device get <id>`
    - `vvs cli device register --name --type --serial`
    - `vvs cli device deploy <id> --customer <customerID> --location`
    - `vvs cli device return <id>`
    - `vvs cli device decommission <id>`
    - `vvs cli device update <id> [--name] [--notes] ...`

---

## Verification

1. `go test ./internal/modules/device/... -v -race` — all transitions tested
2. `templ generate && go build ./...` — clean build
3. Browser: `/devices` loads empty list
4. Register a device via `vvs cli device register --name "Test ONT" --type ont`
5. Detail page shows QR code; scan with phone → opens `/devices/{id}`
6. Deploy to customer; list page shows customer name column
7. Return device → status back to in_stock
8. `vvs cli device list --status deployed` → shows only deployed devices

---

## Progress Log

| Timestamp | Entry |
|-----------|-------|
| 2604160329 | Plan created |
