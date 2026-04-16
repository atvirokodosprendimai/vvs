---
tldr: Generic IoT/hardware device inventory — QR code per device, customer assignment, warranty/lifecycle tracking, stock status
status: completed
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

### Phase 1 — Domain + persistence + migration - status: completed

1. [x] Write domain aggregate `device.go` + `repository.go` + `device_test.go`
   - => 9 tests, all pass — status machine fully covered
2. [x] Write migration `001_create_devices.sql`
   - => unique serial index null-safe: `WHERE serial_number IS NOT NULL AND serial_number != ''`
3. [x] Write GORM persistence (`models.go` + `gorm_repository.go`)

### Phase 2 — Command + query handlers - status: completed

4. [x] Write command handlers (register, deploy, return, decommission, update)
   - => no unit tests beyond domain (command handlers are thin wrappers; tested via CLI smoke)
5. [x] Write query handlers (`ListDevices`, `GetDevice`) + `DeviceReadModel`
   - => ListDevices: status/customerID/type/search filters + pagination

### Phase 3 — HTTP UI - status: completed

6. [x] Write SSE handlers (`handlers.go`) + routes
   - => GET /devices, GET /devices/{id}, GET /devices/{id}/qr.png
   - => SSE list auto-refreshes on isp.device.* events
7. [x] Write templ templates
   - => DeviceListPage: filter tabs (All/In Stock/Deployed/Decommissioned), register modal
   - => DeviceDetailPage: detail card + QR code panel + deploy/return/decommission buttons
8. [x] Add `github.com/skip2/go-qrcode` dependency
   - => server-side PNG, 256px, cached 24h, encodes full URL

### Phase 4 — Wire + RPC + CLI - status: completed

9. [x] Wire into `app.go` and `direct.go`
   - => device module enabled via `cfg.IsEnabled("device")`
10. [x] Add device subjects to `internal/infrastructure/nats/rpc/server.go`
    - => 7 subjects: isp.rpc.device.{list,get,register,deploy,return,decommission,update}
11. [x] Create `cmd/server/cli_device.go`
    - => smoke tested: `vvs cli device register --name "Test ONT" --type ont --serial SN-TEST-001` ✓

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
| 2604160350 | All 4 phases complete — domain tests pass, `go build ./...` clean, CLI smoke tested |
