package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/hetzner"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// SwarmHandlers wires all Swarm management endpoints.
type SwarmHandlers struct {
	// Cluster commands
	createCluster        *commands.CreateSwarmClusterHandler
	importCluster        *commands.ImportSwarmClusterHandler
	initSwarm            *commands.InitSwarmHandler
	deleteCluster        *commands.DeleteSwarmClusterHandler
	updateHetznerConfig  *commands.UpdateClusterHetznerConfigHandler
	updateHetznerFilters *commands.UpdateHetznerFiltersHandler
	// Node commands
	createNode    *commands.CreateSwarmNodeHandler
	provisionNode *commands.ProvisionSwarmNodeHandler
	addNode       *commands.AddSwarmNodeHandler
	removeNode    *commands.RemoveSwarmNodeHandler
	orderHetzner  *commands.OrderHetznerNodeHandler
	// Network commands
	createNetwork    *commands.CreateSwarmNetworkHandler
	deleteNetwork    *commands.DeleteSwarmNetworkHandler
	updateReservedIP *commands.UpdateSwarmNetworkReservedIPsHandler
	// Stack commands
	deployStack  *commands.DeploySwarmStackHandler
	updateStack  *commands.UpdateSwarmStackHandler
	removeStack  *commands.RemoveSwarmStackHandler
	// Queries
	listClusters    *queries.ListSwarmClustersHandler
	getCluster      *queries.GetSwarmClusterHandler
	listNodes       *queries.ListSwarmNodesHandler
	getNode         *queries.GetSwarmNodeHandler
	listNetworks    *queries.ListSwarmNetworksHandler
	getNetwork      *queries.GetSwarmNetworkHandler
	listStacks      *queries.ListSwarmStacksHandler
	getStack        *queries.GetSwarmStackHandler
	listRegistries  *queries.ListRegistriesHandler
	// Repos
	networkRepo domain.SwarmNetworkRepository
	clusterRepo domain.SwarmClusterRepository
}

func NewSwarmHandlers(
	createCluster *commands.CreateSwarmClusterHandler,
	importCluster *commands.ImportSwarmClusterHandler,
	initSwarm *commands.InitSwarmHandler,
	deleteCluster *commands.DeleteSwarmClusterHandler,
	updateHetznerConfig *commands.UpdateClusterHetznerConfigHandler,
	updateHetznerFilters *commands.UpdateHetznerFiltersHandler,
	provisionNode *commands.ProvisionSwarmNodeHandler,
	addNode *commands.AddSwarmNodeHandler,
	removeNode *commands.RemoveSwarmNodeHandler,
	createNode *commands.CreateSwarmNodeHandler,
	orderHetzner *commands.OrderHetznerNodeHandler,
	createNetwork *commands.CreateSwarmNetworkHandler,
	deleteNetwork *commands.DeleteSwarmNetworkHandler,
	updateReservedIP *commands.UpdateSwarmNetworkReservedIPsHandler,
	deployStack *commands.DeploySwarmStackHandler,
	updateStack *commands.UpdateSwarmStackHandler,
	removeStack *commands.RemoveSwarmStackHandler,
	listClusters *queries.ListSwarmClustersHandler,
	getCluster *queries.GetSwarmClusterHandler,
	listNodes *queries.ListSwarmNodesHandler,
	getNode *queries.GetSwarmNodeHandler,
	listNetworks *queries.ListSwarmNetworksHandler,
	getNetwork *queries.GetSwarmNetworkHandler,
	listStacks *queries.ListSwarmStacksHandler,
	getStack *queries.GetSwarmStackHandler,
	listRegistries *queries.ListRegistriesHandler,
	networkRepo domain.SwarmNetworkRepository,
	clusterRepo domain.SwarmClusterRepository,
) *SwarmHandlers {
	return &SwarmHandlers{
		createCluster: createCluster, importCluster: importCluster,
		initSwarm: initSwarm, deleteCluster: deleteCluster,
		updateHetznerConfig:  updateHetznerConfig,
		updateHetznerFilters: updateHetznerFilters,
		createNode: createNode, provisionNode: provisionNode, addNode: addNode, removeNode: removeNode,
		orderHetzner: orderHetzner,
		createNetwork: createNetwork, deleteNetwork: deleteNetwork, updateReservedIP: updateReservedIP,
		deployStack: deployStack, updateStack: updateStack, removeStack: removeStack,
		listClusters: listClusters, getCluster: getCluster,
		listNodes: listNodes, getNode: getNode,
		listNetworks: listNetworks, getNetwork: getNetwork,
		listStacks: listStacks, getStack: getStack,
		listRegistries: listRegistries,
		networkRepo: networkRepo,
		clusterRepo: clusterRepo,
	}
}

func (h *SwarmHandlers) RegisterRoutes(r chi.Router) {
	// ── Cluster pages ──────────────────────────────────────────────────────────
	r.Get("/swarm/clusters", h.clusterListPage)
	r.Get("/swarm/clusters/new", h.clusterCreatePage)
	r.Get("/swarm/clusters/import", h.clusterImportPage)
	r.Get("/swarm/clusters/{id}", h.clusterDetailPage)

	// ── Cluster API ────────────────────────────────────────────────────────────
	r.Get("/api/swarm/clusters", h.clusterListSSE)
	r.Post("/api/swarm/clusters", h.clusterCreateSSE)
	r.Post("/api/swarm/clusters/import", h.clusterImportSSE)
	r.Post("/api/swarm/clusters/{id}/init", h.clusterInitSSE)
	r.Delete("/api/swarm/clusters/{id}", h.clusterDeleteSSE)
	r.Get("/swarm/clusters/{id}/hetzner", h.clusterHetznerConfigPage)
	r.Post("/api/swarm/clusters/{id}/hetzner", h.clusterHetznerConfigSSE)
	r.Post("/api/swarm/clusters/{id}/order-hetzner", h.clusterOrderHetznerSSE)
	r.Get("/api/swarm/clusters/{id}/hetzner-options", h.clusterHetznerOptionsSSE)
	r.Get("/api/swarm/clusters/{id}/hetzner-config-options", h.clusterHetznerConfigOptionsSSE)
	r.Post("/api/swarm/clusters/{id}/hetzner-filters", h.clusterHetznerFiltersSSE)

	// ── Node pages ─────────────────────────────────────────────────────────────
	r.Get("/swarm/nodes/new", h.nodeCreatePage)

	// ── Node API ───────────────────────────────────────────────────────────────
	r.Post("/api/swarm/nodes", h.nodeCreateSSE)
	r.Post("/api/swarm/nodes/{id}/provision", h.nodeProvisionSSE)
	r.Post("/api/swarm/nodes/{id}/join", h.nodeJoinSSE)
	r.Delete("/api/swarm/nodes/{id}", h.nodeRemoveSSE)

	// ── Network pages ──────────────────────────────────────────────────────────
	r.Get("/swarm/networks/new", h.networkCreatePage)
	r.Get("/swarm/networks/{id}", h.networkDetailPage)

	// ── Network API ────────────────────────────────────────────────────────────
	r.Post("/api/swarm/networks", h.networkCreateSSE)
	r.Delete("/api/swarm/networks/{id}", h.networkDeleteSSE)
	r.Post("/api/swarm/networks/{id}/reserved-ips", h.networkAddReservedIPSSE)
	r.Delete("/api/swarm/networks/{id}/reserved-ips/{index}", h.networkRemoveReservedIPSSE)
	r.Get("/api/swarm/networks/{id}/traefik-config", h.networkTraefikConfig)

	// ── Stack pages ─────────────────────────────────────────────────────────────
	r.Get("/swarm/stacks/new", h.stackCreatePage)
	r.Get("/swarm/stacks/{id}", h.stackDetailPage)
	r.Get("/swarm/stacks/{id}/edit", h.stackEditPage)

	// ── Stack API ───────────────────────────────────────────────────────────────
	r.Post("/api/swarm/stacks", h.stackDeploySSE)
	r.Put("/api/swarm/stacks/{id}", h.stackUpdateSSE)
	r.Delete("/api/swarm/stacks/{id}", h.stackRemoveSSE)
}

// ModuleName satisfies ModuleNamed.
func (h *SwarmHandlers) ModuleName() string { return "docker" }

// ── Cluster page handlers ─────────────────────────────────────────────────────

func (h *SwarmHandlers) clusterListPage(w http.ResponseWriter, r *http.Request) {
	SwarmClustersPage().Render(r.Context(), w)
}

func (h *SwarmHandlers) clusterCreatePage(w http.ResponseWriter, r *http.Request) {
	SwarmClusterFormPage().Render(r.Context(), w)
}

func (h *SwarmHandlers) clusterImportPage(w http.ResponseWriter, r *http.Request) {
	SwarmClusterImportPage().Render(r.Context(), w)
}

func (h *SwarmHandlers) clusterDetailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cluster, err := h.getCluster.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "cluster not found", http.StatusNotFound)
		return
	}
	nodes, _ := h.listNodes.Handle(r.Context(), id)
	networks, _ := h.listNetworks.Handle(r.Context(), id)
	stacks, _ := h.listStacks.Handle(r.Context(), id)
	SwarmClusterDetailPage(*cluster, nodes, networks, stacks).Render(r.Context(), w)
}

// ── Cluster SSE handlers ──────────────────────────────────────────────────────

func (h *SwarmHandlers) clusterListSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	clusters, _ := h.listClusters.Handle(r.Context())
	sse.PatchElementTempl(SwarmClusterTable(clusters))
}

func (h *SwarmHandlers) clusterCreateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var sig struct {
		Name      string `json:"name"`
		WgmeshKey string `json:"wgmeshkey"`
		Notes     string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	cluster, err := h.createCluster.Handle(r.Context(), commands.CreateSwarmClusterCommand{
		Name:      sig.Name,
		WgmeshKey: sig.WgmeshKey,
		Notes:     sig.Notes,
	})
	if err != nil {
		slog.Error("swarm: create cluster", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters/" + cluster.ID)
}

func (h *SwarmHandlers) clusterImportSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var sig struct {
		Name          string `json:"name"`
		WgmeshKey     string `json:"wgmeshkey"`
		ManagerToken  string `json:"managertoken"`
		WorkerToken   string `json:"workertoken"`
		AdvertiseAddr string `json:"advertiseaddr"`
		Notes         string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	cluster, err := h.importCluster.Handle(r.Context(), commands.ImportSwarmClusterCommand{
		Name:          sig.Name,
		WgmeshKey:     sig.WgmeshKey,
		ManagerToken:  sig.ManagerToken,
		WorkerToken:   sig.WorkerToken,
		AdvertiseAddr: sig.AdvertiseAddr,
		Notes:         sig.Notes,
	})
	if err != nil {
		slog.Error("swarm: import cluster", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters/" + cluster.ID)
}

func (h *SwarmHandlers) clusterInitSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	clusterID := chi.URLParam(r, "id")
	// Find the manager node with a VPN IP
	nodes, err := h.listNodes.Handle(r.Context(), clusterID)
	if err != nil {
		http.Error(w, "failed to list nodes", http.StatusInternalServerError)
		return
	}
	var managerNodeID string
	for _, n := range nodes {
		if n.Role == "manager" && n.VpnIP != "" {
			managerNodeID = n.ID
			break
		}
	}
	if managerNodeID == "" {
		http.Error(w, "no provisioned manager node found", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(SwarmProgressNotice("Initialising swarm…"))

	handler := h.initSwarm.WithProgress(func(msg string) {
		sse.PatchElementTempl(SwarmProgressNotice(msg))
	})
	_, err = handler.Handle(r.Context(), commands.InitSwarmCommand{
		ClusterID:     clusterID,
		ManagerNodeID: managerNodeID,
	})
	if err != nil {
		slog.Error("swarm: init", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters/" + clusterID)
}

// ── Hetzner config handlers ───────────────────────────────────────────────────

func (h *SwarmHandlers) clusterHetznerConfigPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cluster, err := h.getCluster.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "cluster not found", http.StatusNotFound)
		return
	}
	SwarmClusterHetznerConfigPage(*cluster).Render(r.Context(), w)
}

func (h *SwarmHandlers) clusterHetznerConfigSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var sig struct {
		APIKey      string `json:"hetzner_apikey"`
		SSHKeyID    string `json:"hetzner_sshkeyid"`
		SSHPrivKey  string `json:"hetzner_sshprivkey"`
		SSHPubKey   string `json:"hetzner_sshpubkey"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	keyID, _ := strconv.Atoi(sig.SSHKeyID)
	err := h.updateHetznerConfig.Handle(r.Context(), commands.UpdateClusterHetznerConfigCommand{
		ClusterID:     id,
		APIKey:        sig.APIKey,
		SSHKeyID:      keyID,
		SSHPrivateKey: []byte(sig.SSHPrivKey),
		SSHPublicKey:  sig.SSHPubKey,
	})
	if err != nil {
		slog.Error("swarm: update hetzner config", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters/" + id)
}

func (h *SwarmHandlers) clusterOrderHetznerSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	clusterID := chi.URLParam(r, "id")
	var sig struct {
		Name       string `json:"hetzner_name"`
		ServerType string `json:"hetzner_servertype"`
		Location   string `json:"hetzner_location"`
		Image      string `json:"hetzner_image"`
		Role       string `json:"hetzner_role"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(SwarmHetznerProgress("Starting order…"))

	role := commands.SwarmNodeRoleFromString(sig.Role)
	handler := h.orderHetzner.WithProgress(func(msg string) {
		sse.PatchElementTempl(SwarmHetznerProgress(msg))
	})
	node, err := handler.Handle(r.Context(), commands.OrderHetznerNodeCommand{
		ClusterID:  clusterID,
		Name:       sig.Name,
		ServerType: sig.ServerType,
		Location:   sig.Location,
		Image:      sig.Image,
		Role:       role,
	})
	if err != nil {
		slog.Error("swarm: order hetzner node", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters/" + node.ClusterID)
}

func (h *SwarmHandlers) clusterHetznerOptionsSSE(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	cluster, _ := h.clusterRepo.FindByID(r.Context(), clusterID)
	apiKey := ""
	var enabledLocs, enabledSTs []string
	if cluster != nil {
		apiKey = cluster.HetznerAPIKey
		enabledLocs = cluster.EnabledLocations
		enabledSTs = cluster.EnabledServerTypes
	}
	serverTypes, _ := hetzner.ListServerTypes(r.Context(), apiKey)
	locations, _ := hetzner.ListLocations(r.Context(), apiKey)

	// Filter to enabled subsets (empty = show all)
	if len(enabledLocs) > 0 {
		filtered := locations[:0]
		for _, l := range locations {
			if sliceContains(enabledLocs, l.Name) {
				filtered = append(filtered, l)
			}
		}
		locations = filtered
	}
	if len(enabledSTs) > 0 {
		filtered := serverTypes[:0]
		for _, s := range serverTypes {
			if sliceContains(enabledSTs, s.Name) {
				filtered = append(filtered, s)
			}
		}
		serverTypes = filtered
	}

	sse := datastar.NewSSE(w, r)
	_ = sse.PatchElementTempl(SwarmHetznerOptions(serverTypes, locations))
	_ = sse.PatchSignals([]byte(`{"_hetzner_opts_loaded":true}`))
}

// clusterHetznerConfigOptionsSSE fetches all available locations + server types from Hetzner
// and returns checkbox HTML + pre-set signals for the config page filter section.
func (h *SwarmHandlers) clusterHetznerConfigOptionsSSE(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	cluster, _ := h.clusterRepo.FindByID(r.Context(), clusterID)
	apiKey := ""
	var enabledLocs, enabledSTs []string
	if cluster != nil {
		apiKey = cluster.HetznerAPIKey
		enabledLocs = cluster.EnabledLocations
		enabledSTs = cluster.EnabledServerTypes
	}
	serverTypes, _ := hetzner.ListServerTypes(r.Context(), apiKey)
	locations, _ := hetzner.ListLocations(r.Context(), apiKey)

	sse := datastar.NewSSE(w, r)
	_ = sse.PatchElementTempl(SwarmHetznerConfigOptionsForCluster(clusterID, serverTypes, locations, enabledLocs, enabledSTs))

	// Patch signals: hl_{name} for locations, hs_{name} for server types
	// NOTE: no leading underscore — signals starting with _ are private and never sent to backend
	signals := make(map[string]bool, len(locations)+len(serverTypes)+1)
	signals["_hetzner_filter_loaded"] = true
	for _, l := range locations {
		signals["hl_"+l.Name] = len(enabledLocs) == 0 || sliceContains(enabledLocs, l.Name)
	}
	for _, s := range serverTypes {
		signals["hs_"+s.Name] = len(enabledSTs) == 0 || sliceContains(enabledSTs, s.Name)
	}
	sigJSON, _ := json.Marshal(signals)
	_ = sse.PatchSignals(sigJSON)
}

// clusterHetznerFiltersSSE reads the hl_* and hs_* signals and saves enabled filters.
func (h *SwarmHandlers) clusterHetznerFiltersSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	clusterID := chi.URLParam(r, "id")
	var allSigs map[string]any
	if err := datastar.ReadSignals(r, &allSigs); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var enabledLocs, enabledSTs []string
	for k, v := range allSigs {
		on, _ := v.(bool)
		if !on {
			continue
		}
		if strings.HasPrefix(k, "hl_") {
			enabledLocs = append(enabledLocs, strings.TrimPrefix(k, "hl_"))
		} else if strings.HasPrefix(k, "hs_") {
			enabledSTs = append(enabledSTs, strings.TrimPrefix(k, "hs_"))
		}
	}

	sse := datastar.NewSSE(w, r)
	if err := h.updateHetznerFilters.Handle(r.Context(), commands.UpdateHetznerFiltersCommand{
		ClusterID:          clusterID,
		EnabledLocations:   enabledLocs,
		EnabledServerTypes: enabledSTs,
	}); err != nil {
		slog.Error("swarm: update hetzner filters", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.PatchElementTempl(SwarmFilterSavedNotice())
}

// sliceContains reports whether s contains item.
func sliceContains(s []string, item string) bool {
	for _, v := range s {
		if v == item {
			return true
		}
	}
	return false
}

// hetznerAPIKeyFor loads the decrypted Hetzner API key for a cluster.
func (h *SwarmHandlers) hetznerAPIKeyFor(ctx context.Context, clusterID string) string {
	if h.clusterRepo == nil {
		return ""
	}
	c, err := h.clusterRepo.FindByID(ctx, clusterID)
	if err != nil || c == nil {
		return ""
	}
	return c.HetznerAPIKey
}

func (h *SwarmHandlers) clusterDeleteSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteCluster.Handle(r.Context(), commands.DeleteSwarmClusterCommand{ClusterID: id}); err != nil {
		slog.Error("swarm: delete cluster", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters")
}

// ── Node page handlers ────────────────────────────────────────────────────────

func (h *SwarmHandlers) nodeCreatePage(w http.ResponseWriter, r *http.Request) {
	clusterID := r.URL.Query().Get("cluster")
	clusters, _ := h.listClusters.Handle(r.Context())
	SwarmNodeFormPage(clusterID, clusters).Render(r.Context(), w)
}

// ── Node SSE handlers ─────────────────────────────────────────────────────────

func (h *SwarmHandlers) nodeCreateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var sig struct {
		Name      string `json:"name"`
		SshHost   string `json:"sshhost"`
		SshUser   string `json:"sshuser"`
		SshPort   string `json:"sshport"`
		SshKey    string `json:"sshkey"`
		Role      string `json:"role"`
		ClusterID string `json:"clusterid"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	port := 22
	if p, err := strconv.Atoi(sig.SshPort); err == nil && p > 0 {
		port = p
	}
	role := domain.SwarmNodeWorker
	if sig.Role == "manager" {
		role = domain.SwarmNodeManager
	}

	node, err := h.createNode.Handle(r.Context(), commands.CreateSwarmNodeCommand{
		ClusterID: sig.ClusterID,
		Name:      sig.Name,
		SshHost:   sig.SshHost,
		SshUser:   sig.SshUser,
		SshPort:   port,
		SshKey:    []byte(sig.SshKey),
		Role:      role,
	})
	if err != nil {
		slog.Error("swarm: create node", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	redirectTo := "/swarm/clusters"
	if node.ClusterID != "" {
		redirectTo = "/swarm/clusters/" + node.ClusterID
	}
	sse.Redirect(redirectTo)
}

func (h *SwarmHandlers) nodeProvisionSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(SwarmProgressNotice("Connecting…"))

	handler := h.provisionNode.WithProgress(func(msg string) {
		sse.PatchElementTempl(SwarmProgressNotice(msg))
	})
	node, err := handler.Handle(r.Context(), commands.ProvisionSwarmNodeCommand{NodeID: nodeID})
	if err != nil {
		slog.Error("swarm: provision node", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.PatchElementTempl(SwarmNodeRow(queries.SwarmNodeReadModel{
		ID:          node.ID,
		ClusterID:   node.ClusterID,
		Name:        node.Name,
		Role:        string(node.Role),
		Status:      string(node.Status),
		SshHost:     node.SshHost,
		VpnIP:       node.VpnIP,
		SwarmNodeID: node.SwarmNodeID,
	}))
}

func (h *SwarmHandlers) nodeJoinSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")

	node, err := h.getNode.Handle(r.Context(), nodeID)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(SwarmProgressNotice("Joining swarm…"))

	handler := h.addNode.WithProgress(func(msg string) {
		sse.PatchElementTempl(SwarmProgressNotice(msg))
	})
	updatedNode, err := handler.Handle(r.Context(), commands.AddSwarmNodeCommand{
		ClusterID: node.ClusterID,
		NodeID:    nodeID,
	})
	if err != nil {
		slog.Error("swarm: join node", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.PatchElementTempl(SwarmNodeRow(queries.SwarmNodeReadModel{
		ID:          updatedNode.ID,
		ClusterID:   updatedNode.ClusterID,
		Name:        updatedNode.Name,
		Role:        string(updatedNode.Role),
		Status:      string(updatedNode.Status),
		SshHost:     updatedNode.SshHost,
		VpnIP:       updatedNode.VpnIP,
		SwarmNodeID: updatedNode.SwarmNodeID,
	}))
}

func (h *SwarmHandlers) nodeRemoveSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	node, _ := h.getNode.Handle(r.Context(), nodeID)
	sse := datastar.NewSSE(w, r)
	if err := h.removeNode.Handle(r.Context(), commands.RemoveSwarmNodeCommand{NodeID: nodeID}); err != nil {
		slog.Error("swarm: remove node", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	if node != nil && node.ClusterID != "" {
		sse.Redirect("/swarm/clusters/" + node.ClusterID)
	} else {
		sse.Redirect("/swarm/clusters")
	}
}

// ── Network page handlers ─────────────────────────────────────────────────────

func (h *SwarmHandlers) networkCreatePage(w http.ResponseWriter, r *http.Request) {
	clusterID := r.URL.Query().Get("cluster")
	clusters, _ := h.listClusters.Handle(r.Context())
	SwarmNetworkFormPage(clusterID, clusters).Render(r.Context(), w)
}

func (h *SwarmHandlers) networkDetailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	network, err := h.getNetwork.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "network not found", http.StatusNotFound)
		return
	}
	SwarmNetworkDetailPage(*network).Render(r.Context(), w)
}

// ── Network SSE handlers ──────────────────────────────────────────────────────

func (h *SwarmHandlers) networkCreateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var sig struct {
		Name      string `json:"name"`
		Driver    string `json:"driver"`
		Subnet    string `json:"subnet"`
		Gateway   string `json:"gateway"`
		Parent    string `json:"parent"`
		Scope     string `json:"scope"`
		ClusterID string `json:"clusterid"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	driver := domain.SwarmNetworkOverlay
	switch sig.Driver {
	case "macvlan":
		driver = domain.SwarmNetworkMacvlan
	case "bridge":
		driver = domain.SwarmNetworkBridge
	}
	scope := domain.SwarmNetworkScopeSwarm
	if sig.Scope == "local" {
		scope = domain.SwarmNetworkScopeLocal
	}

	net, err := h.createNetwork.Handle(r.Context(), commands.CreateSwarmNetworkCommand{
		ClusterID: sig.ClusterID,
		Name:      sig.Name,
		Driver:    driver,
		Subnet:    sig.Subnet,
		Gateway:   sig.Gateway,
		Parent:    sig.Parent,
		Scope:     scope,
	})
	if err != nil {
		slog.Error("swarm: create network", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters/" + net.ClusterID)
}

func (h *SwarmHandlers) networkDeleteSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)

	net, err := h.getNetwork.Handle(r.Context(), id)
	if err != nil {
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	clusterID := net.ClusterID

	if err := h.deleteNetwork.Handle(r.Context(), commands.DeleteSwarmNetworkCommand{NetworkID: id}); err != nil {
		slog.Error("swarm: delete network", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters/" + clusterID)
}

func (h *SwarmHandlers) networkAddReservedIPSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var sig struct {
		NewIP       string `json:"newip"`
		NewHostname string `json:"newhostname"`
		NewLabel    string `json:"newlabel"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	net, err := h.getNetwork.Handle(r.Context(), id)
	if err != nil {
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	updated := append(net.ReservedIPs, domain.ReservedIP{
		IP:       sig.NewIP,
		Hostname: sig.NewHostname,
		Label:    sig.NewLabel,
	})
	updatedNet, err := h.updateReservedIP.Handle(r.Context(), commands.UpdateSwarmNetworkReservedIPsCommand{
		NetworkID:   id,
		ReservedIPs: updated,
	})
	if err != nil {
		slog.Error("swarm: add reserved ip", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	model := queries.SwarmNetworkReadModel{
		ID:             updatedNet.ID,
		ClusterID:      updatedNet.ClusterID,
		Name:           updatedNet.Name,
		Driver:         string(updatedNet.Driver),
		Subnet:         updatedNet.Subnet,
		Gateway:        updatedNet.Gateway,
		DhcpRangeStart: updatedNet.DhcpRangeStart,
		DhcpRangeEnd:   updatedNet.DhcpRangeEnd,
		ReservedIPs:    updatedNet.ReservedIPs,
	}
	sse.PatchElementTempl(SwarmReservedIPsEditor(model))
}

func (h *SwarmHandlers) networkRemoveReservedIPSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	indexStr := chi.URLParam(r, "index")
	index, err := strconv.Atoi(indexStr)
	sse := datastar.NewSSE(w, r)
	if err != nil {
		sse.PatchElementTempl(SwarmFormError("invalid index"))
		return
	}

	net, err := h.getNetwork.Handle(r.Context(), id)
	if err != nil {
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	if index < 0 || index >= len(net.ReservedIPs) {
		sse.PatchElementTempl(SwarmFormError("index out of range"))
		return
	}
	updated := append(net.ReservedIPs[:index], net.ReservedIPs[index+1:]...)
	updatedNet, err := h.updateReservedIP.Handle(r.Context(), commands.UpdateSwarmNetworkReservedIPsCommand{
		NetworkID:   id,
		ReservedIPs: updated,
	})
	if err != nil {
		slog.Error("swarm: remove reserved ip", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	model := queries.SwarmNetworkReadModel{
		ID:             updatedNet.ID,
		ClusterID:      updatedNet.ClusterID,
		Name:           updatedNet.Name,
		Driver:         string(updatedNet.Driver),
		Subnet:         updatedNet.Subnet,
		Gateway:        updatedNet.Gateway,
		DhcpRangeStart: updatedNet.DhcpRangeStart,
		DhcpRangeEnd:   updatedNet.DhcpRangeEnd,
		ReservedIPs:    updatedNet.ReservedIPs,
	}
	sse.PatchElementTempl(SwarmReservedIPsEditor(model))
}

func (h *SwarmHandlers) networkTraefikConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	network, err := h.getNetwork.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "network not found", http.StatusNotFound)
		return
	}
	stacks, _ := h.listStacks.Handle(r.Context(), network.ClusterID)

	// Convert to domain types for traefik config generator
	domainStacks := make([]*domain.SwarmStack, len(stacks))
	var domainRoutes []*domain.SwarmRoute
	for i, s := range stacks {
		domainStacks[i] = &domain.SwarmStack{ID: s.ID, Name: s.Name}
		for _, route := range s.Routes {
			domainRoutes = append(domainRoutes, &domain.SwarmRoute{
				StackID:     s.ID,
				Hostname:    route.Hostname,
				Port:        route.Port,
				StripPrefix: route.StripPrefix,
			})
		}
	}

	domainNetwork := &domain.SwarmNetwork{
		Name:   network.Name,
		Subnet: network.Subnet,
	}

	yaml := domain.GenerateTraefikConfig(domainNetwork, domainStacks, domainRoutes)
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="traefik-routes.yml"`)
	w.Write([]byte(yaml))
}

// ── Stack page handlers ───────────────────────────────────────────────────────

func (h *SwarmHandlers) stackCreatePage(w http.ResponseWriter, r *http.Request) {
	clusterID := r.URL.Query().Get("cluster")
	clusters, _ := h.listClusters.Handle(r.Context())
	nodes, _ := h.listNodes.Handle(r.Context(), clusterID)
	registries, _ := h.listRegistries.Handle(r.Context())
	SwarmStackFormPage(clusterID, nil, clusters, nodes, registries).Render(r.Context(), w)
}

func (h *SwarmHandlers) stackDetailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stack, err := h.getStack.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "stack not found", http.StatusNotFound)
		return
	}
	SwarmStackDetailPage(*stack).Render(r.Context(), w)
}

func (h *SwarmHandlers) stackEditPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stack, err := h.getStack.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "stack not found", http.StatusNotFound)
		return
	}
	clusters, _ := h.listClusters.Handle(r.Context())
	nodes, _ := h.listNodes.Handle(r.Context(), stack.ClusterID)
	registries, _ := h.listRegistries.Handle(r.Context())
	SwarmStackFormPage(stack.ClusterID, stack, clusters, nodes, registries).Render(r.Context(), w)
}

// ── Stack SSE handlers ────────────────────────────────────────────────────────

func (h *SwarmHandlers) stackDeploySSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var sig struct {
		ClusterID    string `json:"clusterid"`
		Name         string `json:"name"`
		ComposeYAML  string `json:"composeyaml"`
		TargetNodeID string `json:"targetnodeid"`
		RegistryID   string `json:"registryid"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(SwarmProgressNotice("Deploying stack…"))

	handler := h.deployStack.WithProgress(func(msg string) {
		sse.PatchElementTempl(SwarmProgressNotice(msg))
	})
	stack, err := handler.Handle(r.Context(), commands.DeploySwarmStackCommand{
		ClusterID:    sig.ClusterID,
		Name:         sig.Name,
		ComposeYAML:  sig.ComposeYAML,
		TargetNodeID: sig.TargetNodeID,
		RegistryID:   sig.RegistryID,
	})
	if err != nil {
		slog.Error("swarm: deploy stack", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	if stack.ErrorMsg != "" {
		sse.PatchElementTempl(SwarmFormError(stack.ErrorMsg))
		return
	}
	sse.Redirect("/swarm/stacks/" + stack.ID)
}

func (h *SwarmHandlers) stackUpdateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var sig struct {
		ComposeYAML string `json:"composeyaml"`
		RegistryID  string `json:"registryid"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(SwarmProgressNotice("Updating stack…"))

	handler := h.updateStack.WithProgress(func(msg string) {
		sse.PatchElementTempl(SwarmProgressNotice(msg))
	})
	stack, err := handler.Handle(r.Context(), commands.UpdateSwarmStackCommand{
		StackID:     id,
		ComposeYAML: sig.ComposeYAML,
		RegistryID:  sig.RegistryID,
	})
	if err != nil {
		slog.Error("swarm: update stack", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	if stack.ErrorMsg != "" {
		sse.PatchElementTempl(SwarmFormError(stack.ErrorMsg))
		return
	}
	sse.Redirect("/swarm/stacks/" + id)
}

func (h *SwarmHandlers) stackRemoveSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)

	stack, err := h.getStack.Handle(r.Context(), id)
	if err != nil {
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	clusterID := stack.ClusterID

	if err := h.removeStack.Handle(r.Context(), commands.RemoveSwarmStackCommand{StackID: id}); err != nil {
		slog.Error("swarm: remove stack", "err", err)
		sse.PatchElementTempl(SwarmFormError(err.Error()))
		return
	}
	sse.Redirect("/swarm/clusters/" + clusterID)
}

