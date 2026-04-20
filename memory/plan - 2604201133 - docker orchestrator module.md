---
tldr: Docker orchestrator module — manage multi-node Docker hosts, deploy compose YAML services, stream live logs from the VVS admin UI
status: active
---

# Plan: Docker Orchestrator Module

## Context

- Spec: [[spec - architecture - system design and key decisions]] — hexagonal arch, single binary, NATS events, SSE frontend
- Spec: [[spec - docker - multi-node orchestrator with compose yaml and live logs]]
- Reference: uncloud.run (lightweight Docker management), Coolify.io (compose-based PaaS UI)
- Go libraries: `github.com/docker/docker/client`, `github.com/compose-spec/compose-go/v2`

### Design decisions baked in

- **Multi-node**: Local socket (`/var/run/docker.sock`) AND remote Docker hosts via TCP API (`tcp://host:2376`). Remote credentials (host URL + optional TLS certs) stored AES-256 encrypted in SQLite — same pattern as router/proxmox creds.
- **Deploy model**: docker-compose YAML — store raw YAML in DB, parse with `compose-go`, deploy via Docker SDK. No exec() to docker CLI.
- **Log streaming**: Docker log tailing (`ContainerLogs` with `Follow: true`) → goroutine → NATS subject `isp.docker.logs.{containerID}` → SSE handler → browser.
- **Module**: `internal/modules/docker/` — new nav group "Services", enabled via `--modules docker`.
- **YAML editor**: CodeMirror 5 via CDN (YAML mode + dark theme). Loaded only on the service form page.

---

## Phases

### Phase 1 — Spec + Domain — status: active

1. [x] `/eidos:spec docker - multi-node docker orchestrator with compose yaml and live logs`
   - cover: DockerNode entity, Service entity, lifecycle states, log streaming model, credential encryption, remote vs local node distinction
   - => [[spec - docker - multi-node orchestrator with compose yaml and live logs]] created (08cca1f)

2. [x] Domain: `DockerNode` entity
   - => `internal/modules/docker/domain/node.go`
   - => `NewDockerNode(name, host, isLocal)` — local auto-sets host to unix socket
   - => `Update(...)` only replaces TLS fields if non-empty (partial update safe)
   - => `DockerNodeRepository` interface

3. [x] Domain: `DockerService` entity
   - => `internal/modules/docker/domain/service.go`
   - => `ServiceStatus` typed const: deploying/running/stopped/error/removing
   - => `MarkRunning/MarkStopped/MarkError/MarkRemoving` transitions
   - => `UpdateYAML` resets status to deploying (re-deploy path)

4. [x] Migrations: `001_create_docker_nodes.sql` + `002_create_docker_services.sql` + `embed.go`

5. [x] Persistence: `GormDockerNodeRepository` + `GormDockerServiceRepository`
   - => `internal/modules/docker/adapters/persistence/`
   - => TLS cert/key/CA encrypted via `emailcrypto.EncryptPassword` (same pattern as proxmox)
   - => partial encrypt: only encrypt non-empty fields on Save

6. [x] NATS subjects: added to `internal/shared/events/subjects.go`
   - => DockerNodeAll/Created/Updated/Deleted, DockerServiceAll/Deployed/StatusChanged/Removed, DockerLogsLine (%s format)

### Phase 2 — Docker Client Adapter — status: open

7. [x] Docker client port (interface)
   - => `internal/modules/docker/domain/docker_client.go`
   - => `DockerClient` + `DockerClientFactory` interfaces
   - => `ContainerInfo{ID, Name, Image, Status, State, Ports}`
   - => `RemoveContainers(projectName)` removes all containers for a compose project

8. [ ] Docker client implementation
   - file: `internal/modules/docker/adapters/docker/client.go`
   - `type Client struct { inner *dockerclient.Client }`
   - `NewClient(host string, tlsCert, tlsKey, tlsCA []byte) (*Client, error)`
     - local: `dockerclient.NewClientWithOpts(client.WithHost("unix:///var/run/docker.sock"))`
     - remote TLS: `client.WithTLSClientConfig(caFile, certFile, keyFile)` + `client.WithHost(host)`
   - `Deploy`: parse compose YAML with `compose-go`, create networks → volumes → containers in dependency order
   - `StreamLogs`: `ContainerLogs(ctx, id, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})`

9. [ ] Commands: `CreateNodeCmd`, `UpdateNodeCmd`, `DeleteNodeCmd`, `DeployServiceCmd`, `StopServiceCmd`, `StartServiceCmd`, `RemoveServiceCmd`
   - file: `internal/modules/docker/app/commands/`
   - `DeployServiceCmd`: get node → build docker client → call `client.Deploy()` → save service status=running → publish `DockerServiceAll`
   - All node mutations publish `DockerNodeAll`

10. [ ] Log streamer service
    - file: `internal/modules/docker/app/services/log_streamer.go`
    - `LogStreamer{natsPub, dockerClientFactory}`
    - `Stream(ctx, nodeID, containerID string)` — goroutine: tails logs line by line, publishes `isp.docker.logs.<containerID>` per line
    - goroutine exits on ctx cancel

11. [ ] Queries: `ListNodesHandler`, `GetNodeHandler`, `ListServicesHandler`, `GetServiceHandler`, `ListContainersHandler(nodeID)`
    - file: `internal/modules/docker/app/queries/`

### Phase 3 — Admin UI: Node Management — status: open

12. [ ] HTTP handlers: node CRUD
    - file: `internal/modules/docker/adapters/http/handlers.go`
    - routes: `GET /docker/nodes`, `GET /docker/nodes/new`, `POST /docker/nodes`, `GET /docker/nodes/{id}/edit`, `PUT /docker/nodes/{id}`, `DELETE /docker/nodes/{id}`
    - `POST /docker/nodes/{id}/ping` — test connectivity, return success/error badge inline

13. [ ] Templ: node list + form
    - `DockerNodeListPage`, `DockerNodeTable`, `DockerNodeRow`
    - `DockerNodeFormPage`, `DockerNodeForm`
    - Form: name, host URL, isLocal toggle (hides TLS cert fields when local), tls_cert/key/ca textareas
    - Connectivity test button with inline result badge (no page reload)

### Phase 4 — Admin UI: Service Management — status: open

14. [ ] HTTP handlers: service CRUD + actions
    - `GET /docker/services` — list all services across nodes
    - `GET /docker/services/new` — deploy form (select node + YAML editor)
    - `POST /docker/services` — deploy
    - `GET /docker/services/{id}` — service detail (containers, status)
    - `POST /docker/services/{id}/stop`
    - `POST /docker/services/{id}/start`
    - `DELETE /docker/services/{id}` — remove + cleanup containers

15. [ ] Templ: service list + deploy form
    - `DockerServiceListPage`, `DockerServiceTable`, `DockerServiceRow` (status badge)
    - `DockerServiceFormPage` — node selector + CodeMirror YAML editor
      - Load CodeMirror 5 via CDN: codemirror.net/5 (JS+CSS+YAML mode+Dracula theme)
      - textarea with `id="compose-yaml"`, small JS init block to mount CodeMirror
      - On submit: sync CodeMirror value back to hidden textarea → Datastar sends signal
    - `DockerServiceDetailPage` — container table + start/stop/logs buttons

### Phase 5 — Live Log Streaming — status: open

16. [ ] SSE log endpoint
    - `GET /docker/services/{id}/logs` — SSE handler
    - Subscribes NATS `isp.docker.logs.*` wildcard for service's containers
    - Calls `logStreamer.Stream(ctx, nodeID, containerID)` for each container in the service
    - Each line → `PatchElementTempl` appending to `#log-output`

17. [ ] Templ: log viewer
    - `DockerLogPage` — dark terminal `#log-output` pre/code block
    - `DockerLogLine(ts, line string)` fragment — dim timestamp + text
    - Auto-scroll: inline `<script>` sets `scrollTop = scrollHeight` after each patch

### Phase 6 — Wiring + Nav — status: open

18. [ ] Wire: `internal/app/wire_docker.go`
    - `dockerWired{nodeRepo, serviceRepo, logStreamer, commands, queries}`
    - Register HTTP routes, inject enc key
    - Add to `builder.go` module chain + `allMigrations()`
    - Add `"docker"` to modules enum in `cmd/server/main.go` + `app/config.go`

19. [ ] Nav: "Services" group in sidebar
    - `internal/infrastructure/http/layout.templ` — new collapsible group
    - Items: Docker Nodes (`/docker/nodes`), Services (`/docker/services`)
    - Guarded by `RequireModuleAccess("docker")` middleware

20. [ ] Config: `--docker-enc-key` / `VVS_DOCKER_ENC_KEY`
    - `internal/app/config.go`: `DockerEncKey string`
    - `cmd/server/main.go`: flag + env var
    - `.env.example`: add to encryption keys section

---

## Verification

- [ ] `go test ./internal/modules/docker/...` — domain tests pass
- [ ] Add local Docker node → ping returns "OK"
- [ ] Add remote Docker node with bad creds → ping returns error inline (no crash)
- [ ] Deploy nginx compose YAML → container appears in service detail page
- [ ] Service detail lists containers with status badges
- [ ] Start/stop container → status updates live via SSE (no page refresh)
- [ ] `/docker/services/{id}/logs` streams live output in terminal viewer
- [ ] Delete service removes containers
- [ ] `--modules docker` excluded → Docker nav items hidden
- [ ] TLS credentials stored encrypted in SQLite (raw DB value not plaintext)

---

## Adjustments

<!-- none yet -->

## Progress Log

- **2026-04-20** Plan created
- **2026-04-20** Phase 1 complete: spec created + domain (node/service/client port) + migrations + persistence + NATS subjects
