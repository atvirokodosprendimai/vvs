---
tldr: Implement Docker Swarm cluster management — wgmesh WireGuard transport, SSH provisioning, overlay/macvlan networks with subnet split, Traefik routing, stack deploy
status: active
---

# Plan: Docker Swarm cluster with wgmesh VPN transport

## Context

- Spec: [[spec - docker swarm - swarm cluster management with ssh transport and overlay macvlan networks]]
- Extends: [[spec - docker - multi-node orchestrator with compose yaml and live logs]]
- Prior plan: `memory/plan - 2604201133 - docker orchestrator module.md` (completed)

## Phases

### Phase 1 - Foundation: deps + domain + migrations + SSH transport - status: open

1. [x] Add `golang.org/x/crypto/ssh` dependency
   - `go get golang.org/x/crypto/ssh`
   - verify `go build ./...` still passes
   - => upgraded x/crypto v0.49.0 → v0.50.0; build clean

2. [x] Domain entities: SwarmCluster, SwarmNode
   - `domain/swarm_cluster.go` — SwarmCluster{ID, Name, WgmeshKey, ManagerToken, WorkerToken, Notes, Status}, SwarmClusterRepository interface
   - `domain/swarm_node.go` — SwarmNode{ID, ClusterID, Role, Name, SshHost, SshUser, SshPort, SshKey, VpnIP, SwarmNodeID, Status}, SwarmNodeRepository interface
   - WgmeshKey + tokens AES-encrypted (same pattern as DockerNode TLS creds)
   - Status types as typed consts
   - => created; SetTokens(), SetVpnIP(), SetSwarmNodeID() helpers

3. [x] Domain entities: SwarmNetwork, SwarmStack, SwarmRoute
   - `domain/swarm_network.go` — SwarmNetwork with ReservedIP{IP, Hostname, Label}, SwarmNetworkRepository
   - `domain/swarm_stack.go` — SwarmStack + SwarmRoute + SwarmStackRepository
   - => created; UpdateReservedIPs(), MarkRunning/Error/Updating() helpers

4. [x] Domain helpers
   - `domain/wgmesh.go` — `RenderWgmeshCompose(clusterKey, hostname string) string`
   - `domain/traefik_config.go` — `GenerateTraefikConfig(network, stacks, routes)` → traefik-routes.yml YAML
   - `domain/subnet.go` — `SplitSubnet(cidr)` → dhcpStart/End, reservedStart/End
   - => all created; build clean

5. [x] Migrations 003–007
   - `migrations/003_create_swarm_clusters.sql`
   - `migrations/004_create_swarm_nodes.sql`
   - `migrations/005_create_swarm_networks.sql`
   - `migrations/006_create_swarm_stacks.sql`
   - `migrations/007_create_swarm_routes.sql`
   - => all created with goose Up/Down

6. [x] Persistence: GORM repositories
   - `adapters/persistence/swarm_models.go` — all 5 GORM models with to/from domain funcs
   - `adapters/persistence/gorm_swarm_repositories.go` — 4 repositories; enc/dec WgmeshKey, tokens, SSH key
   - => created; JSON marshal for Options/ReservedIPs; build clean

7. [x] SSH transport adapter
   - `adapters/dockerclient/ssh_client.go`
   - `NewSSH(host, user, port, pemKey)` → custom http.Transport with SSH dialer via unix socket
   - `ExecSSH(...)` for provisioning commands (wgmesh0 IP polling)
   - => created; HostKeyCallback=InsecureIgnoreHostKey (TODO: known_hosts); build clean

### Phase 2 - Commands: Node provisioning + Swarm init/join - status: open

8. [ ] ProvisionSwarmNodeCommand + handler
   - `app/commands/swarm_node_commands.go`
   - SSH to node → run `docker compose up -d` with `RenderWgmeshCompose(...)` YAML
   - Poll `ip -4 addr show wgmesh0` via SSH exec (30 s timeout, 2 s interval)
   - Store `vpnIP` on SwarmNode, save, publish NATS event
   - HTTP handler streams progress via SSE: "deploying wgmesh…" → "waiting for wgmesh0…" → "VPN IP: X.X.X.X"

9. [ ] CreateSwarmCluster + InitSwarm command
   - `app/commands/swarm_cluster_commands.go`
   - **VVS-init path**: select manager SwarmNode (must have vpnIP) → `SwarmInit(ctx, vpnIP)` with all three addr flags → stores managerToken + workerToken encrypted
   - **Import path**: save cluster with pasted tokens + vpnIP; no Docker API call
   - SSE handler streams "initializing swarm…" → "cluster active, tokens stored"

10. [ ] AddSwarmNode command (join)
    - Worker: `SwarmJoin(ctx, managerVpnIP, workerToken)` after wgmesh provisioned
    - Manager: same with managerToken
    - Saves SwarmNodeID returned by Docker
    - SSE handler: provision wgmesh → get vpnIP → join → done

11. [ ] RemoveSwarmNode command
    - Demote if manager → `SwarmLeave(force=false)` on target → `SwarmNodeRemove(swarmNodeID)` on manager
    - Delete SwarmNode record; publish event

### Phase 3 - Commands: Networks + Stacks - status: open

12. [ ] Network commands
    - `app/commands/swarm_network_commands.go`
    - CreateSwarmNetwork: `NetworkCreate(req)` with `IPRange = dhcpRange CIDR` to enforce lower-half boundary
    - DeleteSwarmNetwork: `NetworkRemove(networkID)`
    - UpdateReservedIPs: update JSON column only (no Docker API call)

13. [ ] Stack commands
    - `app/commands/swarm_stack_commands.go`
    - DeploySwarmStack: `StackDeploy(ctx, name, composeYAML)` via manager SSH client; SSE-streamed (keep connection open during deploy)
    - UpdateSwarmStack: same (stack deploy is idempotent)
    - RemoveSwarmStack: `StackRemove(ctx, name)`

14. [ ] Queries
    - `app/queries/swarm_queries.go`
    - ListClusters, GetCluster, ListNodes (by clusterID), GetNode
    - ListNetworks (by clusterID), GetNetwork
    - ListStacks (by clusterID), GetStack, ListRoutes (by stackID)

### Phase 4 - HTTP layer: templates + handlers - status: open

15. [ ] Swarm templates: Clusters
    - `adapters/http/swarm_templates.templ`
    - SwarmClustersPage + SwarmClusterTable
    - SwarmClusterFormPage (create: name + wgmeshKey; import: +paste tokens)
    - SwarmClusterDetailPage (node table, network table, stack table)

16. [ ] Swarm templates: Nodes
    - SwarmNodesPage + SwarmNodeTable (columns: name, role badge, VPN IP, status, Docker node ID)
    - SwarmNodeFormPage (sshHost, sshUser, sshPort, sshKey textarea, clusterID, role)
    - Node ping/provision buttons; VPN IP shown once provisioned

17. [ ] Swarm templates: Networks
    - SwarmNetworksPage + SwarmNetworkTable
    - SwarmNetworkFormPage: subnet CIDR input → JS auto-splits into DHCP/reserved ranges (editable); driver select; gateway; parent (macvlan free-text)
    - SwarmNetworkDetailPage: reserved IPs inline editor (IP / hostname / label rows, add/remove)
    - "Download Traefik config" button → `GET /api/swarm/networks/{id}/traefik-config`

18. [ ] Swarm templates: Stacks
    - SwarmStacksPage + SwarmStackTable (columns: name, cluster, status, service count)
    - SwarmStackFormPage: CodeMirror YAML editor (reuse existing CDN includes) + Routes section (hostname:port rows)
    - SwarmStackDetailPage: status badge, service list, route table, action buttons

19. [ ] Swarm HTTP handlers
    - `adapters/http/swarm_handlers.go`
    - All CRUD + SSE: clusters (create/import/delete), nodes (create/provision/join/remove), networks (create/delete/update-reserved-ips), stacks (deploy/update/remove)
    - `GET /api/swarm/networks/{id}/traefik-config` → streams generated YAML as download
    - Follows same patterns as existing handlers.go (datastar.NewSSE, PatchElementTempl, Redirect)

### Phase 5 - Wiring + Nav - status: open

20. [ ] Wire swarm into docker module
    - `internal/app/wire_docker.go` — add swarm repos, commands, queries; pass to NewHandlers
    - `adapters/http/handlers.go` — add swarm handler registration (or new `RegisterSwarmRoutes`)
    - `builder.go` — migrations 003–007 already embed via dockermigrations.FS

21. [ ] Add Swarm nav group to sidebar
    - `internal/infrastructure/http/templates/layout.templ`
    - Collapsible "Swarm" group (same pattern as existing Services group)
    - Nav items: Clusters / Nodes / Networks / Stacks
    - Guard with `CanView(authdomain.ModuleDocker)`

### Phase 6 - Verification - status: open

22. [ ] Manual smoke test
    - Add swarm node → wgmesh deploys → vpnIP shown in UI
    - Create cluster (VVS-init path) → tokens masked
    - Add worker node → joins over WireGuard
    - Create overlay network 10.100.0.0/17 → DHCP range auto-calculated
    - Add reserved IP in panel → visible, not fetched from Docker
    - Deploy nginx stack → runs
    - Add route `nginx.local:80` → download Traefik config → valid YAML

## Verification

- `go build ./...` and `templ generate` pass after each phase
- SSH dial test: node ping returns Docker version over SSH transport
- wgmesh0 IP stored on node after provision (non-empty vpnIP field)
- Swarm init: cluster status = active, tokens non-empty (masked in UI)
- Worker join: node appears in `docker node ls` on manager
- Network DHCP range boundary: containers get IPs in lower half only
- Traefik config download contains correct hostRouter entries for each route

## Adjustments

## Progress Log

- 2026-04-20 13:15 — Action 1: x/crypto/ssh dependency added, build clean
- 2026-04-20 13:25 — Actions 2-7: Phase 1 complete — domain, migrations, persistence, SSH transport (commit 3638e0a)
