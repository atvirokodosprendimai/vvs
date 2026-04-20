---
tldr: Manage multiple Docker hosts from the VVS admin UI — add nodes, deploy services from compose YAML, stream live container logs
---

# Docker Orchestrator

VVS can manage Docker containers alongside the ISP business it already runs. Operators add Docker nodes (local socket or remote TCP), paste a docker-compose YAML to deploy a service, and watch logs stream live in the admin UI — no separate tool needed.

## Target

ISP operators who self-host additional services (monitoring, billing daemons, bots, custom apps) on the same infrastructure VVS manages. Rather than SSH + `docker compose up` from a terminal, they get a lightweight panel inside VVS that covers the deploy-observe-restart loop.

Not a replacement for Kubernetes, Nomad, or Swarm. Single-digit node counts, single operator, compose-compatible workloads only.

## Behaviour

### Node management
- Operator registers a Docker node with a name and host URL
- Two node types:
  - **Local** — `unix:///var/run/docker.sock`; no credentials, VVS must share the Docker socket
  - **Remote** — `tcp://host:2376`; optional TLS (cert + key + CA); credentials stored encrypted
- Connectivity test is available at any time — returns "OK" or the error inline (no page reload)
- Deleting a node with active services is blocked until services are removed

### Service deployment
- Operator selects a node and pastes a docker-compose YAML
- VVS validates the YAML (required: `services` key present), then stores it and deploys
- Deployment creates networks → volumes → containers in dependency order (respects `depends_on`)
- Service enters status `deploying` → `running` on success, `error` on failure (error message stored)
- Each service tracks its current compose YAML; re-deploy replaces running containers

### Container lifecycle
- Operator can start, stop, or remove individual containers from the service detail page
- Removing a service stops and removes all its containers, networks (if created by this service), and the DB record
- Status badges update live via SSE — no manual refresh needed

### Live log streaming
- Operator opens the log view for a service → all containers in that service start streaming
- Logs appear in a dark terminal viewer, newest line at bottom, auto-scrolling
- Streaming stops when the page is closed (SSE connection drops, goroutine cancelled)
- Stderr and stdout both shown; timestamps optional (off by default, toggle available)

### Module gating
- All routes require the `docker` module to be enabled (`--modules docker`)
- Nav group "Services" is hidden when the module is disabled
- Module-level role permissions apply (same `RequireModuleAccess("docker")` middleware)

## Design

### Node types and credentials

```
DockerNode {
  ID        uuid
  Name      string          // human label
  Host      string          // unix:///... or tcp://host:port
  IsLocal   bool            // local socket — no creds needed
  TLSCert   []byte          // encrypted at rest (AES-256-GCM)
  TLSKey    []byte          // encrypted at rest
  TLSCA     []byte          // encrypted at rest
  Notes     string
}
```

Credential encryption uses the same AES-256-GCM pattern as `proxmox` and `network` modules. Key comes from `--docker-enc-key` / `VVS_DOCKER_ENC_KEY`. Without the key, remote TLS nodes cannot be used (local socket nodes are unaffected).

### Service entity

```
DockerService {
  ID          uuid
  NodeID      uuid
  Name        string     // derived from compose project name
  ComposeYAML string     // raw stored YAML, re-deployable
  Status      string     // deploying | running | stopped | error | removing
  ErrorMsg    string     // populated when status=error
}
```

Status is a simple string field — not a full state machine. Transitions happen inside commands:
- `DeployServiceCmd` → sets `deploying` then `running`/`error`
- `StopServiceCmd` → sets `stopped`
- `StartServiceCmd` → sets `running`/`error`
- `RemoveServiceCmd` → sets `removing`, then deletes record on success

### Deployment flow

```
HTTP POST /docker/services
  → parse YAML (compose-go/v2)
  → validate: services block present, image set for each
  → save DockerService{status: deploying}
  → publish DockerServiceAll
  → build DockerClient for node
  → create networks (if defined)
  → create volumes (if defined)
  → create + start containers (topological order via depends_on)
  → update status → running | error
  → publish DockerServiceStatus
```

`compose-go/v2` handles YAML parsing and dependency ordering. The VVS Docker client wraps `github.com/docker/docker/client` directly — no exec() to the docker CLI binary.

### Log streaming architecture

```
HTTP GET /docker/services/{id}/logs  (SSE)
  → for each container in service:
      logStreamer.Stream(ctx, nodeID, containerID)
        → goroutine: docker.ContainerLogs(Follow:true)
          → read lines from multiplexed stream
          → publish NATS isp.docker.logs.{containerID}
  → SSE handler subscribes NATS isp.docker.logs.*
  → each message → PatchElementTempl appends DockerLogLine to #log-output
  → context cancel (tab closed) → goroutines exit
```

The NATS subject per container (`isp.docker.logs.{containerID}`) allows multiple browser tabs to share one active tail. The goroutine is scoped to the SSE handler context — no orphan goroutines when the page closes.

### YAML editor

CodeMirror 5 (CDN, YAML mode + Dracula theme) loaded only on the service form page via inline `<script src>` tags. On form submit, a small JS snippet syncs the CodeMirror editor value into the hidden `<textarea>` that Datastar reads as a signal.

### NATS subjects

```go
DockerNodeAll       = "isp.docker.node.all"        // node list changed
DockerServiceAll    = "isp.docker.service.all"     // service list changed
DockerServiceStatus = "isp.docker.service.status"  // single service status change
DockerLogsLine      = "isp.docker.logs.%s"         // per containerID, use Format()
```

## Verification

- Add local node → connectivity test returns "OK"
- Add remote node with bad TLS → connectivity test shows error inline, no crash
- Deploy single-container compose (e.g. `nginx:alpine`) → container appears in service detail, status = running
- Deploy multi-service compose with `depends_on` → containers start in correct order
- Stop container → status badge updates live (SSE, no refresh)
- Open log view → stdout/stderr stream in terminal viewer, auto-scrolls
- Close log view tab → goroutine exits (no goroutine leak)
- Delete service → containers removed from Docker node
- `VVS_DOCKER_ENC_KEY` absent → remote TLS nodes cannot be created (error returned); local nodes unaffected
- `--modules docker` excluded → `/docker/` routes return 404, nav group absent

## Friction

- **Docker socket access**: Running VVS in a container requires mounting `/var/run/docker.sock` — a well-known docker-in-docker risk. Documented; operator's responsibility.
- **compose-go version skew**: `compose-go/v2` may not support every compose feature (e.g. `deploy.replicas`, Swarm-only fields). Unsupported fields are silently ignored by the library. Pure single-node compose features work.
- **Remote node TLS setup**: Enabling remote TCP with TLS on Docker daemon requires daemon config changes (`tlsverify`, `tlscacert` etc.). UI cannot do this for the operator — documented prerequisite.
- **No image pull progress**: Docker image pulls happen synchronously during deploy. Large images cause the deploy command to take tens of seconds. Status stays `deploying` during this time. No streaming progress in v1.

## Interactions

- Depends on [[spec - architecture - system design and key decisions]] — hexagonal module shape, NATS subjects, SSE pattern
- Follows credential encryption pattern established in proxmox and network modules (AES-256-GCM, enc key in config)
- Uses existing `RequireModuleAccess` middleware from auth module
- Publishes NATS events consumed by SSE handlers (same pattern as all other modules)

## Mapping

> [[internal/modules/docker/domain/node.go]]
> [[internal/modules/docker/domain/service.go]]
> [[internal/modules/docker/domain/docker_client.go]]
> [[internal/modules/docker/adapters/docker/client.go]]
> [[internal/modules/docker/adapters/http/handlers.go]]
> [[internal/modules/docker/adapters/http/templates.templ]]
> [[internal/app/wire_docker.go]]

## Future

{[!] Image pull progress — stream pull output to UI during deploy (requires separate pull step before container create)}
{[!] Compose re-deploy — diff existing containers against new YAML, recreate only changed services}
{[?] Multi-node service distribution — deploy different services on different nodes from one compose}
{[?] Resource limits display — show CPU/memory usage per container (docker stats API)}
{[?] Registry auth — stored encrypted registry credentials for private images}
