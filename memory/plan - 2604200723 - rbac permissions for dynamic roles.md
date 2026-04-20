---
tldr: Fix two RBAC gaps — permissions page shows only hardcoded Operator+Viewer (custom roles invisible), and role creation has no permission setup (new roles start deny-all with no UI path to configure them)
status: active
---

# Plan: RBAC permissions for dynamic roles

## Context

- Related plan: [[plan - 2604192124 - dynamic roles assignable at runtime]] — marked completed, but Phase 5 action 3 ("Update permissions UI: load all roles from DB") was NOT implemented
- `internal/modules/auth/domain/role.go` — `RoleDefinition{Name, DisplayName, IsBuiltin, CanWrite}`
- `internal/modules/auth/domain/permissions.go` — `RoleModulePermission`, `PermissionSet`, `RolePermissionsRepository`
- `internal/modules/auth/adapters/http/templates.templ:421` — `PermissionsGrid` hardcodes Operator + Viewer only
- `internal/modules/auth/adapters/http/handlers.go:489` — `permissionsSSE` fetches only those two roles
- `internal/modules/auth/app/commands/role_commands.go` — `CreateRoleCommand` has no `Permissions` field; `CreateRoleHandler` never seeds `role_module_permissions`

### Current gaps

1. **Permissions page blind to custom roles** — `PermissionsGrid(opPerms, viewerPerms)` signature hardcodes two roles; any custom role created via `/settings/roles` is invisible and unconfigurable
2. **New roles start with deny-all, no setup path** — `CreateRoleHandler` only saves to `roles` table; no `role_module_permissions` rows seeded; user must somehow know to go to permissions page (which won't show the new role)

### Decisions

- `RoleDefinition.CanWrite` kept as **global write killswitch** — blocks all mutations regardless of per-module `can_edit`
- Role creation form includes **per-module view/edit checkboxes** (not post-creation step)
- Admin role always hardcoded full-access — never appears in permissions grid

---

## Phases

### Phase 1 — Permissions page: dynamic role loading — status: completed

**Goal:** `/settings/permissions` shows all non-admin roles (including custom ones), not just hardcoded Operator + Viewer.

1. [x] Update `PermissionsGrid` signature in `templates.templ`
   - => New: `templ PermissionsGrid(roles []domain.RoleDefinition, perms map[domain.Role]domain.PermissionSet)`
   - => Uses `roleOptionLabel(rd)` for display name fallback
2. [x] Update `permissionsSSE` in `handlers.go` — calls `roleRepo.List()`, fetches perms for each non-admin role, falls back to `DefaultPermissions` on error
3. [x] `templ generate && go build ./...` — clean

### Phase 2 — Role creation: per-module permission matrix — status: completed

**Goal:** The "Add Role" form on `/settings/roles` includes a module permission matrix. Submitting creates the role AND seeds its `role_module_permissions` rows atomically.

1. [x] `RolesPage` form expanded with module permission matrix
   - => Signal names: `rolePerm{Module}View` / `rolePerm{Module}Edit` (camelCase PascalCase module)
   - => `data-bind={ viewSig }` value form → exact camelCase signal match
   - => Edit checkbox: `data-on:change` auto-sets view=true when edit checked
   - => Helpers: `moduleSignalBase`, `rolePermViewSignal`, `rolePermEditSignal`, `rolePermInitSignals`
2. [x] `ModulePermInput` + `Permissions map[domain.Module]ModulePermInput` added to `CreateRoleCommand`
3. [x] `CreateRoleHandler` injects `permRepo`; seeds `role_module_permissions` after role save
4. [x] `createRoleSSE` parses via `map[string]interface{}` — extracts per-module signals dynamically
5. [x] `wire_auth.go` passes `permRepo` to `NewCreateRoleHandler`
6. [x] `go test ./internal/modules/auth/...` — all pass

### Phase 3 — Verification & commit — status: open

1. [ ] Manual smoke test
   - Create new role "billing" with `can_write=true`, customers+invoices view+edit, rest denied
   - Go to `/settings/permissions` — billing role section visible with correct checkboxes
   - Create another role "read-ext" with `can_write=false`, all view-only
   - Assign billing role to a test user → user can edit customers/invoices, blocked on others
   - Assign read-ext → user blocked from all mutations (RequireWrite fails globally)

2. [x] Committed as `6272ee3` — phases 1+2 combined (template changes to same files made splitting impractical)

---

## Verification

```bash
templ generate && go build ./...
go test ./internal/modules/auth/...

# Manual:
# /settings/permissions — all roles show (not just operator/viewer)
# /settings/roles — Add Role form has module permission matrix
# Create role → permissions page shows it immediately
# Assign role to user → access enforced correctly
```

---

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

---

## Progress Log

- 2026-04-20 07:23 — Plan created. Two-phase fix: dynamic permissions page + role creation with permission matrix.
- 2026-04-20 — Phase 1+2 implemented. Commit `6272ee3`. All auth tests pass. Phase 3 (manual smoke test) pending.
