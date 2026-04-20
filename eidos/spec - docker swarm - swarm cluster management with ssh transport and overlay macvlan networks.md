---
tldr: Extend the Docker module with Swarm cluster management — wgmesh WireGuard transport, SSH provisioning, overlay/macvlan networks with reserved IPs, Traefik routing, and stack deploy
---

# Docker Swarm

Extension of the existing Docker orchestrator module. Adds Swarm cluster management (init or import), wgmesh WireGuard mesh transport, SSH-based node provisioning, multi-host network creation (overlay, macvlan, bridge), and stack deployment onto a swarm cluster.

## Target

Manage a Docker Swarm cluster entirely from the VVS admin UI:
- SSH into nodes to provision wgmesh (WireGuard mesh VPN) — Swarm communicates over the mesh, not the public network
- Initialise a new swarm or import an existing one (paste join tokens)
- Add/remove manager and worker nodes; each node's wgmesh VPN IP is auto-discovered and stored
- Create overlay, macvlan, and bridge networks with subnet/gateway and reserved IP assignments
- Deploy multi-service apps as stacks (compose YAML) onto the swarm

## Behaviour

### Transport: wgmesh over SSH

Swarm nodes communicate over a WireGuard mesh VPN (`wgmesh`) rather than the public network. SSH is used only for provisioning; all Docker Swarm traffic travels over the encrypted mesh.

- `SwarmCluster.wgmeshKey` — 32+ character shared mesh key (AES-256-GCM encrypted in SQLite). All nodes in a cluster share one key.
- When a node is added, VVS SSH-deploys wgmesh as a docker compose stack on that node, passing the cluster key. wgmesh creates a `wgmesh0` WireGuard interface.
- VVS reads the assigned `wgmesh0` IP via SSH exec (`ip -4 addr show wgmesh0`) and stores it as `SwarmNode.vpnIP`
- All Swarm advertise/listen addresses and join targets use `vpnIP`, not `sshHost`
- Docker SDK still dials via SSH (`ssh://user@host:port`) for management calls; the Swarm data plane goes over WireGuard
- The existing TLS/TCP path (DockerNode) is unchanged — swarm nodes use SSH for mgmt + wgmesh for Swarm transport

#### wgmesh Compose Deploy

VVS renders and deploys the following compose on each node via SSH (parameterised with cluster key and node hostname):

```yaml
# managed by VVS — do not edit manually
services:
  wgmesh:
    image: ghcr.io/atvirokodosprendimai/wgmesh:latest
    cap_add: [NET_ADMIN]
    network_mode: host
    environment:
      WGMESH_KEY: "<clusterWgmeshKey>"
      HOSTNAME: "<nodeHostname>"
    restart: always
```

After deploy VVS polls `ip -4 addr show wgmesh0` (up to 30 s, 2 s interval) until an IP appears, then stores it.

### Swarm Cluster

- `SwarmCluster` entity: ID, name, wgmeshKey (AES-encrypted, 32+ chars), manager join token, worker join token (AES-encrypted), notes, status (`initializing | active | degraded | unknown`)
- Two creation paths:
  - **VVS-init**: enter cluster name + wgmesh key → select first manager node (must have vpnIP already) → VVS calls `SwarmInit(vpnIP)` → stores returned join tokens
  - **Import**: enter wgmesh key + paste manager/worker tokens → add nodes manually
- Join tokens are stored encrypted; never displayed in plaintext (masked: `SWMTKN-1-****`)
- wgmeshKey is never displayed after save (masked: `****`)
- Cluster status is derived from the manager node's swarm info on page load (not polled continuously)

### Swarm Nodes

- `SwarmNode` entity: ID, clusterID (optional — standalone SSH nodes also valid), role (`manager | worker`), name, sshHost, sshUser, sshPort, sshKey (PEM, AES-encrypted), vpnIP (wgmesh0 IP, auto-populated), swarmNodeID (Docker internal node ID after join), status
- **Node setup flow** (SSE-streamed, keeping connection open):
  1. SSH to `sshHost` → deploy wgmesh compose with cluster key
  2. Poll for `wgmesh0` IP → store as `vpnIP`
  3. Stream progress updates to UI; show VPN IP once acquired
- **Adding a worker**: VVS uses stored worker join token, calls `SwarmJoin(managerVpnIP, workerToken)` on target node via SSH transport
- **Adding a manager**: same with manager join token
- **Removing a node**: demote if manager → `SwarmLeave` on node → `NodeRemove` on manager
- Swarm node list shows: name, role badge, status, VPN IP (`wgmesh0`), Docker node ID

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
domain/swarm_cluster.go   — SwarmCluster (+ wgmeshKey), SwarmClusterRepository
domain/swarm_node.go      — SwarmNode (+ vpnIP), SwarmNodeRepository
domain/swarm_network.go   — SwarmNetwork (+ ReservedIP, DHCPRange), SwarmNetworkRepository
domain/swarm_stack.go     — SwarmStack, SwarmRoute, SwarmStackRepository
domain/traefik_config.go  — GenerateTraefikConfig(network *SwarmNetwork, stacks []SwarmStack) string
domain/wgmesh.go          — RenderWgmeshCompose(clusterKey, hostname string) string
                            PollVpnIP(ctx, sshClient, timeout) (string, error)
```

Key fields:
```go
type SwarmCluster struct {
    // ...
    WgmeshKey string // AES-encrypted, 32+ chars; shared by all nodes in cluster
}

type SwarmNode struct {
    // ...
    VpnIP      string // wgmesh0 IP — auto-populated after wgmesh deploy; empty until provisioned
    SshHost    string // physical/public IP for SSH provisioning only
}
```

### New Migrations

```
migrations/003_create_swarm_clusters.sql   — includes wgmesh_key column
migrations/004_create_swarm_nodes.sql      — includes vpn_ip column
migrations/005_create_swarm_networks.sql
migrations/006_create_swarm_stacks.sql
```

### Extended DockerClient Interface

New methods on `domain.DockerClient`:

```go
// vpnIP is the node's wgmesh0 IP — passed to all three addr flags:
//   --advertise-addr vpnIP   (what peers use to reach this node)
//   --data-path-addr vpnIP   (VXLAN overlay data plane binds here)
//   --listen-addr    vpnIP   (Swarm management API listens here)
// All Swarm traffic (mgmt + data plane) travels over the WireGuard mesh.
SwarmInit(ctx, vpnIP string) (managerToken, workerToken string, err error)
SwarmJoin(ctx, managerVpnIP, joinToken string) error
SwarmLeave(ctx context.Context, force bool) error
SwarmNodeList(ctx context.Context) ([]SwarmNodeInfo, error)
SwarmNodeRemove(ctx context.Context, nodeID string) error
NetworkCreate(ctx context.Context, req NetworkCreateRequest) (string, error)
NetworkList(ctx context.Context) ([]NetworkInfo, error)
StackDeploy(ctx context.Context, name, composeYAML string) error
StackRemove(ctx context.Context, name string) error
StackList(ctx context.Context) ([]StackInfo, error)
// NOTE: InterfaceList removed — macvlan parent is free-text in VVS, not from Docker API
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
- Add node (sshHost + sshKey + cluster) → wgmesh compose deployed → `wgmesh0` IP polled and stored as vpnIP → UI shows VPN IP
- VVS-init: select manager node (vpnIP must exist) → `SwarmInit(vpnIP)` with all three addr flags = vpnIP → cluster created, tokens stored masked
- Add worker: wgmesh deployed first → vpnIP obtained → `SwarmJoin(managerVpnIP, token)` → node joins over WireGuard mesh
- Import: paste tokens + enter manager vpnIP → add worker nodes → nodes appear in cluster list with vpnIP
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
- wgmesh IP polling: `wgmesh0` may take several seconds to appear after compose deploy; retry loop needed (30 s timeout, 2 s interval)
- `docker stack deploy` requires compose v3 format with `deploy:` keys; v2-only compose files will fail at the swarm scheduler
- SwarmJoin ordering: wgmesh must be running and vpnIP obtained before `SwarmJoin` — manager's wgmesh must be reachable on vpnIP
- Macvlan networks require kernel macvlan support and appropriate NIC promiscuous mode — VVS cannot verify this; surface as a warning in the UI
- Docker does not enforce the VVS-defined DHCP range boundary — pass `IPRange` in `NetworkCreate` to constrain Docker's pool to the lower half
- wgmesh key length: must be ≥ 32 chars; validate in VVS form before storing

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
