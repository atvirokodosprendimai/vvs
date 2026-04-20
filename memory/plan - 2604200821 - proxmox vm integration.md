---
tldr: Proxmox VE integration — manage VMs (create/suspend/restart/delete) from admin UI; VMs linked to customers; node connection management
status: active
---

# Plan: Proxmox VM Integration

## Context

- Architecture: [[spec - architecture - system design and key decisions]]
- No existing Proxmox spec — recommend `/eidos:spec proxmox` before implementation starts
- Closest reference module: `internal/modules/network/` (provisioner port pattern, encrypted credentials, node registry)
- Config pattern: `Config.IsEnabled("proxmox")`, new `ProxmoxEncKey` field (like `RouterEncKey`)
- NATS subjects: extend `internal/shared/events/subjects.go` (no bare strings)
- Nav placement: new **Compute** collapsible group in sidebar (between Network and System)
- CRM tab: add **VMs** as 9th tab to customer detail panel (`CRMTabBar`)

### Design decisions

| Decision | Choice | Reason |
|---|---|---|
| VM creation approach | Clone from template | Practical for ISP — templates pre-configured, fast provisioning |
| Proxmox auth | API token (`PVEAPIToken=USER@REALM!TOKENID=SECRET`) | No password storage, scoped permissions, Proxmox best practice |
| Token storage | AES-256-GCM encrypted at rest | Same as `RouterEncKey` / `EmailEncKey` pattern |
| Async operations | Optimistic status + background goroutine poll | Commands return instantly; goroutine polls UPID, publishes NATS event on completion |
| VMID allocation | `GET /cluster/nextid` from Proxmox | Avoids manual conflict management across cluster nodes |
| TLS | Configurable `InsecureTLS` per node | Self-signed certs common in Proxmox homelab / SMB deployments |
| Delete safety | Stop VM if running, then delete with `purge=1` | Prevents orphaned disks; operator must confirm in UI |

### Proxmox VE API reference

- Base URL: `https://{host}:{port}/api2/json` (default port 8006)
- Auth header: `Authorization: PVEAPIToken=USER@REALM!TOKENID=SECRET`
- Key endpoints used:
  - `GET  /cluster/nextid` — get next available VMID
  - `GET  /nodes/{node}/qemu` — list VMs on node
  - `POST /nodes/{node}/qemu/{vmid}/clone` — clone from template (returns UPID)
  - `GET  /nodes/{node}/qemu/{vmid}/status/current` — get VM status + config
  - `POST /nodes/{node}/qemu/{vmid}/status/start` — start stopped VM (returns UPID)
  - `POST /nodes/{node}/qemu/{vmid}/status/suspend` — suspend/pause VM (returns UPID)
  - `POST /nodes/{node}/qemu/{vmid}/status/reboot` — restart VM (returns UPID)
  - `POST /nodes/{node}/qemu/{vmid}/status/stop` — hard stop (returns UPID)
  - `DELETE /nodes/{node}/qemu/{vmid}` — delete (must be stopped; add `?purge=1&destroy-unreferenced-disks=1`)
  - `GET /nodes/{node}/tasks/{upid}/status` — poll task status (fields: `status`, `exitstatus`)
  - `GET /nodes` — list all cluster nodes (for node-picker in UI)

---

## Phases

### Phase 1 — Spec (recommended prerequisite) — status: open

1. [ ] `/eidos:spec proxmox VE VM management integration`
   - Describe VM lifecycle, node management, customer association, async task model
   - Link from this plan once created
   - Skip this phase only if speed is prioritised over documentation

---

### Phase 2 — Domain layer — status: completed

**Goal:** Pure Go domain types with zero framework dependencies. All business rules here.

1. [x] Create `internal/modules/proxmox/domain/node.go`
   - `ProxmoxNode` struct: `ID, Name, NodeName, Host string; Port int; User, TokenID, TokenSecret, Notes string; InsecureTLS bool; CreatedAt, UpdatedAt time.Time`
   - `NodeConn` struct (passed to provisioner): `NodeName, Host, User, TokenID, TokenSecret string; Port int; InsecureTLS bool`
   - `NewProxmoxNode(name, nodeName, host string, port int, user, tokenID, tokenSecret, notes string, insecureTLS bool) (*ProxmoxNode, error)` — validates required fields
   - `(n *ProxmoxNode) Update(...)` — same params, validates, sets UpdatedAt
   - `(n *ProxmoxNode) ToConn() NodeConn` — project into connection value
   - Sentinel errors: `ErrNodeNameRequired`, `ErrHostRequired`, `ErrUserRequired`, `ErrTokenIDRequired`, `ErrNodeNotFound`
   - Default port: 8006 when `port <= 0`

2. [x] Create `internal/modules/proxmox/domain/vm.go`
   - `VMStatus` type alias `string` with constants: `VMStatusRunning`, `VMStatusStopped`, `VMStatusPaused`, `VMStatusCreating`, `VMStatusDeleting`, `VMStatusUnknown`
   - `VirtualMachine` struct: `ID, VMID int; NodeID, CustomerID, Name, Notes, IPAddress string; Status VMStatus; Cores, MemoryMB, DiskGB int; CreatedAt, UpdatedAt time.Time`
     - Note: `ID` is internal UUID string; `VMID` is Proxmox integer ID (100–999999)
     - `CustomerID` is optional (empty string = unassigned)
   - Constructor `NewVirtualMachine(vmid int, nodeID, customerID, name string, cores, memoryMB, diskGB int, notes string) (*VirtualMachine, error)`
   - Methods: `Suspend()`, `Resume()`, `Restart()`, `MarkDeleting()`, `MarkDeleted()` — update `Status` and `UpdatedAt`
   - `AssignCustomer(customerID string)` / `UnassignCustomer()`
   - Sentinel errors: `ErrVMIDRequired`, `ErrVMNameRequired`, `ErrCoresPositive`, `ErrMemoryPositive`, `ErrVMNotFound`, `ErrVMNotRunning` (for suspend/restart guard), `ErrVMNotStopped` (for delete guard)

3. [x] Create `internal/modules/proxmox/domain/provisioner.go`
   - `VMSpec` struct: `TemplateVMID int; NewVMID int; Name, Storage string; Cores, MemoryMB, DiskGB int; FullClone bool`
     - `NewVMID == 0` means auto-allocate via `NextVMID`
   - `VMInfo` struct: `VMID int; Name string; Status VMStatus; Cores, MemoryMB int`
   - `VMProvisioner` interface:
     ```go
     NextVMID(ctx, conn NodeConn) (int, error)
     CreateVM(ctx, conn NodeConn, spec VMSpec) (upid string, err error)
     SuspendVM(ctx, conn NodeConn, vmid int) (upid string, err error)
     StartVM(ctx, conn NodeConn, vmid int) (upid string, err error)
     RestartVM(ctx, conn NodeConn, vmid int) (upid string, err error)
     StopVM(ctx, conn NodeConn, vmid int) (upid string, err error)
     DeleteVM(ctx, conn NodeConn, vmid int) (upid string, err error)
     GetVMInfo(ctx, conn NodeConn, vmid int) (*VMInfo, error)
     WaitForTask(ctx context.Context, conn NodeConn, upid string) error
     ```
   - `WaitForTask` polls every 2s; returns `ErrTaskFailed` on non-OK exitstatus; honours context deadline

4. [x] Create `internal/modules/proxmox/domain/repository.go`
   - => also added `FindByNodeID` to VMRepository (needed by delete_node guard)
   - `NodeRepository` interface: `Save(ctx, *ProxmoxNode) error; FindByID(ctx, id) (*ProxmoxNode, error); FindAll(ctx) ([]*ProxmoxNode, error); Delete(ctx, id) error`
   - `VMRepository` interface: `Save(ctx, *VirtualMachine) error; FindByID(ctx, id) (*VirtualMachine, error); FindByCustomerID(ctx, customerID) ([]*VirtualMachine, error); FindAll(ctx) ([]*VirtualMachine, error); Delete(ctx, id) error; UpdateStatus(ctx, id string, status VMStatus) error`

---

### Phase 3 — Proxmox REST API adapter — status: open

**Goal:** Stateless HTTP client that satisfies `VMProvisioner`. Lives in `internal/infrastructure/proxmox/`.

1. [ ] Create `internal/infrastructure/proxmox/client.go`
   - `Client` struct: `httpClient *http.Client` (configured with optional `InsecureSkipVerify`)
   - `NewClient(insecureTLS bool) *Client`
   - Private helper `(c *Client) do(ctx, method, url, body, tokenAuth string) (*http.Response, error)`
   - Private helper `(c *Client) buildURL(conn NodeConn, path string) string` → `https://{host}:{port}/api2/json{path}`
   - Private helper `authHeader(conn NodeConn) string` → `"PVEAPIToken=" + conn.User + "!" + conn.TokenID + "=" + conn.TokenSecret`
   - `(c *Client) NextVMID(ctx, conn) (int, error)` — GET `/cluster/nextid`, parse `data` field
   - All methods handle `data` wrapper in Proxmox JSON responses: `{"data": ...}`

2. [ ] Create `internal/infrastructure/proxmox/vm_ops.go`
   - `(c *Client) CreateVM(ctx, conn, spec VMSpec) (upid string, error)` — POST `/nodes/{node}/qemu/{templateVMID}/clone` with body: `newid, name, full, storage`; auto-allocate VMID if `spec.NewVMID == 0`
   - `(c *Client) SuspendVM(ctx, conn, vmid int) (upid string, error)` — POST `.../status/suspend`
   - `(c *Client) StartVM(ctx, conn, vmid int) (upid string, error)` — POST `.../status/start`
   - `(c *Client) RestartVM(ctx, conn, vmid int) (upid string, error)` — POST `.../status/reboot`
   - `(c *Client) StopVM(ctx, conn, vmid int) (upid string, error)` — POST `.../status/stop`
   - `(c *Client) DeleteVM(ctx, conn, vmid int) (upid string, error)` — DELETE `.../qemu/{vmid}?purge=1&destroy-unreferenced-disks=1`
   - `(c *Client) GetVMInfo(ctx, conn, vmid int) (*domain.VMInfo, error)` — GET `.../status/current`; map `status` string to `VMStatus` constants

3. [ ] Create `internal/infrastructure/proxmox/task.go`
   - `(c *Client) WaitForTask(ctx, conn, upid string) error`
   - URL-encodes UPID (contains `:`); polls `GET /nodes/{node}/tasks/{upid}/status` every 2s
   - Task done when `status == "stopped"`; success when `exitstatus == "OK"`
   - Returns `ErrTaskFailed{ExitStatus: exitstatus}` on failure
   - Returns `ctx.Err()` on context cancellation/timeout

4. [ ] Create `internal/infrastructure/proxmox/client_test.go`
   - Unit tests with `httptest.NewServer` — mock Proxmox responses
   - Test: `NextVMID` returns int from `{"data": 101}`
   - Test: `WaitForTask` polls until `status == "stopped"`, returns nil on `exitstatus == "OK"`
   - Test: `WaitForTask` returns `ErrTaskFailed` on non-OK exitstatus
   - Test: `WaitForTask` returns context error on cancelled context

---

### Phase 4 — Persistence layer — status: open

**Goal:** GORM models, Goose migrations, repository implementations.

1. [ ] Create `internal/modules/proxmox/migrations/001_create_proxmox_nodes.sql`
   ```sql
   -- +goose Up
   CREATE TABLE proxmox_nodes (
       id           TEXT PRIMARY KEY,
       name         TEXT NOT NULL,
       node_name    TEXT NOT NULL,
       host         TEXT NOT NULL,
       port         INTEGER NOT NULL DEFAULT 8006,
       "user"       TEXT NOT NULL,
       token_id     TEXT NOT NULL,
       token_secret TEXT NOT NULL,
       insecure_tls INTEGER NOT NULL DEFAULT 0,
       notes        TEXT NOT NULL DEFAULT '',
       created_at   DATETIME NOT NULL,
       updated_at   DATETIME NOT NULL
   );
   -- +goose Down
   DROP TABLE proxmox_nodes;
   ```

2. [ ] Create `internal/modules/proxmox/migrations/002_create_proxmox_vms.sql`
   ```sql
   -- +goose Up
   CREATE TABLE proxmox_vms (
       id          TEXT PRIMARY KEY,
       vmid        INTEGER NOT NULL,
       node_id     TEXT NOT NULL REFERENCES proxmox_nodes(id),
       customer_id TEXT,
       name        TEXT NOT NULL,
       status      TEXT NOT NULL DEFAULT 'unknown',
       cores       INTEGER NOT NULL DEFAULT 1,
       memory_mb   INTEGER NOT NULL DEFAULT 1024,
       disk_gb     INTEGER NOT NULL DEFAULT 10,
       ip_address  TEXT NOT NULL DEFAULT '',
       notes       TEXT NOT NULL DEFAULT '',
       created_at  DATETIME NOT NULL,
       updated_at  DATETIME NOT NULL
   );
   CREATE UNIQUE INDEX idx_proxmox_vms_vmid_node ON proxmox_vms(vmid, node_id);
   CREATE INDEX idx_proxmox_vms_customer ON proxmox_vms(customer_id);
   -- +goose Down
   DROP INDEX IF EXISTS idx_proxmox_vms_customer;
   DROP INDEX IF EXISTS idx_proxmox_vms_vmid_node;
   DROP TABLE proxmox_vms;
   ```

3. [ ] Create `internal/modules/proxmox/migrations/embed.go`
   - `//go:embed *.sql` → `var Migrations embed.FS`
   - Goose version table: `goose_proxmox`

4. [ ] Create `internal/modules/proxmox/adapters/persistence/models.go`
   - `GormProxmoxNode` — maps to `proxmox_nodes`, `TableName() string` returns `"proxmox_nodes"`
   - `GormProxmoxVM` — maps to `proxmox_vms`, `TableName() string` returns `"proxmox_vms"`
   - `toNodeDomain(*GormProxmoxNode) *domain.ProxmoxNode` / `fromNodeDomain` conversion helpers
   - `toVMDomain(*GormProxmoxVM) *domain.VirtualMachine` / `fromVMDomain` conversion helpers

5. [ ] Create `internal/modules/proxmox/adapters/persistence/gorm_node_repository.go`
   - `GormNodeRepository` struct: `db *gormsqlite.DB; encKey []byte`
   - Implements `domain.NodeRepository`
   - `Save` — encrypt `TokenSecret` before write using same AES-256-GCM helper as router persistence
   - `FindByID` / `FindAll` — decrypt `TokenSecret` after read
   - `Delete` — hard delete (cascade doesn't apply since VMs FK must be cleaned first — document this constraint)

6. [ ] Create `internal/modules/proxmox/adapters/persistence/gorm_vm_repository.go`
   - `GormVMRepository` struct: `db *gormsqlite.DB`
   - Implements `domain.VMRepository`
   - `UpdateStatus` — targeted UPDATE, not full model write (avoids overwriting other fields)
   - `FindByCustomerID` — used by CRM tab query

7. [ ] Create `internal/modules/proxmox/adapters/persistence/node_repository_test.go`
   - Integration test — open in-memory SQLite, run migrations, test save/find/delete roundtrip
   - Verify token secret is encrypted at rest (raw DB value != plaintext)
   - Verify decryption on read restores original secret

---

### Phase 5 — NATS subjects + Config — status: open

**Goal:** Register all Proxmox NATS subjects centrally. Add config fields.

1. [ ] Add Proxmox subjects to `internal/shared/events/subjects.go`
   ```go
   // ── Proxmox ──────────────────────────────────────────────────────────
   var ProxmoxVMAll       Subject = "isp.proxmox.vm.*"
   var ProxmoxVMCreated   Subject = "isp.proxmox.vm.created"
   var ProxmoxVMSuspended Subject = "isp.proxmox.vm.suspended"
   var ProxmoxVMResumed   Subject = "isp.proxmox.vm.resumed"
   var ProxmoxVMRestarted Subject = "isp.proxmox.vm.restarted"
   var ProxmoxVMDeleted   Subject = "isp.proxmox.vm.deleted"
   var ProxmoxVMStatusChanged Subject = "isp.proxmox.vm.status_changed"
   var ProxmoxNodeAll     Subject = "isp.proxmox.node.*"
   var ProxmoxNodeCreated Subject = "isp.proxmox.node.created"
   var ProxmoxNodeUpdated Subject = "isp.proxmox.node.updated"
   var ProxmoxNodeDeleted Subject = "isp.proxmox.node.deleted"
   ```

2. [ ] Add `ProxmoxEncKey string` to `Config` in `internal/app/config.go`
   - Comment: `// VVS_PROXMOX_ENC_KEY — 32-byte hex key for encrypting Proxmox node token secrets`
   - Also add to `cmd/server/main.go` env var parsing (same pattern as `RouterEncKey`)

---

### Phase 6 — Node management commands + HTTP handlers — status: open

**Goal:** CRUD for Proxmox nodes. Nodes are connection profiles; list + detail + create + edit + delete.

1. [ ] Create `internal/modules/proxmox/app/commands/create_node.go`
   - `CreateNodeCommand{Name, NodeName, Host string; Port int; User, TokenID, TokenSecret, Notes string; InsecureTLS bool}`
   - `CreateNodeHandler.Handle` → `domain.NewProxmoxNode(...)` → `nodeRepo.Save` → publish `ProxmoxNodeCreated`
   - Event data: marshal node read model (ID + Name + Host), NOT the secret

2. [ ] Create `internal/modules/proxmox/app/commands/update_node.go`
   - `UpdateNodeCommand{ID, Name, NodeName, Host string; Port int; User, TokenID, TokenSecret, Notes string; InsecureTLS bool}`
   - `UpdateNodeHandler.Handle` → `nodeRepo.FindByID` → `node.Update(...)` → `nodeRepo.Save` → publish `ProxmoxNodeUpdated`
   - Partial update: if `TokenSecret == ""` preserve existing secret (same pattern as router password)

3. [ ] Create `internal/modules/proxmox/app/commands/delete_node.go`
   - `DeleteNodeCommand{ID string}`
   - Guard: check no VMs exist for this node (`vmRepo.FindAll` + filter) — return `ErrNodeHasVMs` if any
   - `nodeRepo.Delete` → publish `ProxmoxNodeDeleted`

4. [ ] Create `internal/modules/proxmox/app/queries/` — node queries
   - `list_nodes.go` — `ListNodesHandler` reads from `db.R`, returns `[]NodeReadModel{ID, Name, NodeName, Host, Port, User, InsecureTLS, Notes, CreatedAt}`
     - Never includes `TokenSecret` in read models
   - `get_node.go` — `GetNodeHandler` returns single `NodeReadModel` by ID

5. [ ] Create `internal/modules/proxmox/app/queries/` — VM queries
   - `list_vms.go` — `ListVMsHandler` reads all VMs with optional node JOIN for NodeName; returns `[]VMReadModel`
   - `list_vms_for_customer.go` — `ListVMsForCustomerHandler` — filters by `customer_id`; returns `[]VMReadModel`
   - `get_vm.go` — `GetVMHandler` — single VM by ID; joins node for display
   - `VMReadModel{ID, VMID int, NodeID, NodeName, CustomerID, Name string; Status VMStatus; Cores, MemoryMB, DiskGB int; IPAddress, Notes string; CreatedAt, UpdatedAt time.Time}`

6. [ ] Create `internal/modules/proxmox/adapters/http/handlers.go` — Node CRUD handlers
   - `Handlers` struct: node+VM commands, queries, subscriber, publisher
   - `RegisterRoutes(r chi.Router)`:
     - `GET  /proxmox/nodes` — list page
     - `GET  /proxmox/nodes/new` — create form page
     - `GET  /proxmox/nodes/{id}` — detail page
     - `GET  /proxmox/nodes/{id}/edit` — edit form page
     - `GET  /api/proxmox/nodes` — list SSE
     - `POST /api/proxmox/nodes` — create SSE
     - `PUT  /api/proxmox/nodes/{id}` — update SSE
     - `DELETE /api/proxmox/nodes/{id}` — delete SSE

7. [ ] Create `internal/modules/proxmox/adapters/http/node_templates.templ`
   - `ProxmoxNodesPage` — nodes list with table: Name, Node, Host, VMs count, actions
   - `NodeForm` — shared create/edit form component: name, nodeName, host, port, user, tokenID, tokenSecret (password input), insecureTLS checkbox, notes
   - `NodeCard` — single row fragment for SSE patching
   - `NodeFormPage(node *NodeReadModel)` — wraps NodeForm for edit (nil = create)
   - Confirm delete: inline `data-on:click` with `confirm()` before DELETE request

8. [ ] `go build ./internal/modules/proxmox/...` — verify node CRUD compiles clean

---

### Phase 7 — VM Create command (async with task polling) — status: open

**Goal:** Operator clones a Proxmox template into a new VM, tracks async task, auto-updates status.

1. [ ] Create `internal/modules/proxmox/app/commands/create_vm.go`
   - `CreateVMCommand{NodeID, CustomerID, Name, Storage, Notes string; TemplateVMID, Cores, MemoryMB, DiskGB int; FullClone bool}`
   - Handler steps:
     1. `nodeRepo.FindByID` — get node and build `NodeConn`
     2. `provisioner.NextVMID` if no `NewVMID` specified
     3. `domain.NewVirtualMachine(vmid, nodeID, customerID, name, cores, memoryMB, diskGB, notes)` — create record in `creating` status
     4. `vmRepo.Save` — persist immediately (so UI shows "Creating..." row)
     5. Publish `ProxmoxVMCreated` (status=creating)
     6. Launch goroutine: call `provisioner.CreateVM(ctx, conn, spec)` → get UPID → `provisioner.WaitForTask` → on success update status to `running` via `vmRepo.UpdateStatus` + publish `ProxmoxVMStatusChanged`; on failure update status to `unknown` + log error
   - Goroutine uses a fresh context (not request context — request will be gone)
   - Handler returns as soon as `vmRepo.Save` succeeds (status=creating)

2. [ ] Create `internal/modules/proxmox/app/commands/create_vm_test.go`
   - Stub `VMProvisioner` that records calls and returns a mock UPID
   - Stub repos
   - Verify: VM saved with `creating` status immediately
   - Verify: after goroutine completes (use short sleep or sync.WaitGroup helper), VM status = `running`

---

### Phase 8 — VM Suspend / Restart / Delete commands — status: open

**Goal:** Lifecycle operations for existing VMs. All async via same goroutine+task-polling pattern.

1. [ ] Create `internal/modules/proxmox/app/commands/suspend_vm.go`
   - `SuspendVMCommand{VMID string}` (internal UUID)
   - Guard: `vm.Status != VMStatusRunning` → `ErrVMNotRunning`
   - Update status → `VMStatusPaused` (optimistic) → `vmRepo.UpdateStatus`
   - Goroutine: `provisioner.SuspendVM` → wait → on success publish `ProxmoxVMSuspended`; on failure revert status to `running`

2. [ ] Create `internal/modules/proxmox/app/commands/restart_vm.go`
   - `RestartVMCommand{VMID string}`
   - Guard: `vm.Status != VMStatusRunning` → `ErrVMNotRunning`
   - Optimistic status update (keep `running` — reboot is brief), publish `ProxmoxVMRestarted` on success
   - Goroutine: `provisioner.RestartVM` → wait → publish event

3. [ ] Create `internal/modules/proxmox/app/commands/resume_vm.go`
   - `ResumeVMCommand{VMID string}` — complement to suspend
   - Guard: `vm.Status != VMStatusPaused` → error
   - Goroutine: `provisioner.StartVM` → wait → update to `running` → publish `ProxmoxVMResumed`

4. [ ] Create `internal/modules/proxmox/app/commands/delete_vm.go`
   - `DeleteVMCommand{VMID string}`
   - Guard: allow any non-deleting status (user confirmed in UI)
   - If `vm.Status == VMStatusRunning`: stop first (`provisioner.StopVM` → wait), then delete
   - If already stopped/paused: go straight to delete
   - Update status → `VMStatusDeleting` before starting goroutine
   - Goroutine: `provisioner.DeleteVM` → wait → `vmRepo.Delete` (hard delete) → publish `ProxmoxVMDeleted`
   - On failure: revert status to previous value

5. [ ] Create `internal/modules/proxmox/app/commands/assign_customer.go`
   - `AssignVMCustomerCommand{VMID, CustomerID string}` — link/unlink VM from customer
   - Pure DB operation (no Proxmox API call)
   - Publish `ProxmoxVMStatusChanged` so CRM tab refreshes

---

### Phase 9 — VM HTTP handlers + templates — status: open

**Goal:** VM list, detail page, action buttons (suspend/restart/delete), create form.

1. [ ] Add VM routes to `Handlers.RegisterRoutes` in `handlers.go`
   - `GET  /proxmox/vms` — VM list page (all VMs across all nodes)
   - `GET  /proxmox/vms/new` — create VM form page
   - `GET  /proxmox/vms/{id}` — VM detail page (status + actions)
   - `GET  /api/proxmox/vms` — VM list SSE (live updates)
   - `POST /api/proxmox/vms` — create VM SSE (signals: nodeID, customerID, name, templateVMID, cores, memoryMB, diskGB, storage, fullClone, notes)
   - `POST /api/proxmox/vms/{id}/suspend` — suspend VM
   - `POST /api/proxmox/vms/{id}/resume` — resume VM
   - `POST /api/proxmox/vms/{id}/restart` — restart VM
   - `DELETE /api/proxmox/vms/{id}` — delete VM (with confirm in UI)
   - `PUT /api/proxmox/vms/{id}/customer` — assign/unassign customer

2. [ ] Create `internal/modules/proxmox/adapters/http/vm_templates.templ`
   - `ProxmoxVMsPage(vms []VMReadModel, nodes []NodeReadModel)` — full page
   - `VMListTable(vms []VMReadModel)` — `id="proxmox-vms-table"` for SSE patching; columns: Name, Node, Customer, Status (badge), Cores, RAM, IP, Actions
   - `VMStatusBadge(status VMStatus)` — colour-coded: running=green, paused=yellow, creating=blue (pulse), stopped=grey, deleting=red (pulse), unknown=neutral
   - `VMDetailPage(vm VMReadModel, node NodeReadModel)` — detail with action buttons
   - `VMActionButtons(vm VMReadModel)` — suspend/resume/restart/delete conditionally rendered based on `vm.Status`
     - Suspend: only when `running`
     - Resume: only when `paused`
     - Restart: only when `running`
     - Delete: when not `deleting` — confirm dialog via `data-on:click="confirm('Delete VM?') && @delete(...)"`
   - `VMCreateForm(nodes []NodeReadModel)` — form: node selector, customer picker, template VMID, name, cores, memory, disk, storage, fullClone toggle
   - `VMRow(vm VMReadModel)` — `id="vm-row-{id}"` single row fragment for partial updates

3. [ ] Create `internal/modules/proxmox/adapters/http/vm_list_sse.go` (or add to handlers.go)
   - `vmListSSE` — opens `NewSSE`, subscribes `ProxmoxVMAll.String()`, on event re-queries all VMs and patches `#proxmox-vms-table`
   - Sends initial data on open

4. [ ] Create `internal/modules/proxmox/adapters/http/vm_detail_sse.go`
   - `vmDetailSSE` — subscribes `ProxmoxVMAll.String()`, filters by `aggregate_id == vmID`, patches `#vm-status-badge` and `#vm-action-buttons`
   - Operator sees live status: "Creating..." → "Running" without refresh

---

### Phase 10 — Customer CRM "VMs" tab — status: open

**Goal:** 9th tab on customer detail page showing VMs assigned to that customer.

1. [ ] Update `CRMTabBar` in `internal/modules/customer/adapters/http/templates.templ`
   - Add `vms int` parameter (9th) — pass `-1` from callers that don't have VM data yet
   - Tab key: `"vms"`, label: `"VMs"`, count badge from `vms` param

2. [ ] Update `crmLiveSSE` in `internal/modules/customer/adapters/http/handlers.go`
   - Add `listVMs invoicequeries.ListVMsForCustomerQuerier` interface field to CRM handler (or pass as func)
   - Or: subscribe to `ProxmoxVMAll` wildcard and re-query VMs on those events
   - Query VM count + render VM section, patch `#customer-vms-section`

3. [ ] Create `VMSection(vms []proxmoxqueries.VMReadModel)` in customer templates (or inline in `templates.templ`)
   - `id="customer-vms-section"` for SSE patching
   - Table: VMID, Name, Node, Status badge, Cores, RAM, IP, link to `/proxmox/vms/{id}`
   - Empty state: "No VMs assigned" with link to create a VM for this customer

4. [ ] Update `GET /customers/{id}` page handler to also query VMs for this customer
   - Pass VM list and count to template

5. [ ] Verify CRM tab signal `_crmTab` default remains `"tickets"` — no change needed to default tab

---

### Phase 11 — Sidebar nav — status: open

**Goal:** Add "Compute" collapsible group to sidebar with Proxmox nodes and VMs entries.

1. [ ] Edit `internal/infrastructure/http/templates/layout.templ`
   - Add `_navCompute` bool signal to `data-signals`, default `false` (collapsed)
   - Add **Compute** collapsible group between Network and System sections:
     ```
     Compute (toggle)
     ├── Nodes  → /proxmox/nodes
     └── VMs    → /proxmox/vms
     ```
   - Follow existing `_navNetwork` collapsible pattern exactly (same CSS classes, toggle button)

---

### Phase 12 — Wire into app — status: open

**Goal:** `wire_proxmox.go` follows exact pattern of `wire_network.go`.

1. [ ] Create `internal/app/wire_proxmox.go`
   - `proxmoxWired` struct with all handlers + repo fields
   - `wireProxmox(gdb, pub, sub, cfg) *proxmoxWired`
   - `nodeRepo := proxmoxpersistence.NewGormNodeRepository(gdb, []byte(cfg.ProxmoxEncKey))`
   - `vmRepo := proxmoxpersistence.NewGormVMRepository(gdb)`
   - `provisioner := proxmox.NewClient(false)` (TLS config from node `InsecureTLS` per-call)
   - Wire all command handlers and query handlers
   - Guard with `cfg.IsEnabled("proxmox")` — only register routes if enabled
   - `log.Printf("module enabled: proxmox")`

2. [ ] Edit `internal/app/app.go` (or its `Build()` function)
   - Call `wireProxmox(gdb, pub, sub, cfg)` and register routes

3. [ ] Run Goose migrations at startup for proxmox module
   - In `gormsqlite` DB init: `goose.Up(db, proxmoxmigrations.Migrations, "goose_proxmox")`
   - Follow same pattern as other modules in `internal/infrastructure/gormsqlite/`

4. [ ] Verify `go build ./...` is clean

---

### Phase 13 — Tests — status: open

**Goal:** Domain unit tests, command tests, HTTP handler tests, integration tests.

1. [ ] Create `internal/modules/proxmox/domain/vm_test.go`
   - Test: `NewVirtualMachine` validates required fields (VMID > 0, name non-empty, cores > 0, memory > 0)
   - Test: `Suspend()` transitions `running` → `paused`
   - Test: `MarkDeleting()` transition
   - Test: `AssignCustomer` / `UnassignCustomer`

2. [ ] Create `internal/modules/proxmox/domain/node_test.go`
   - Test: `NewProxmoxNode` validates required fields
   - Test: `Update` with empty `TokenSecret` preserves existing secret
   - Test: `ToConn` projection

3. [ ] Create `internal/modules/proxmox/app/commands/create_vm_test.go` (as noted in Phase 7)
   - Stub provisioner with controllable goroutine completion

4. [ ] Create `internal/modules/proxmox/adapters/http/handlers_test.go`
   - Test: `GET /proxmox/nodes` requires auth (or module-level permission)
   - Test: `POST /api/proxmox/nodes` creates node, returns SSE with node data
   - Test: `DELETE /api/proxmox/nodes/{id}` blocked when VMs exist
   - Test: `POST /api/proxmox/vms/{id}/suspend` guard — returns error SSE when VM not running

5. [ ] Run full test suite: `go test ./...` — all green

---

## Verification

```bash
# 1. Build
go build ./...

# 2. Tests
go test ./internal/modules/proxmox/...
go test ./internal/infrastructure/proxmox/...
go test ./...

# 3. Manual (with real Proxmox node):
# - Add a Proxmox node via UI (/proxmox/nodes/new)
# - Create a VM (clone from template) → see status "Creating..."
# - Wait for background task → status changes to "Running" without page refresh
# - Suspend VM → status "Paused" immediately reflected in list
# - Restart VM → stays "Running"
# - Delete VM → status "Deleting..." → row disappears on completion
# - Open customer detail → VMs tab shows assigned VM
# - Assign VM to customer from VM detail → CRM tab count updates

# 4. No Proxmox node? Verify with InsecureTLS node config pointing to a mock server
#    or disable proxmox module via EnabledModules config and verify no startup errors
```

---

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

---

## Progress Log

- 2026-04-20 08:21 — Plan created. 13 phases, ~60 actions. Proxmox VE REST API, port/adapter pattern mirrors network module, async task polling via goroutines + NATS events, 9th CRM tab.
- 2026-04-20 — Phase 2 complete (8c6f8bb). All domain tests green. Note: VMRepository has FindByNodeID added beyond plan (needed by delete_node guard).
