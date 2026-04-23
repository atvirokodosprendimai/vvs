---
tldr: SwarmStack deploy authenticates to a private container registry (Gitea or any Docker-compatible registry) before running docker compose up on the target node
---

# Container Registry for Swarm Stack Deploy

Extension of the Docker Swarm module. Allows a SwarmStack to reference a registered `ContainerRegistry` so that VVS automatically runs `docker login` on the target node before executing `docker compose up`, enabling compose stacks to pull private images.

## Target

ISP operators who deploy compose stacks with images hosted in private registries — primarily Gitea's built-in container registry (`gitea.example.com/{owner}/{image}:{tag}`), but compatible with any Docker-protocol registry (Docker Hub private repos, GitHub Packages, etc.).

The operator registers the registry once in VVS, then selects it on any stack form. VVS handles credential injection at deploy time — no manual `docker login` on each node.

## Behaviour

### ContainerRegistry (already built)

`ContainerRegistry` entity already exists with full CRUD and UI (in the VVS Deploy section):
- Fields: `ID`, `Name`, `URL` (registry hostname, e.g. `gitea.example.com`), `Username`, `Password` (AES-256-GCM encrypted)
- Repository, commands, migration `011_container_registries.sql`, wire all complete

This spec does **not** change the registry entity or its management UI.

### Gitea Container Registry

Gitea uses the standard Docker registry protocol:
- Registry URL: `{gitea_host}` (e.g. `gitea.example.com`)
- Image reference: `gitea.example.com/{owner}/{image}:{tag}`
- Authentication: standard `docker login {host} -u {username} -p {password}`
- Login credentials: Gitea username + password (or Gitea access token as password)

No Gitea-specific code needed — the standard Docker login flow works.

### SwarmStack RegistryID

- `SwarmStack` gets an optional `RegistryID` field (FK to `ContainerRegistry.ID`)
- `RegistryID` is empty by default — stacks that only use public images need no registry
- On deploy and update: if `RegistryID` is set, VVS resolves the registry, decrypts credentials, and runs `docker login` on the target node before `docker compose up`
- If `docker login` fails → deploy fails immediately with the error message; no compose up attempted
- `RegistryID` is persisted so redeploys (update stack) reuse the same credentials without re-entering them

### Deploy Flow with Registry

```
composeUp(node, stackName, composeYAML, registry, progress)
  1. mkdir -p /opt/vvs/stacks/{name}
  2. write compose file to /opt/vvs/stacks/{name}/docker-compose.yml
  3. IF registry != nil:
       echo {password} | docker login {url} -u {username} --password-stdin 2>&1
       → error → fail immediately
       → success → continue
  4. docker compose -f {path} up -d --remove-orphans 2>&1
```

Registry credentials are passed in memory; never written to disk on VVS host or target node (beyond Docker's own `~/.docker/config.json` which `docker login` manages).

### UI Changes

- SwarmStack create/edit form gains a "Registry" dropdown
- Dropdown populated with all registered `ContainerRegistry` entries (name + URL)
- First option: `— none —` (empty, no login)
- If no registries configured: shows empty dropdown with hint "Add a registry in Docker → Deploy → Registries"
- Stack list/detail page: shows registry name badge if RegistryID is set

## Design

### Domain change: SwarmStack

```go
type SwarmStack struct {
    // ... existing fields ...
    RegistryID string // optional; empty = no registry auth
}
```

`NewSwarmStack` signature unchanged — RegistryID set via `stack.RegistryID = cmd.RegistryID` after construction.

### Migration

```sql
-- +goose Up
ALTER TABLE swarm_stacks ADD COLUMN registry_id TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; leave column
```

### Commands change

`DeploySwarmStackCommand` and `UpdateSwarmStackCommand` get `RegistryID string` field.

`DeploySwarmStackHandler` gets `registryRepo domain.ContainerRegistryRepository`.

`composeUp` signature:

```go
func composeUp(node *domain.SwarmNode, stackName, composeYAML string, reg *domain.ContainerRegistry, progress func(string)) error
```

When `reg != nil`, inject before compose up (same pattern as `deploy_vvs_component.go:198`):

```go
loginCmd := fmt.Sprintf("echo %s | docker login %s -u %s --password-stdin 2>&1",
    shellQuote(reg.Password), shellQuote(reg.URL), shellQuote(reg.Username))
```

`shellQuote` already exists in the commands package.

### Persistence change

`swarm_models.go` GORM model gets `RegistryID string` + `gorm:"column:registry_id"`.
`toDomain` / `fromDomain` mapping updated.

### HTTP/Template change

`SwarmStackFormPage` template gets a registry selector:
- Populated by `listRegistriesQuery` (already available via `VVSDeployHandlers`; needs to be passed to `SwarmHandlers` too)
- `SwarmHandlers` gets `listRegistriesQuery` dep injected
- Stack detail page shows registry name badge if set

## Verification

- Create stack with `RegistryID` set → deploy → SSH command includes `docker login {url}` before `docker compose up`
- Deploy with invalid credentials → deploy fails, `ErrorMsg` set, no compose up attempted
- Deploy with no `RegistryID` → deploy proceeds without any `docker login` call (no regression)
- Update stack YAML → redeploy uses stored `RegistryID` (no need to re-select)
- Stack form registry dropdown lists all registered registries; selecting `— none —` clears RegistryID
- Stack detail shows registry badge when set, blank when not

## Friction

- `docker login` writes to `~/.docker/config.json` on the target node. Credentials persist until explicit `docker logout`. Acceptable for long-running nodes.
- SQLite `ALTER TABLE ADD COLUMN` with `DEFAULT ''` works on SQLite 3.1.3+; goose migration safe.

## Interactions

- Extends [[spec - docker swarm - swarm cluster management with ssh transport and overlay macvlan networks]]
- Reuses `ContainerRegistry` entity and `shellQuote` helper from `deploy_vvs_component.go`
- `listRegistriesQuery` already wired via `VVSDeployHandlers`; needs to also reach `SwarmHandlers`

## Mapping

> [[internal/modules/docker/domain/swarm_stack.go]] — add RegistryID field
> [[internal/modules/docker/app/commands/swarm_stack_commands.go]] — composeUp registry param + handler dep
> [[internal/modules/docker/adapters/persistence/swarm_models.go]] — registry_id column mapping
> [[internal/modules/docker/adapters/http/swarm_handlers.go]] — listRegistriesQuery dep
> [[internal/modules/docker/adapters/http/swarm_templates.templ]] — registry dropdown + detail badge
> [[internal/modules/docker/migrations/015_swarm_stack_registry_id.sql]] — ALTER TABLE
> [[internal/app/wire_docker.go]] — pass registryRepo + listRegistriesQuery to SwarmHandlers

## Future

{[?] docker logout after deploy — explicitly remove credentials from node after compose up completes}
{[?] per-node registry login — log in on all cluster nodes at provision time, not just deploy time}
{[?] image pre-pull — explicit docker pull on all nodes before compose up for zero-downtime updates}
