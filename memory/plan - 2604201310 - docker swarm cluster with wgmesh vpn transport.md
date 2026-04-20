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

### Phase 2 - Commands: Node provisioning + Swarm init/join - status: completed

8. [x] ProvisionSwarmNodeCommand + handler
   - `app/commands/swarm_node_commands.go`
   - => SSH → deploy wgmesh compose → poll `ip -4 addr show wgmesh0` (30s/2s) → store vpnIP
   - => WithProgress(fn) shallow-copy pattern for SSE streaming

9. [x] CreateSwarmCluster + InitSwarm command
   - => CreateSwarmClusterHandler + InitSwarmHandler + ImportSwarmClusterHandler
   - => VVS-init: SwarmInit(vpnIP) → SetTokens; Import: paste tokens directly

10. [x] AddSwarmNode command (join)
    - => AddSwarmNodeHandler: joins via workerToken/managerToken; fetches SwarmNodeID from manager node list

11. [x] RemoveSwarmNode command
    - => RemoveSwarmNodeHandler: SwarmLeave + SwarmNodeRemove on manager + delete record

### Phase 3 - Commands: Networks + Stacks - status: completed

12. [x] Network commands
    - => swarm_network_commands.go: CreateSwarmNetwork with DHCPRangeCIDR() IPRange enforcement
    - => DeleteSwarmNetwork, UpdateSwarmNetworkReservedIPs (metadata only)

13. [x] Stack commands
    - => swarm_stack_commands.go: DeploySwarmStack (SSE-streamed, error on entity), UpdateSwarmStack, RemoveSwarmStack

14. [x] Queries
    - => swarm_queries.go: all read models + handlers for clusters/nodes/networks/stacks/routes

### Phase 4 - HTTP layer: templates + handlers - status: completed

15. [x] Swarm templates: Clusters
    - => SwarmClustersPage, SwarmClusterTable, SwarmClusterFormPage, SwarmClusterImportPage, SwarmClusterDetailPage

16. [x] Swarm templates: Nodes
    - => SwarmNodeTable, SwarmNodeRow (provision/join buttons), SwarmNodeFormPage

17. [x] Swarm templates: Networks
    - => SwarmNetworkTable, SwarmNetworkFormPage, SwarmNetworkDetailPage, SwarmReservedIPsEditor
    - => DHCPRangeCIDR helper; Traefik config download link

18. [x] Swarm templates: Stacks
    - => SwarmStackTable, SwarmStackFormPage, SwarmStackDetailPage with routes table

19. [x] Swarm HTTP handlers
    - => swarm_handlers.go: SwarmHandlers with all CRUD + SSE + traefik-config download
    - => nodeCreateSSE uses CreateSwarmNodeHandler; provision/join/remove stream via WithProgress

### Phase 5 - Wiring + Nav - status: completed

20. [x] Wire swarm into docker module
    - => wire_docker.go: dockerWired.swarmRoutes; all swarm repos/commands/queries wired
    - => builder.go: collectRoutes adds docker.swarmRoutes

21. [x] Add Swarm nav group to sidebar
    - => layout.templ: "Swarm" collapsible group under Docker Services; _navSwarm signal; Clusters item
    - => swarmNavIcon() added

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
- 2026-04-20 — Actions 8-21: Phases 2-5 complete — commands, queries, templates, handlers, wiring, nav (commit a15af55)
