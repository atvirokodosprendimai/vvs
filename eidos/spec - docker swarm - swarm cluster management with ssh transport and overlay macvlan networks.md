---
tldr: Extend the Docker module with Swarm cluster management — SSH transport, overlay/macvlan/bridge networks with reserved IPs, and stack deploy onto swarm
---

# Docker Swarm

Extension of the existing Docker orchestrator module. Adds Swarm cluster management (init or import), SSH-based node transport, multi-host network creation (overlay, macvlan, bridge), and stack deployment onto a swarm cluster.

## Target

Manage a Docker Swarm cluster entirely from the VVS admin UI:
- SSH into nodes without exposing the Docker TCP port publicly
- Initialise a new swarm or import an existing one (paste join tokens)
- Add/remove manager and worker nodes
- Create overlay, macvlan, and bridge networks with subnet/gateway and reserved IP assignments
- Deploy multi-service apps as stacks (compose YAML) onto the swarm

## Behaviour

### Transport: SSH

- Each `SwarmNode` stores an SSH private key (PEM, AES-256-GCM encrypted in SQLite)
- Docker SDK connects via `ssh://user@host:port` scheme using a custom dialer backed by `golang.org/x/crypto/ssh`
- SSH key is decrypted at runtime, used to dial, never written to disk
- SSHUser defaults to `root`; SSHPort defaults to 22
- The existing TLS/TCP path (DockerNode) is unchanged — swarm nodes use SSH only

### Swarm Cluster

- `SwarmCluster` entity: ID, name, advertise address of first manager, manager join token, worker join token (both AES-encrypted), notes, status (`initializing | active | degraded | unknown`)
- Two creation paths:
  - **VVS-init**: select an existing SwarmNode → VVS calls `SwarmInit` via Docker SDK → stores returned join tokens and advertise addr
  - **Import**: admin pastes manager join token, worker join token, and advertise addr → VVS stores them; subsequent node additions use the stored tokens
- Join tokens are stored encrypted; never displayed in plaintext after save (show masked: `SWMTKN-1-****`)
- Cluster status is derived from the manager node's swarm info on page load (not polled continuously)

### Swarm Nodes

- `SwarmNode` entity: ID, clusterID (optional — standalone SSH nodes are also valid), role (`manager | worker`), name, sshHost, sshUser, sshPort, sshKey (PEM, encrypted), advertiseAddr, swarmNodeID (Docker's internal node ID after join), status
- Adding a worker: VVS uses stored worker join token, calls `SwarmJoin` on the target node's Docker API via SSH transport
- Adding a manager: same with manager join token
- Removing a node: demote if manager → call `SwarmLeave` on the node → call `NodeRemove` on the manager
- Swarm node list shows: name, role badge (manager/worker), status (ready/down), IP, Docker node ID

### Networks

- `SwarmNetwork` entity: ID, clusterID (nullable), name, driver (`overlay | macvlan | bridge`), subnet (CIDR), gateway (optional), dhcpRangeStart, dhcpRangeEnd, parent (macvlan only), options (JSON map), reservedIPs (JSON array of `{ip, hostname, label}`), scope (`swarm | local`)
- **Overlay**: multi-host, requires active swarm cluster, `attachable: true` flag optional
- **Macvlan**: requires parent interface — free-text input (predefined in VVS, not pulled from Docker API). Requires subnet (CIDR) and gateway
- **Bridge**: single-host, standard Docker bridge, subnet and gateway optional

#### Subnet Split — DHCP pool vs Reserved range

A network subnet is divided into two halves in VVS:

- **Lower half (DHCP pool)**: defined by `dhcpRangeStart`/`dhcpRangeEnd`. Docker auto-assigns IPs from this range when containers start. Services/stacks always land here.
- **Upper half (Reserved range)**: IPs pre-allocated in VVS panel by admin. Each entry has an IP, hostname, and label. Used for infrastructure components (Traefik, internal DNS, gateways). VVS owns these definitions — they are not pulled from Docker.

Example for `10.100.0.0/17`:
- DHCP pool: `10.100.0.1 – 10.100.63.254` (lower half)
- Reserved: `10.100.64.0 – 10.100.127.254` (upper half, managed in VVS)
  - `10.100.100.1` — `traefik` — HTTP router
  - `10.100.100.100` — `dns` — internal DNS

Reserved IPs are stored in VVS only; Docker does not enforce them via IPAM. They serve as assignment documentation and as input for generated configs (see Traefik routing below).

### HTTP Routing — Traefik Integration

VVS generates a Traefik [file provider](https://doc.traefik.io/traefik/providers/file/) config from the service + network registry:

- Admin assigns a reserved IP to a Traefik instance on the network
- Each deployed stack/service can be tagged with a hostname route (e.g. `nginx.internal`)
- VVS generates a `traefik-routes.yml` (dynamic config) mapping hostnames → services by container name/IP
- Config is available for download or can be volume-mounted into the Traefik container via a stack YAML
- No live sync to Docker — admin re-downloads/re-deploys when routes change

`SwarmRoute` entity (lightweight, attached to SwarmStack): ID, stackID, hostname, port, stripPrefix (bool)

### Stack Deploy (Swarm)

- Reuses the existing compose YAML editor (CodeMirror 5)
- `SwarmStack` entity: ID, clusterID, name, composeYAML, status (`deploying | running | updating | error | removing`)
- Deploy calls `docker stack deploy --compose-file <yaml> <name>` via the manager node's Docker API
- Update: same command (stack deploy is idempotent — re-running updates services)
- Remove: `docker stack rm <name>`
- Stack list shows services and their replica counts (via `ServiceList` filtered by stack label)

### UI — Swarm Section

- New nav group "Swarm" in sidebar (collapsible, guarded by `ModuleDocker` permission)
- Nav items: Clusters, Nodes, Networks, Stacks
- Cluster detail page: summary (node count, status), node table, network table, stack table
- Node form: SSH connection test button (Ping via SSH → Docker info)
- Network form: subnet CIDR input auto-calculates suggested DHCP range (lower half) and reserved range (upper half); both boundaries are editable
- Reserved IPs table: IP / hostname / label columns, add/remove rows inline, no Docker API call — pure VVS data
- Stack form: optional "Routes" section — add hostname + port entries; generates `SwarmRoute` records
- Network detail page: "Download Traefik config" button generates `traefik-routes.yml` for all stacks on that network

## Design

### SSH Transport Adapter

```
adapters/dockerclient/ssh_client.go
```

- `NewSSH(host, user string, port int, privateKeyPEM []byte) (*Client, error)`
- Dials via `golang.org/x/crypto/ssh` using `ssh.ParsePrivateKey`
- Wraps SSH connection in an `http.Transport` using `sshDialer` that hijacks the HTTP client's dial function
- Docker SDK option: `dockerclient.WithHTTPClient` + custom transport
- No temp files; key bytes used in-memory and zeroed after dialing

### New Domain Entities

```
domain/swarm_cluster.go   — SwarmCluster, SwarmClusterRepository
domain/swarm_node.go      — SwarmNode, SwarmNodeRepository
domain/swarm_network.go   — SwarmNetwork (+ ReservedIP, DHCPRange), SwarmNetworkRepository
domain/swarm_stack.go     — SwarmStack, SwarmRoute, SwarmStackRepository
domain/traefik_config.go  — GenerateTraefikConfig(network *SwarmNetwork, stacks []SwarmStack) string
```

### New Migrations

```
migrations/003_create_swarm_clusters.sql
migrations/004_create_swarm_nodes.sql
migrations/005_create_swarm_networks.sql
migrations/006_create_swarm_stacks.sql
```

### Extended DockerClient Interface

New methods on `domain.DockerClient`:

```go
SwarmInit(ctx, advertiseAddr string) (managerToken, workerToken string, err error)
SwarmJoin(ctx, managerAddr, joinToken string) error
SwarmLeave(ctx context.Context, force bool) error
SwarmNodeList(ctx context.Context) ([]SwarmNodeInfo, error)
SwarmNodeRemove(ctx context.Context, nodeID string) error
NetworkCreate(ctx context.Context, req NetworkCreateRequest) (string, error)
NetworkList(ctx context.Context) ([]NetworkInfo, error)
InterfaceList(ctx context.Context) ([]string, error) // for macvlan parent dropdown
StackDeploy(ctx context.Context, name, composeYAML string) error
StackRemove(ctx context.Context, name string) error
StackList(ctx context.Context) ([]StackInfo, error)
```

### NATS Subjects (new)

```
isp.docker.swarm.cluster.*   — created/updated/deleted
isp.docker.swarm.node.*      — joined/removed/status_changed
isp.docker.swarm.network.*   — created/deleted
isp.docker.swarm.stack.*     — deployed/updated/removed/status_changed
```

### Enc Key

- Swarm entities (join tokens, SSH keys) reuse `DockerEncKey` (same AES key as TLS creds)
- No new env var needed

## Verification

- Add swarm node with SSH key → Ping returns Docker version (SSH dial successful)
- VVS-init: select node → cluster created, join tokens stored masked
- Import: paste tokens → add worker node → node appears in cluster node list with role=worker
- Create overlay network `10.100.0.0/17` → DHCP range auto-calculates lower half, reserved range shows upper half
- Add reserved IP `10.100.100.1 / traefik / HTTP router` in VVS panel → visible in reserved IP table, not fetched from Docker
- Deploy nginx stack → container gets auto-assigned IP from DHCP lower half
- Add route `nginx.local:80` to nginx stack → "Download Traefik config" generates valid `traefik-routes.yml`
- Create macvlan network → parent interface entered as free text (not from Docker API)
- Edit stack YAML → redeploy updates services (rolling update)
- Remove stack → services gone from `docker service ls`

## Friction

- SSH dialing in Go requires `golang.org/x/crypto/ssh` — not in current go.mod; needs `go get`
- Docker SDK has no built-in SSH transport helper; must implement custom `http.Transport` with SSH dial function
- `docker stack deploy` requires compose v3 format with `deploy:` keys; v2-only compose files will fail at the swarm scheduler
- SwarmJoin via Docker API requires the target node's Docker daemon to be reachable; ordering matters (manager must be up first)
- Macvlan networks require kernel macvlan support and appropriate NIC promiscuous mode — VVS cannot verify this; surface as a warning in the UI
- Docker does not enforce the VVS-defined DHCP range boundary — it allocates from the full subnet unless the network is created with an explicit `--ip-range` flag; VVS should pass `IPRange` in the `NetworkCreate` call to constrain Docker's pool to the lower half

## Interactions

- Extends [[spec - docker - multi-node orchestrator with compose yaml and live logs]]
- Reuses `DockerEncKey` config, `GormDockerNodeRepository` pattern, CodeMirror YAML editor
- `ModuleDocker` permission gates all swarm UI

## Mapping

> [[internal/modules/docker/domain/docker_client.go]]
> [[internal/modules/docker/adapters/dockerclient/client.go]]
> [[internal/modules/docker/adapters/http/handlers.go]]
> [[internal/modules/docker/adapters/http/templates.templ]]
> [[internal/app/wire_docker.go]]

## Future

{[!] SSH key rotation — re-encrypt stored keys when DockerEncKey changes}
{[?] Swarm service constraints UI — pin services to nodes by label}
{[?] Network usage map — show which services are attached to which networks}
{[?] Swarm secrets/configs management}
