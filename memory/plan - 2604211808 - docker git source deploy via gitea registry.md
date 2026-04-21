---
tldr: Implement DockerApp entity — build pipeline (clone→build→push→run) with VVS UI for env/ports/volumes/networks and optional Gitea webhook
status: completed
---

# Plan: Docker Git Source Deploy via Gitea Registry

## Context

- Spec: [[spec - docker - git source deploy via gitea registry]]
- Extends: [[spec - docker - multi-node orchestrator with compose yaml and live logs]]

## Phases

### Phase 1 — Domain + Persistence — status: completed

1. [x] Define `DockerApp` entity and supporting types (`KV`, `PortMap`, `VolumeMount`)
   - => `internal/modules/docker/domain/app.go`
   - => Status constants: `idle|building|pushing|deploying|running|error|stopped`
   - => `DockerAppRepository` interface: `Save`, `FindByID`, `FindAll`, `Delete`
2. [x] Write migration `014_create_docker_apps.sql`
   - => `internal/modules/docker/migrations/014_create_docker_apps.sql`
3. [x] Implement `GormDockerAppRepository`
   - => `internal/modules/docker/adapters/persistence/gorm_app_repository.go`
   - => JSON marshal/unmarshal for slice fields, AES encrypt/decrypt RegPass via `emailcrypto`

### Phase 2 — Build Pipeline — status: completed

1. [x] Implement `BuildDockerAppHandler` — full pipeline in goroutine
   - => `internal/modules/docker/app/commands/app_commands.go`
   - => clone via `os/exec git clone --depth 1`, build via Docker SDK `ImageBuild`, push via `ImagePush`
   - => tags: `:latest` + `:YYYYMMDD-HHMMSS`
2. [x] `dirToTar` helper — tar build context from cloned dir via pipe goroutine
3. [x] `deployContainer` helper — stop/remove existing, ContainerCreate+Start
4. [x] `StopDockerAppHandler` + `RemoveDockerAppHandler`
5. [x] NATS subjects added: `DockerAppAll`, `DockerAppStatusChanged`, `DockerAppBuildLog`
   - => `internal/shared/events/subjects.go`

### Phase 3 — HTTP Handlers — status: completed

1. [x] All handlers in `app_handlers.go`: list, new/edit form, create, update, delete, build, stop, buildLogs SSE, webhook trigger, register webhook
   - => `internal/modules/docker/adapters/http/app_handlers.go`
   - => Form parsing: parseKVForm, parsePortsForm, parseVolumesForm helpers
2. [x] Gitea API webhook registration via `registerWebhook` handler

### Phase 4 — Templates — status: completed

1. [x] `AppListPage`, `AppTable`, `AppStatusBadge`, `AppInlineError`
2. [x] `AppFormPage` with Source / Build / Runtime cards
3. [x] `AppBuildLogLine`, `AppWebhookRegistered`
   - => `internal/modules/docker/adapters/http/app_templates.templ`
4. [x] `app_queries.go`: `ListDockerAppsHandler`, `GetDockerAppHandler`

### Phase 5 — Wire + Nav — status: completed

1. [x] `GormDockerAppRepository`, commands, queries, `AppHandlers` wired into `wire_docker.go`
2. [x] "Apps" nav item added under Docker group with `dockerAppNavIcon`
   - => `internal/infrastructure/http/templates/layout.templ`
3. [x] `appRoutes` registered in `builder.go`

### Phase 6 — E2E Tests — status: completed

1. [x] `e2e/docker-apps.spec.js` — 7 tests: page loads, table SSE, new app link, form fields (repo/branch/name/creds), build args, runtime section, cancel

## Verification

- Create app → form saves → app appears in list with status `idle`
- Build app → status transitions: building → pushing → deploying → running; image appears in Gitea Packages tab
- Env var `FOO=bar` → `docker exec <name> env | grep FOO` shows `FOO=bar`
- Port `8080:80/tcp` → `curl localhost:8080` reaches container
- Volume `/tmp/data:/data` → file persists after container restart
- Network attached → container visible on that network
- Redeploy → old container gone, new container running with fresh image
- Invalid credentials → status = error, ErrorMsg set, tmp dir cleaned
- Webhook POST → build triggered

## Progress Log

- 2026-04-21 18:xx — All 6 phases complete. Domain, migration, GORM repo, build pipeline, HTTP handlers, templates, wire, nav, e2e tests. Build: clean. Pre-existing wire_docker.go errors (Hetzner filters incomplete feature) not part of this task.
