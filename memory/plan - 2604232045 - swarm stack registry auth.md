---
tldr: Wire ContainerRegistry into SwarmStack deploy — RegistryID field, migration, docker login injection, stack form dropdown
status: completed
---

# Plan: Swarm Stack Registry Auth

## Context

- Spec: [[spec - docker - container registry for swarm stack deploy]]
- Extends: [[spec - docker swarm - swarm cluster management with ssh transport and overlay macvlan networks]]

## Phases

### Phase 1 — Domain + Migration — status: completed

1. [x] Add `RegistryID string` to `SwarmStack` struct
   - => `internal/modules/docker/domain/swarm_stack.go`

2. [x] Migration `015_swarm_stack_registry_id.sql`
   - => `internal/modules/docker/migrations/015_swarm_stack_registry_id.sql`
   - => `ALTER TABLE swarm_stacks ADD COLUMN registry_id TEXT NOT NULL DEFAULT ''`

3. [x] Update GORM model for SwarmStack
   - => `internal/modules/docker/adapters/persistence/swarm_models.go`
   - => `RegistryID` added to `SwarmStackModel` + `toSwarmStackModel` / `toSwarmStackDomain`

### Phase 2 — Commands — status: completed

4. [x] Add `RegistryID` to stack commands + inject registry dep
   - => `DeploySwarmStackCommand.RegistryID` + `UpdateSwarmStackCommand.RegistryID`
   - => `DeploySwarmStackHandler.registryRepo` + `UpdateSwarmStackHandler.registryRepo`
   - => `registryRepo` declaration moved before stack commands in `wire_docker.go`

5. [x] Inject `docker login` into `composeUp`
   - => `composeUp(node, stackName, composeYAML string, reg *domain.ContainerRegistry, progress func(string)) error`
   - => reuses `shellQuote` from `deploy_vvs_component.go`
   - => login fail → immediate error, no compose up

6. [x] Set `RegistryID` on stack in deploy/update handlers
   - => `stack.RegistryID = cmd.RegistryID` in Deploy; conditional update in UpdateSwarmStack (only if cmd.RegistryID non-empty)

### Phase 3 — HTTP Layer — status: completed

7. [x] Inject `listRegistriesQuery` into `SwarmHandlers`
   - => `SwarmHandlers.listRegistries *queries.ListRegistriesHandler`
   - => `NewSwarmHandlers` updated

8. [x] Registry dropdown on SwarmStack form + badge on detail
   - => `SwarmStackFormPage` gets `registries []queries.RegistryReadModel` param
   - => `<select data-bind:registryid>` with `— none —` + registry options
   - => `swarmStackFormSignals` includes `registryid`
   - => `SwarmStackDetailPage` shows blue registry name badge when set
   - => `SwarmStackReadModel` gets `RegistryID` + `RegistryName`; `GetSwarmStackHandler` resolves name via `registryRepo`
   - => `templ generate` run; `swarm_templates_templ.go` regenerated

9. [x] Wire: pass `registryRepo` + `listRegistriesQuery` to SwarmHandlers
   - => `NewDeploySwarmStackHandler` + `NewUpdateSwarmStackHandler` + `NewGetSwarmStackHandler` + `NewSwarmHandlers` all updated
   - => `go build ./...` clean

## Verification

- `go build ./...` passes ✓
- Deploy stack with RegistryID set → SSH log includes `docker login {url}` line before compose up
- Deploy with invalid registry credentials → deploy fails with error, no compose up
- Deploy stack with no RegistryID → no `docker login` call (no regression for public images)
- Update stack YAML → stored RegistryID used without re-selecting
- Stack form shows registry dropdown; `— none —` clears RegistryID
- Stack detail shows registry name badge when set

## Adjustments

- Phase 3 also required updating `SwarmStackReadModel` to carry `RegistryID`/`RegistryName` and adding `registryRepo` to `GetSwarmStackHandler` — not in original plan but natural extension.

## Progress Log

- 2026-04-23 — Plan created
- 2026-04-23 — Phase 1 complete: domain field, migration, GORM model (fb71b02)
- 2026-04-23 — Phase 2 complete: composeUp registry auth, commands, wire (f0c2f6f)
- 2026-04-23 — Phase 3 complete: form dropdown, detail badge, read model, wire (264e8d0)
