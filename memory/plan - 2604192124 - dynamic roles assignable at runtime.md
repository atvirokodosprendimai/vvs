---
tldr: Refactor hardcoded admin/operator/viewer roles into a DB-managed roles table — admins can create custom roles, assign them to users, and configure per-module permissions per role
status: completed
---

# Plan: Dynamic roles assignable at runtime

## Context

- Spec: [[spec - auth - session based authentication.md]] (current auth spec — roles section is out of date after this plan)
- Related: `internal/modules/auth/domain/user.go` — `Role` type, `ValidRole()`, `CanWrite()`
- Related: `internal/modules/auth/domain/permissions.go` — `RoleModulePermission`, `DefaultPermissions()`
- Related: `internal/modules/auth/migrations/003_role_module_permissions.sql` — current seeded roles

### Current state

Roles are hardcoded constants: `RoleAdmin`, `RoleOperator`, `RoleViewer`.  
`ValidRole()` only accepts those 3.  
`CanWrite()` hardcodes `role == admin || role == operator`.  
`DefaultPermissions()` hardcodes operator/viewer templates.  
`role_module_permissions` table has TEXT role column but no `roles` FK table.

### Target state

- `roles` table: `(name TEXT PK, display_name TEXT, is_builtin BOOL, can_write BOOL)`
- Admin UI: `/settings/roles` — create / delete custom roles, set display name + write flag
- User create/edit: role dropdown loaded from DB, not hardcoded options
- Permissions UI: shows all roles from DB (not just operator/viewer)
- `ValidRole()` replaced by app-layer DB check
- `CanWrite()` reads `is_write_role bool` field on User (populated from roles table JOIN)
- Built-in roles (admin/operator/viewer) flagged `is_builtin=true` — cannot be deleted

---

## Phases

### Phase 1 — Migration: roles table — status: completed

1. [ ] Write migration `007_dynamic_roles.sql`
   - `roles (name TEXT PK, display_name TEXT, is_builtin INTEGER NOT NULL DEFAULT 0, can_write INTEGER NOT NULL DEFAULT 1)`
   - Seed 3 builtin rows: admin (can_write=1), operator (can_write=1), viewer (can_write=0)
   - Also seed missing modules for existing roles in `role_module_permissions`:
     reports + iptv rows for operator and viewer (were missing from 003)
   - Add FK note: `role_module_permissions.role` references `roles.name` (soft FK — SQLite doesn't enforce by default, but keep consistent)

### Phase 2 — Domain: RoleDefinition + User.IsWriteRole — status: completed

1. [ ] Add `RoleDefinition` struct and `RoleRepository` interface in `domain/`
   - `RoleDefinition{Name Role, DisplayName string, IsBuiltin bool, CanWrite bool}`
   - `RoleRepository` interface: `List(ctx) ([]RoleDefinition, error)`, `FindByName(ctx, Role) (*RoleDefinition, error)`, `Save(ctx, *RoleDefinition) error`, `Delete(ctx, Role) error`
   - New file: `internal/modules/auth/domain/role.go`

2. [ ] Update `User` struct + `CanWrite()` method
   - Add `IsWriteRole bool` field to `User` (populated from DB join, not calculated from role name)
   - Change `CanWrite() bool` to return `u.IsWriteRole`
   - Update `NewUser()` and `ChangeRole()`: remove `ValidRole()` check (move to app layer)
   - Keep `RoleAdmin/Operator/Viewer` constants — they're still valid role name values

### Phase 3 — Persistence: GormRoleRepository + User JOIN — status: completed

1. [ ] Implement `GormRoleRepository` in `adapters/persistence/role_repository.go`
   - List, FindByName, Save, Delete (builtin guard: return error if `is_builtin=true`)
   - GORM model: `RoleModel{Name, DisplayName, IsBuiltin, CanWrite}`

2. [ ] Update `GormUserRepository` to JOIN roles table
   - `FindByID`, `FindByUsername`, `FindAll` — LEFT JOIN `roles` on `users.role = roles.name`
   - Populate `User.IsWriteRole` from `roles.can_write`
   - Fallback: if role not found in DB, default to `IsWriteRole=false`

### Phase 4 — App commands: role CRUD — status: completed

1. [ ] Add `CreateRoleCommand` + `DeleteRoleCommand` + handlers
   - `CreateRole{Name, DisplayName, CanWrite}` → validate name non-empty, not duplicate, slug-safe
   - `DeleteRole{Name}` → reject if `is_builtin=true` or if any users have this role
   - File: `internal/modules/auth/app/commands/role_commands.go`

2. [ ] Update `CreateUserCommand` and `UpdateUserCommand` validation
   - Replace `ValidRole(role)` check with `roleRepo.FindByName(ctx, role)` — return error if not found
   - Pass `RoleRepository` to both handlers (new constructor param)

### Phase 5 — HTTP: roles admin UI + updated user/permissions UI — status: completed

1. [ ] Add `/settings/roles` CRUD endpoint + template
   - GET renders role list + "Add Role" form (name, display_name, can_write toggle)
   - POST `/api/roles` — create role (SSE)
   - DELETE `/api/roles/{name}` — delete role (SSE, blocks builtin)
   - Template: `RolesPage(roles []domain.RoleDefinition)` in `auth/adapters/http/templates.templ`
   - Nav: add "Roles" link under Settings sidebar group

2. [ ] Update user create/edit modal: role dropdown from DB
   - `CreateUserPage` and user edit modal: pass `[]domain.RoleDefinition` to template
   - Dropdown replaces hardcoded `<option>` values for admin/operator/viewer

3. [ ] Update permissions UI: load all roles from DB
   - `PermissionsPage` currently only shows operator + viewer
   - Change to load all roles where `name != 'admin'` (admin always full access)
   - Pass `[]domain.RoleDefinition` to template

### Phase 6 — Wire + tests — status: completed

1. [ ] Wire `GormRoleRepository` in `internal/app/wire_auth.go`
   - Inject into user commands + auth HTTP handlers + permissions handler
   - Add to `authWired` struct

2. [ ] Domain tests for `RoleDefinition` and updated `User.CanWrite()`
   - `TestRoleDefinition_CanWrite` — builtin + custom roles
   - `TestUser_CanWriteFromIsWriteRole` — field propagation

3. [ ] Integration test: create custom role → assign to user → verify permissions
   - Create role "billing" (can_write=true)
   - Assign to test user
   - Verify `FindByID` returns user with `IsWriteRole=true`
   - Verify `RequireWrite` middleware allows mutations for that user

---

## Verification

```bash
# Build + domain tests
go test ./internal/modules/auth/...

# Integration test
go test ./internal/app/... -run TestDynamicRole

# Manual smoke test
# 1. Go to /settings/roles — see admin/operator/viewer listed as builtin
# 2. Create new role "billing-team" (can_write=true)
# 3. Create user, role dropdown shows billing-team
# 4. Assign billing-team to user → user can mutate (RequireWrite passes)
# 5. Create role "read-only-extern" (can_write=false)
# 6. Assign to user → user blocked from mutations
# 7. Try to delete admin role → rejected (builtin)
# 8. Permissions UI shows billing-team and read-only-extern tabs
```

---

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

---

## Progress Log

<!-- Timestamped entries tracking work done. Updated after every action. -->
