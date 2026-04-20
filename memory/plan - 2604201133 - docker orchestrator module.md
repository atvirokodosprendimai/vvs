---
tldr: Docker orchestrator module — manage multi-node Docker hosts, deploy compose YAML services, stream live logs from the VVS admin UI
status: completed
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

### Phase 2 — Docker Client Adapter — status: completed

7. [x] Docker client port (interface)
   - => `internal/modules/docker/domain/docker_client.go`
   - => `DockerClient` + `DockerClientFactory` interfaces
   - => `ContainerInfo{ID, Name, Image, Status, State, Ports}`
   - => `RemoveContainers(projectName)` removes all containers for a compose project

8. [x] Docker client implementation
   - => `internal/modules/docker/adapters/dockerclient/client.go`
   - => `Factory.ForNode(node)` builds client from domain node (no encKey leakage)
   - => TLS from PEM bytes via `tls.X509KeyPair` + custom `http.Transport` (no temp files)
   - => `Deploy`: compose-go `loader.LoadWithContext` → `graph.InDependencyOrder` → NetworkCreate → VolumeCreate → ContainerCreate+Start
   - => `External` fields: `bool(netCfg.External)` not `.External.External` (compose-go v2 External is a bool type)
   - => `ReadMultiplexLine` helper strips Docker 8-byte multiplex frame header from log stream

9. [x] Commands: node_commands.go + service_commands.go
   - => `internal/modules/docker/app/commands/`
   - => `DeployServiceHandler` saves status=deploying first, then deploys, then updates to running/error
   - => DeleteNode checks for active services first (ErrNodeHasServices)
   - => RemoveService: marks removing → removes containers → deletes DB record

10. [x] Log streamer service
    - => `internal/modules/docker/app/services/log_streamer.go`
    - => `Stream(ctx, node, containerID)` spawns goroutine; exits on context cancel
    - => uses `ReadMultiplexLine` to strip Docker multiplex header

11. [x] Queries: `ListNodesHandler`, `GetNodeHandler`, `ListServicesHandler`, `GetServiceHandler`
    - => `internal/modules/docker/app/queries/queries.go`
    - => `NodeReadModel.HasTLS` = len(TLSCert) > 0 (never exposes raw cert bytes)
    - => `ServiceReadModel` joins NodeName from node repo

### Phase 3 — Admin UI: Node Management — status: completed

12. [x] HTTP handlers: node CRUD
    - => `internal/modules/docker/adapters/http/handlers.go`
    - => routes: GET/new/edit pages + GET/POST/PUT/DELETE/ping SSE API
    - => `requireAdmin` guard on all mutations

13. [x] Templ: node list + form
    - => `DockerNodesPage`, `DockerNodeTable`, `DockerNodeRow`
    - => `DockerNodeFormPage`, `DockerNodeForm`
    - => `data-show="!$isLocal"` hides TLS cert textareas for local nodes
    - => `DockerPingResult(nodeID, ok, msg)` replaces `#ping-result-{id}` inline

### Phase 4 — Admin UI: Service Management — status: completed

14. [x] HTTP handlers: service CRUD + actions
    - => handlers merged with Phase 3 in same file (handlers.go)
    - => `serviceLogsSSE`: fans in Docker goroutines directly to SSE channel (no NATS)

15. [x] Templ: service list + deploy form
    - => `DockerServicesPage`, `DockerServiceTable`, `DockerServiceRow` with status badge
    - => `DockerServiceFormPage` / `DockerServiceForm` — node selector + CodeMirror 5 YAML
    - => CodeMirror `onChange` syncs to textarea + dispatches 'input' event for Datastar
    - => `DockerServiceDetailPage` — containers table + start/stop/logs/remove actions

### Phase 5 — Live Log Streaming — status: completed

16. [x] SSE log endpoint
    - => `GET /api/docker/services/{id}/logs` — direct Docker fan-in (not NATS)
    - => goroutine per container → logCh channel → SSE PatchElementTempl

17. [x] Templ: log viewer
    - => `DockerLogPage` — dark terminal `#log-output` pre block, `min-height: calc(100vh-180px)`
    - => `DockerLogLine(line)` — `data-patch-mode="append"` on `#log-output`
    - => Auto-scroll: MutationObserver watches `#log-output` childList, scrollTop = scrollHeight

### Phase 6 — Wiring + Nav — status: completed

18. [x] Wire: `internal/app/wire_docker.go`
    - => `dockerWired{routes}` — nc passed for LogStreamer NATSPublisher interface
    - => `&dockerclientadapter.Factory{}` (value, not constructor)
    - => wired in builder.go `New()` + `collectRoutes()` + `allMigrations()`

19. [x] Nav: "Services" group in sidebar
    - => `internal/infrastructure/http/templates/layout.templ` — new collapsible group after Compute
    - => `_navDocker` signal persisted to localStorage; `dockerVisible` from `ModuleDocker`
    - => Items: Nodes (`/docker/nodes`), Services (`/docker/services`)

20. [x] Config: `--docker-enc-key` / `VVS_DOCKER_ENC_KEY`
    - => `internal/app/config.go`: `DockerEncKey string`
    - => `cmd/server/main.go`: flag + env var wired
    - => `.env.example`: `VVS_DOCKER_ENC_KEY=` added

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
- **2026-04-20** Phase 2 complete: docker client adapter (compose-go + docker SDK), log streamer, node/service commands, queries
- **2026-04-20** Phases 3-6 complete: HTTP handlers, templ templates, wire_docker.go, ModuleDocker, nav group — full build clean (ee64c84)
