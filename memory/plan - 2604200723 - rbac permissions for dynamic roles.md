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

### Phase 1 — Permissions page: dynamic role loading — status: open

**Goal:** `/settings/permissions` shows all non-admin roles (including custom ones), not just hardcoded Operator + Viewer.

1. [ ] Update `PermissionsGrid` signature in `templates.templ:421`
   - Old: `templ PermissionsGrid(opPerms domain.PermissionSet, viewerPerms domain.PermissionSet)`
   - New: `templ PermissionsGrid(roles []domain.RoleDefinition, perms map[domain.Role]domain.PermissionSet)`
   - Body: `for _, rd := range roles { if rd.Name != domain.RoleAdmin { @permRoleSection(rd.DisplayName, rd.Name, perms[rd.Name]) } }`
   - `permRoleSection` is unchanged — it already takes `(label string, role domain.Role, ps domain.PermissionSet)`

2. [ ] Update `permissionsSSE` in `handlers.go:489`
   - Load all roles: `h.roleRepo.List(ctx)`
   - For each non-admin role, call `h.permRepo.FindByRole(ctx, role)` — on error fallback to `domain.DefaultPermissions(role)` (deny-all for unknown custom roles)
   - Build `perms map[domain.Role]domain.PermissionSet`
   - Call `sse.PatchElementTempl(PermissionsGrid(roles, perms))`

3. [ ] `templ generate && go build ./...` — verify no compile errors

### Phase 2 — Role creation: per-module permission matrix — status: open

**Goal:** The "Add Role" form on `/settings/roles` includes a module permission matrix. Submitting creates the role AND seeds its `role_module_permissions` rows atomically.

1. [ ] Expand `RolesPage` / `RoleRows` template in `templates.templ:625`
   - Add module permission section to the "Add role" form below `roleCanWrite` toggle
   - Signal names: flat, e.g. `rolePerm_customers_view`, `rolePerm_customers_edit`, ... for each module in `domain.AllModules`
   - Initialize all to `false` in `data-signals`
   - Render a compact 2-column grid (module name | view checkbox | edit checkbox)
   - Tie `can_edit` checkbox: `data-on:change` that also clears `can_view` if edit is disabled (edit implies view)

2. [ ] Extend `CreateRoleCommand` in `role_commands.go`
   ```go
   type ModulePermInput struct {
       CanView bool
       CanEdit bool
   }
   type CreateRoleCommand struct {
       Name        string
       DisplayName string
       CanWrite    bool
       Permissions map[domain.Module]ModulePermInput
   }
   ```

3. [ ] Inject `RolePermissionsRepository` into `CreateRoleHandler`
   - Constructor: `NewCreateRoleHandler(roles domain.RoleRepository, perms domain.RolePermissionsRepository) *CreateRoleHandler`
   - After `h.roles.Save(ctx, rd)`, iterate `cmd.Permissions` and call `h.perms.Save(ctx, &domain.RoleModulePermission{...})` for each
   - Skip modules not present in `cmd.Permissions` (they stay deny-all)

4. [ ] Update `createRoleSSE` in `handlers.go:523` to parse module permission signals
   - Extend `signals` struct with per-module fields matching the template signal names
   - Build `map[domain.Module]commands.ModulePermInput` from parsed signals
   - Pass to `CreateRoleCommand.Permissions`

5. [ ] Wire `permRepo` into `CreateRoleHandler` in `internal/app/app.go` (or wherever auth is wired)
   - Find where `commands.NewCreateRoleHandler(roleRepo)` is called — add `permRepo` arg

6. [ ] `templ generate && go build ./...` — verify

### Phase 3 — Verification & commit — status: open

1. [ ] Manual smoke test
   - Create new role "billing" with `can_write=true`, customers+invoices view+edit, rest denied
   - Go to `/settings/permissions` — billing role section visible with correct checkboxes
   - Create another role "read-ext" with `can_write=false`, all view-only
   - Assign billing role to a test user → user can edit customers/invoices, blocked on others
   - Assign read-ext → user blocked from all mutations (RequireWrite fails globally)

2. [ ] Commit Phase 1 separately, Phase 2 separately (two atomic commits)

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

<!-- Timestamped entries tracking work done. Updated after every action. -->
- 2026-04-20 07:23 — Plan created. Two-phase fix: dynamic permissions page + role creation with permission matrix.
