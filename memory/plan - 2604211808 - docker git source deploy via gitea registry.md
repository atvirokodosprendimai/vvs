---
tldr: Implement DockerApp entity — build pipeline (clone→build→push→run) with VVS UI for env/ports/volumes/networks and optional Gitea webhook
status: active
---

# Plan: Docker Git Source Deploy via Gitea Registry

## Context

- Spec: [[spec - docker - git source deploy via gitea registry]]
- Extends: [[spec - docker - multi-node orchestrator with compose yaml and live logs]]

## Phases

### Phase 1 — Domain + Persistence — status: open

1. [ ] Define `DockerApp` entity and supporting types (`KV`, `PortMap`, `VolumeMount`)
   - `internal/modules/docker/domain/app.go`
   - Status constants: `idle|building|pushing|deploying|running|error|stopped`
   - `DockerAppRepository` interface: `Save`, `FindByID`, `FindAll`, `Delete`
2. [ ] Write migration `007_create_docker_apps.sql`
   - columns: id, name, repo_url, branch, reg_user, reg_pass, build_args, env_vars, ports, volumes, networks, restart_policy, container_name, image_ref, status, error_msg, last_built_at, created_at
3. [ ] Implement `GormDockerAppRepository`
   - JSON marshal/unmarshal for slice fields (BuildArgs, EnvVars, Ports, Volumes, Networks)
   - AES encrypt/decrypt `RegPass` using `DockerEncKey`

### Phase 2 — Build Pipeline — status: open

1. [ ] Implement `BuildDockerAppCmd` (application service)
   - status transitions published via NATS `isp.docker.app.status`
   - `git clone --depth 1 https://user:pass@host/repo tmpdir` via `os/exec`
   - build log lines published to NATS `isp.docker.app.build.{id}`
2. [ ] Implement `dockerBuild` helper — tar build context → `ImageBuild` Docker SDK
   - stream Docker daemon output lines to callback
   - image tags: `gitea.host/owner/repo:latest` + `gitea.host/owner/repo:YYYYMMDD-HHMMSS`
3. [ ] Implement `dockerPush` helper — `ImagePush` with base64 JSON registry auth
4. [ ] Implement `containerDeploy` helper
   - stop + remove existing container by `ContainerName` (ignore not-found)
   - `ContainerCreate` with Env, PortBindings, HostConfig.Binds, NetworkingConfig, RestartPolicy
   - `ContainerStart`
5. [ ] `StopDockerAppCmd` — stop container, set status = stopped
6. [ ] `RemoveDockerAppCmd` — stop + remove container, delete DB record

### Phase 3 — HTTP Handlers — status: open

1. [ ] List handler `GET /docker/apps` — SSE, publishes `#docker-apps-table`
2. [ ] Create/edit form handlers `GET /docker/apps/new`, `GET /docker/apps/{id}/edit`, `POST /docker/apps`, `PUT /docker/apps/{id}`
3. [ ] Build trigger `POST /docker/apps/{id}/build` — starts `BuildDockerAppCmd` goroutine, redirects to detail
4. [ ] Stop handler `POST /docker/apps/{id}/stop`
5. [ ] Delete handler `DELETE /docker/apps/{id}`
6. [ ] Build log SSE `GET /docker/apps/{id}/logs` — subscribes `isp.docker.app.build.{id}`, streams to `#app-build-log`
7. [ ] Webhook endpoint `POST /docker/apps/{id}/webhook` — triggers build pipeline (no auth in v1)
8. [ ] Register webhook handler `POST /docker/apps/{id}/register-webhook` — calls Gitea API to create hook
   - Gitea API: `POST /repos/{owner}/{repo}/hooks` with `active:true, type:"gitea", config:{url, content_type:"json"}`
   - VVS webhook URL derived from request host

### Phase 4 — Templates — status: open

1. [ ] App list page `GET /docker/apps` — table: name, status badge, last built, image ref, Build/Stop/Delete buttons
2. [ ] App form page — 4 cards:
   - **Source**: repo URL, branch, registry user, registry password (masked)
   - **Build**: build args K=V table with add/remove rows
   - **Runtime**: env vars K=V table, ports table, volumes table, networks multi-select, restart policy dropdown
3. [ ] App detail page — status badge, image ref, last built, build log terminal (dark #app-build-log, SSE auto-scroll), "Register Gitea Webhook" button

### Phase 5 — Wire + Nav — status: open

1. [ ] Wire `DockerApp` repository, commands, handlers into `wire_docker.go`
2. [ ] Add "Apps" nav item under Docker sidebar group (after existing Docker items)
3. [ ] Register new routes under `ModuleDocker` middleware

### Phase 6 — E2E Tests — status: open

1. [ ] Add `docker-apps.spec.js` Playwright tests
   - apps page loads
   - new app form has required fields (repo URL, branch, registry user/pass)
   - build/stop/delete buttons present

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

<!-- Updated after every completed action -->
