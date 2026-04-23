---
tldr: Wire ContainerRegistry into SwarmStack deploy — RegistryID field, migration, docker login injection, stack form dropdown
status: active
---

# Plan: Swarm Stack Registry Auth

## Context

- Spec: [[spec - docker - container registry for swarm stack deploy]]
- Extends: [[spec - docker swarm - swarm cluster management with ssh transport and overlay macvlan networks]]

## Phases

### Phase 1 — Domain + Migration — status: open

1. [ ] Add `RegistryID string` to `SwarmStack` struct
   - `internal/modules/docker/domain/swarm_stack.go`
   - field: `RegistryID string // optional; empty = no registry auth`
   - `NewSwarmStack` signature unchanged; caller sets `stack.RegistryID = cmd.RegistryID` after construction

2. [ ] Migration `015_swarm_stack_registry_id.sql`
   - `internal/modules/docker/migrations/015_swarm_stack_registry_id.sql`
   - `ALTER TABLE swarm_stacks ADD COLUMN registry_id TEXT NOT NULL DEFAULT ''`
   - goose Up/Down

3. [ ] Update GORM model for SwarmStack
   - `internal/modules/docker/adapters/persistence/swarm_models.go`
   - add `RegistryID string \`gorm:"column:registry_id"\`` to GORM model
   - update `toDomain` / `fromDomain` mapping functions

### Phase 2 — Commands — status: open

4. [ ] Add `RegistryID` to stack commands + inject registry dep
   - `internal/modules/docker/app/commands/swarm_stack_commands.go`
   - `DeploySwarmStackCommand.RegistryID string`
   - `UpdateSwarmStackCommand.RegistryID string`
   - `DeploySwarmStackHandler` gets `registryRepo domain.ContainerRegistryRepository` dep
   - `UpdateSwarmStackHandler` gets `registryRepo` dep
   - update `NewDeploySwarmStackHandler` / `NewUpdateSwarmStackHandler` constructors

5. [ ] Inject `docker login` into `composeUp`
   - `composeUp` signature: `func composeUp(node, stackName, composeYAML string, reg *domain.ContainerRegistry, progress func(string)) error`
   - before compose up: if `reg != nil` → `echo {pass} | docker login {url} -u {user} --password-stdin 2>&1`
   - reuse `shellQuote` helper (already in commands package)
   - if login fails → return error immediately (no compose up)
   - update all `composeUp` call sites (Deploy + Update handlers)

6. [ ] Set `RegistryID` on stack in deploy/update handlers
   - `Handle` for DeploySwarmStackCommand: `stack.RegistryID = cmd.RegistryID`
   - resolve registry before calling `composeUp`: `if cmd.RegistryID != "" { reg, _ = h.registryRepo.FindByID(...) }`
   - `Handle` for UpdateSwarmStackCommand: same pattern; also update `stack.RegistryID` on update

### Phase 3 — HTTP Layer — status: open

7. [ ] Inject `listRegistriesQuery` into `SwarmHandlers`
   - `internal/modules/docker/adapters/http/swarm_handlers.go`
   - add `listRegistriesQuery *dockerqueries.ListRegistriesHandler` to `SwarmHandlers` struct
   - update `NewSwarmHandlers` constructor
   - pass to stack form page data

8. [ ] Registry dropdown on SwarmStack form + badge on detail
   - `internal/modules/docker/adapters/http/swarm_templates.templ`
   - `SwarmStackFormPage` page data: add `Registries []queries.RegistryReadModel`
   - render `<select name="registry_id">` with `— none —` + registry options
   - `SwarmStackDetailPage`: show registry name badge if `RegistryID` non-empty
   - run `templ generate`

9. [ ] Wire: pass `registryRepo` + `listRegistriesQuery` to SwarmHandlers
   - `internal/app/wire_docker.go`
   - `NewDeploySwarmStackHandler` — add `registryRepo` arg
   - `NewUpdateSwarmStackHandler` — add `registryRepo` arg
   - `NewSwarmHandlers` — add `listRegistriesQuery` arg
   - verify `go build ./...` passes

## Verification

- `go build ./...` passes after every phase
- Deploy stack with RegistryID set → SSH log includes `docker login {url}` line before compose up
- Deploy with invalid registry credentials → deploy fails with error, no compose up
- Deploy stack with no RegistryID → no `docker login` call (no regression for public images)
- Update stack YAML → stored RegistryID used without re-selecting
- Stack form shows registry dropdown; `— none —` clears RegistryID
- Stack detail shows registry name badge when set

## Adjustments

<!-- none yet -->

## Progress Log

- 2026-04-23 — Plan created
