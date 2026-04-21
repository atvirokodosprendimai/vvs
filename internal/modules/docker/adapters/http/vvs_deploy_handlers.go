package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// VVSDeployHandlers manages VVS component deployment (portal/stb) and container registries.
type VVSDeployHandlers struct {
	createRegistry    *commands.CreateRegistryHandler
	updateRegistry    *commands.UpdateRegistryHandler
	deleteRegistry    *commands.DeleteRegistryHandler
	listRegistries    *queries.ListRegistriesHandler
	deployComponent   *commands.DeployVVSComponentHandler
	redeployComponent *commands.RedeployVVSComponentHandler
	deleteDeployment  *commands.DeleteVVSDeploymentHandler
	listDeployments   *queries.ListVVSDeploymentsHandler
	getDeployment     *queries.GetVVSDeploymentHandler
	listNodes         *queries.ListSwarmNodesHandler
	listClusters      *queries.ListSwarmClustersHandler
}

func NewVVSDeployHandlers(
	createRegistry *commands.CreateRegistryHandler,
	updateRegistry *commands.UpdateRegistryHandler,
	deleteRegistry *commands.DeleteRegistryHandler,
	listRegistries *queries.ListRegistriesHandler,
	deployComponent *commands.DeployVVSComponentHandler,
	redeployComponent *commands.RedeployVVSComponentHandler,
	deleteDeployment *commands.DeleteVVSDeploymentHandler,
	listDeployments *queries.ListVVSDeploymentsHandler,
	getDeployment *queries.GetVVSDeploymentHandler,
	listNodes *queries.ListSwarmNodesHandler,
	listClusters *queries.ListSwarmClustersHandler,
) *VVSDeployHandlers {
	return &VVSDeployHandlers{
		createRegistry:    createRegistry,
		updateRegistry:    updateRegistry,
		deleteRegistry:    deleteRegistry,
		listRegistries:    listRegistries,
		deployComponent:   deployComponent,
		redeployComponent: redeployComponent,
		deleteDeployment:  deleteDeployment,
		listDeployments:   listDeployments,
		getDeployment:     getDeployment,
		listNodes:         listNodes,
		listClusters:      listClusters,
	}
}

func (h *VVSDeployHandlers) RegisterRoutes(r chi.Router) {
	r.Get("/swarm/vvs", h.deploymentsPage)

	r.Get("/api/swarm/vvs/deployments", h.listDeploymentsSSE)
	r.Post("/api/swarm/vvs/deployments", h.createDeploymentSSE)
	r.Post("/api/swarm/vvs/deployments/{id}/redeploy", h.redeploySSE)
	r.Delete("/api/swarm/vvs/deployments/{id}", h.deleteDeploymentSSE)
	r.Get("/api/swarm/vvs/deployments/{id}/status", h.deploymentStatusSSE)

	r.Get("/api/swarm/vvs/registries", h.listRegistriesSSE)
	r.Post("/api/swarm/vvs/registries", h.createRegistrySSE)
	r.Delete("/api/swarm/vvs/registries/{id}", h.deleteRegistrySSE)
}

func (h *VVSDeployHandlers) ModuleName() string { return "docker" }

// ── Page ──────────────────────────────────────────────────────────────────────

func (h *VVSDeployHandlers) deploymentsPage(w http.ResponseWriter, r *http.Request) {
	nodes, _ := h.listNodes.Handle(r.Context(), "")
	registries, _ := h.listRegistries.Handle(r.Context())
	clusters, _ := h.listClusters.Handle(r.Context())
	VVSDeployPage(nodes, registries, clusters).Render(r.Context(), w)
}

// ── Deployments SSE ───────────────────────────────────────────────────────────

func (h *VVSDeployHandlers) listDeploymentsSSE(w http.ResponseWriter, r *http.Request) {
	deps, err := h.listDeployments.Handle(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElementTempl(VVSDeployTable(deps)); err != nil {
		slog.Error("vvs deploy table patch", "err", err)
	}
}

func (h *VVSDeployHandlers) createDeploymentSSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		Component  string `json:"vvsComponent"`
		Source     string `json:"vvsSource"`
		NodeID     string `json:"vvsNodeID"`
		ClusterID  string `json:"vvsClusterID"`
		ImageURL   string `json:"vvsImageURL"`
		RegistryID string `json:"vvsRegistryID"`
		GitURL     string `json:"vvsGitURL"`
		GitRef     string `json:"vvsGitRef"`
		NATSUrl    string `json:"vvsNATSUrl"`
		Port       int    `json:"vvsPort"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	dep, err := h.deployComponent.
		WithProgress(func(msg string) {
			_ = sse.PatchElementTempl(VVSDeployProgress(msg))
		}).
		Handle(r.Context(), commands.DeployVVSComponentCommand{
			ClusterID:  sig.ClusterID,
			NodeID:     sig.NodeID,
			Component:  domain.VVSComponentType(sig.Component),
			Source:     domain.VVSDeploySource(sig.Source),
			ImageURL:   sig.ImageURL,
			RegistryID: sig.RegistryID,
			GitURL:     sig.GitURL,
			GitRef:     sig.GitRef,
			NATSUrl:    sig.NATSUrl,
			Port:       sig.Port,
		})
	if err != nil {
		slog.Error("deploy vvs component", "err", err)
		_ = sse.PatchElementTempl(VVSDeployFormError(err.Error()))
		return
	}

	deps, _ := h.listDeployments.Handle(r.Context())
	_ = sse.PatchElementTempl(VVSDeployTable(deps))
	patchSignals(sse, map[string]any{
		"_vvsDeployOpen": false,
		"vvsComponent":   "",
		"vvsSource":      "image",
		"vvsNodeID":      "",
		"vvsClusterID":   "",
		"vvsImageURL":    "",
		"vvsRegistryID":  "",
		"vvsGitURL":      "",
		"vvsGitRef":      "main",
		"vvsNATSUrl":     "",
		"vvsPort":        0,
		"vvsDeployingID": dep.ID,
	})
}

func (h *VVSDeployHandlers) redeploySSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.redeployComponent.Handle(r.Context(), commands.RedeployVVSComponentCommand{DeploymentID: id}); err != nil {
		slog.Error("redeploy vvs component", "err", err)
		_ = sse.PatchElementTempl(VVSDeployFormError(err.Error()))
		return
	}
	deps, _ := h.listDeployments.Handle(r.Context())
	_ = sse.PatchElementTempl(VVSDeployTable(deps))
}

func (h *VVSDeployHandlers) deleteDeploymentSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteDeployment.Handle(r.Context(), commands.DeleteVVSDeploymentCommand{ID: id}); err != nil {
		slog.Error("delete vvs deployment", "err", err)
		_ = sse.PatchElementTempl(VVSDeployFormError(err.Error()))
		return
	}
	deps, _ := h.listDeployments.Handle(r.Context())
	_ = sse.PatchElementTempl(VVSDeployTable(deps))
}

func (h *VVSDeployHandlers) deploymentStatusSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	dep, err := h.getDeployment.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	sse := datastar.NewSSE(w, r)
	patchSignals(sse, map[string]any{
		"vvsDeployStatus_" + id: dep.Status,
		"vvsDeployError_" + id:  dep.ErrorMsg,
	})
}

// ── Registries SSE ────────────────────────────────────────────────────────────

func (h *VVSDeployHandlers) listRegistriesSSE(w http.ResponseWriter, r *http.Request) {
	regs, err := h.listRegistries.Handle(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElementTempl(VVSRegistryTable(regs)); err != nil {
		slog.Error("vvs registry table patch", "err", err)
	}
}

func (h *VVSDeployHandlers) createRegistrySSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		Name     string `json:"vvsRegName"`
		URL      string `json:"vvsRegURL"`
		Username string `json:"vvsRegUsername"`
		Password string `json:"vvsRegPassword"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	_, err := h.createRegistry.Handle(r.Context(), commands.CreateRegistryCommand{
		Name:     sig.Name,
		URL:      sig.URL,
		Username: sig.Username,
		Password: sig.Password,
	})
	if err != nil {
		slog.Error("create registry", "err", err)
		_ = sse.PatchElementTempl(VVSRegistryFormError(err.Error()))
		return
	}

	regs, _ := h.listRegistries.Handle(r.Context())
	_ = sse.PatchElementTempl(VVSRegistryTable(regs))
	patchSignals(sse, map[string]any{
		"_vvsRegOpen":    false,
		"vvsRegName":     "",
		"vvsRegURL":      "",
		"vvsRegUsername": "",
		"vvsRegPassword": "",
	})
}

func (h *VVSDeployHandlers) deleteRegistrySSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteRegistry.Handle(r.Context(), id); err != nil {
		slog.Error("delete registry", "err", err)
		_ = sse.PatchElementTempl(VVSRegistryFormError(err.Error()))
		return
	}
	regs, _ := h.listRegistries.Handle(r.Context())
	_ = sse.PatchElementTempl(VVSRegistryTable(regs))
}

// patchSignals marshals vals to JSON and calls sse.PatchSignals.
func patchSignals(sse *datastar.ServerSentEventGenerator, vals map[string]any) {
	b, _ := json.Marshal(vals)
	sse.PatchSignals(b)
}
