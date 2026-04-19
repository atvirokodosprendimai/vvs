---
tldr: Per-role module access control — each role has view/edit permissions per module; hidden from nav + 403 on direct URL
status: active
---

# Plan: Per-Role Module Access Control

## Context

- Auth spec: [[spec - auth - session based authentication.md]]
- Architecture spec: [[spec - architecture - system design and key decisions.md]]
- Related work: RBAC viewer role (commit 24dd0a1), RequireWrite middleware

**Decision:** Per-role (not per-user). Admin always full access (hardcoded). Operator/viewer roles get configurable module permissions. Denied modules: hidden from nav sidebar + 403 on direct URL access.

**Modules (13):** `customers`, `tickets`, `deals`, `tasks`, `contacts`, `invoices`, `products`, `payments`, `network`, `email`, `cron`, `audit_log`, `users`

---

## Phases

### Phase 1 — Domain + Migration + Repository - status: open

1. [ ] Define `Module` type + constants + `RoleModulePermission` struct in `internal/modules/auth/domain/permissions.go`
   - `type Module string` with 13 constants (customers, tickets, deals, tasks, contacts, invoices, products, payments, network, email, cron, audit_log, users)
   - `type RoleModulePermission struct { Role Role; Module Module; CanView bool; CanEdit bool }`
   - `type RolePermissionsRepository interface { FindByRole(ctx, role) ([]*RoleModulePermission, error); Save(ctx, p) error }`
   - Helpers: `PermissionSet` (map from Module → RoleModulePermission) with `CanView(m) bool` and `CanEdit(m) bool`
   - Admin bypass: `PermissionSet.CanView` returns true for any module when role == RoleAdmin

2. [ ] Migration `internal/modules/auth/migrations/003_role_module_permissions.sql`
   - Table: `role_module_permissions(role TEXT, module TEXT, can_view INT NOT NULL DEFAULT 1, can_edit INT NOT NULL DEFAULT 1, PRIMARY KEY(role, module))`
   - Seed defaults — operator: all modules view+edit; viewer: all modules view-only, no edit

3. [ ] GORM persistence `internal/modules/auth/adapters/persistence/permissions_repository.go`
   - `RolePermissionsRepository` impl using `gormsqlite.DB` / `ReadTX`
   - `FindByRole` → returns all rows for that role as `PermissionSet`
   - `Save` → upsert single row

4. [ ] Unit tests `internal/modules/auth/domain/permissions_test.go`
   - `PermissionSet.CanView` returns true when module present + can_view=true
   - `PermissionSet.CanView` returns false when module absent
   - Admin role always returns true (bypass)
   - `PermissionSet.CanEdit` respects can_edit flag

### Phase 2 — Permission Injection Middleware - status: open

1. [ ] `InjectModulePermissions(permRepo)` middleware in `internal/infrastructure/http/auth_middleware.go`
   - After `RequireAuth` stores user in context, this middleware loads the role's `PermissionSet` from DB
   - Admin: skip DB load, store full-access sentinel in context
   - Stores `PermissionSet` in context via typed key
   - Helper: `PermissionsFromCtx(ctx) PermissionSet`

2. [ ] Wire `InjectModulePermissions` after `RequireAuth` in `internal/infrastructure/http/router.go`
   - Requires passing `permRepo` to `NewRouter` — add as new param
   - Update `internal/app/app.go` to construct `permRepo` + pass to `NewRouter`

3. [ ] Tests `internal/infrastructure/http/auth_middleware_test.go`
   - InjectModulePermissions stores PermissionSet in context for non-admin
   - Admin gets full-access set regardless of DB

### Phase 3 — RequireModuleAccess Middleware + Route Wiring - status: open

1. [ ] `RequireModuleAccess(module Module)` middleware in `internal/infrastructure/http/auth_middleware.go`
   - Reads `PermissionSet` from context
   - Any method + module not in set with can_view=true → 403
   - Mutation methods (POST/PUT/PATCH/DELETE) + can_edit=false → 403
   - Admin: always passes (PermissionSet sentinel)

2. [ ] `ModuleNamed` interface in `internal/infrastructure/http/router.go`
   ```go
   type ModuleNamed interface {
       ModuleName() string
   }
   ```
   - `NewRouter` checks `m.(ModuleNamed)` — if present, wraps module routes in sub-group with `RequireModuleAccess(m.ModuleName())`
   - Modules without `ModuleName()` (e.g. auth, cron, audit) bypass module-level check

3. [ ] Add `ModuleName()` to each module handler that needs protection
   - `customers`, `tickets`, `deals`, `tasks`, `contacts` → customer/ticket/deal/task/contact handlers
   - `invoices`, `products`, `payments` → invoice/product/payment handlers
   - `network` → network handler
   - `email` → email handler
   - Pattern: `func (h *Handlers) ModuleName() string { return "customers" }`

4. [ ] Tests for RequireModuleAccess
   - can_view=false → GET 403
   - can_view=true, can_edit=false → GET 200, POST 403
   - can_view=true, can_edit=true → GET 200, POST 200
   - admin bypasses all

### Phase 4 — Nav Hiding (Server-Side via Context) - status: open

1. [ ] Thread `PermissionSet` into nav rendering in `internal/infrastructure/http/templates/layout.templ`
   - Add `PermissionsFromCtx(ctx)` call inside `Sidebar` templ (ctx is already available as first param)
   - Each nav group/item checks `perms.CanView(module)` before rendering
   - Admin: all items always visible

2. [ ] Map nav items to modules
   - CRM group: Customers→`customers`, Tickets→`tickets`, Deals→`deals`, Tasks→`tasks`
   - Finance group: Invoices→`invoices`, Products→`products`, Payments→`payments`
   - Network group: Devices/Prefixes→`network`
   - System group: Email→`email`, Cron→`cron`, Audit Log→`audit_log`, Users→`users`

3. [ ] Hide entire nav group if all its modules are hidden
   - `NavGroup` renders only if at least one child module is visible

### Phase 5 — Admin Configuration UI - status: open

1. [ ] Add `GET /settings/permissions` page + `GET /sse/permissions` + `POST /api/permissions/:role/:module` handlers to auth module
   - Page: `PermissionsPage()` templ — admin-only
   - SSE: renders `PermissionsTable(rows []RoleModulePermission)` for a role
   - API: toggles single (role, module, field) cell

2. [ ] `PermissionsPage` templ in `internal/modules/auth/adapters/http/templates.templ`
   - Two tabs (or sections): Operator | Viewer
   - 13-row table: Module name | View checkbox | Edit checkbox
   - Checkboxes POST on change: `data-on:change="@post('/api/permissions/{role}/{module}')"`
   - Signals: `{canView: bool, canEdit: bool}` per row

3. [ ] Add to sidebar nav under System as "Permissions" (admin-only link)

4. [ ] Wire routes + commands in `internal/modules/auth/adapters/http/handlers.go` and `app.go`

---

## Verification

```bash
# Phase 1
go test ./internal/modules/auth/domain/...

# Phase 2-3
go test ./internal/infrastructure/http/...
go build ./...

# Phase 4 — manual
# Login as operator with network module disabled:
# - /network → 403
# - Sidebar: Network group absent
# Login as admin:
# - /network → 200
# - All nav items visible

# Phase 5 — manual
# Admin navigates to /settings/permissions
# Unchecks "View" on Network for operator role → save
# Login as operator → /network → 403, nav hidden
```

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

- 2026-04-19 10:36 — Plan created. Per-role model chosen; admin hardcoded full access; hide+block approach.
