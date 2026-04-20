package http

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	authhttp "github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/http"
	authdomain "github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/dockerclient"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// Handlers wires all Docker Orchestrator HTTP endpoints.
type Handlers struct {
	// Node commands
	createNode *commands.CreateNodeHandler
	updateNode *commands.UpdateNodeHandler
	deleteNode *commands.DeleteNodeHandler
	// Service commands
	deployService *commands.DeployServiceHandler
	updateService *commands.UpdateServiceHandler
	stopService   *commands.StopServiceHandler
	startService  *commands.StartServiceHandler
	removeService *commands.RemoveServiceHandler
	// Queries
	listNodes    *queries.ListNodesHandler
	getNode      *queries.GetNodeHandler
	listServices *queries.ListServicesHandler
	getService   *queries.GetServiceHandler
	// Infra
	subscriber    events.EventSubscriber
	nodeRepo      domain.DockerNodeRepository
	serviceRepo   domain.DockerServiceRepository
	clientFactory domain.DockerClientFactory
}

func NewHandlers(
	createNode *commands.CreateNodeHandler,
	updateNode *commands.UpdateNodeHandler,
	deleteNode *commands.DeleteNodeHandler,
	deployService *commands.DeployServiceHandler,
	updateService *commands.UpdateServiceHandler,
	stopService *commands.StopServiceHandler,
	startService *commands.StartServiceHandler,
	removeService *commands.RemoveServiceHandler,
	listNodes *queries.ListNodesHandler,
	getNode *queries.GetNodeHandler,
	listServices *queries.ListServicesHandler,
	getService *queries.GetServiceHandler,
	subscriber events.EventSubscriber,
	nodeRepo domain.DockerNodeRepository,
	serviceRepo domain.DockerServiceRepository,
	clientFactory domain.DockerClientFactory,
) *Handlers {
	return &Handlers{
		createNode: createNode, updateNode: updateNode, deleteNode: deleteNode,
		deployService: deployService, updateService: updateService,
		stopService: stopService, startService: startService, removeService: removeService,
		listNodes: listNodes, getNode: getNode,
		listServices: listServices, getService: getService,
		subscriber:    subscriber,
		nodeRepo:      nodeRepo,
		serviceRepo:   serviceRepo,
		clientFactory: clientFactory,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// ── Node pages ─────────────────────────────────────────────────────────────
	r.Get("/docker/nodes", h.nodeListPage)
	r.Get("/docker/nodes/new", h.nodeCreatePage)
	r.Get("/docker/nodes/{id}/edit", h.nodeEditPage)

	// ── Node SSE / API ─────────────────────────────────────────────────────────
	r.Get("/api/docker/nodes", h.nodeListSSE)
	r.Post("/api/docker/nodes", h.nodeCreateSSE)
	r.Put("/api/docker/nodes/{id}", h.nodeUpdateSSE)
	r.Delete("/api/docker/nodes/{id}", h.nodeDeleteSSE)
	r.Post("/api/docker/nodes/{id}/ping", h.nodePingSSE)

	// ── Service pages ──────────────────────────────────────────────────────────
	r.Get("/docker/services", h.serviceListPage)
	r.Get("/docker/services/new", h.serviceCreatePage)
	r.Get("/docker/services/{id}", h.serviceDetailPage)
	r.Get("/docker/services/{id}/edit", h.serviceEditPage)
	r.Get("/docker/services/{id}/logs", h.serviceLogsPage)

	// ── Service SSE / API ──────────────────────────────────────────────────────
	r.Get("/api/docker/services", h.serviceListSSE)
	r.Post("/api/docker/services", h.serviceDeploySSE)
	r.Put("/api/docker/services/{id}", h.serviceUpdateSSE)
	r.Get("/api/docker/services/{id}/containers", h.serviceContainersSSE)
	r.Get("/api/docker/services/{id}/logs", h.serviceLogsSSE)
	r.Post("/api/docker/services/{id}/stop", h.serviceStopSSE)
	r.Post("/api/docker/services/{id}/start", h.serviceStartSSE)
	r.Delete("/api/docker/services/{id}", h.serviceRemoveSSE)
}

// ModuleName satisfies the ModuleNamed interface for permission checks.
func (h *Handlers) ModuleName() string { return "docker" }

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	user := authhttp.UserFromContext(r.Context())
	if user == nil || user.Role != authdomain.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// ── Node page handlers ────────────────────────────────────────────────────────

func (h *Handlers) nodeListPage(w http.ResponseWriter, r *http.Request) {
	DockerNodesPage().Render(r.Context(), w)
}

func (h *Handlers) nodeCreatePage(w http.ResponseWriter, r *http.Request) {
	DockerNodeFormPage(nil).Render(r.Context(), w)
}

func (h *Handlers) nodeEditPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	node, err := h.getNode.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	DockerNodeFormPage(node).Render(r.Context(), w)
}

// ── Node SSE handlers ─────────────────────────────────────────────────────────

func (h *Handlers) nodeListSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(events.DockerNodeAll.String())
	defer cancel()

	current, _ := h.listNodes.Handle(r.Context())
	sse.PatchElementTempl(DockerNodeTable(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listNodes.Handle(r.Context())
			if err != nil {
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(DockerNodeTable(next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) nodeCreateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var sig struct {
		Name    string `json:"name"`
		Host    string `json:"host"`
		IsLocal bool   `json:"isLocal"`
		TLSCert string `json:"tlsCert"`
		TLSKey  string `json:"tlsKey"`
		TLSCA   string `json:"tlsCa"`
		Notes   string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	_, err := h.createNode.Handle(r.Context(), commands.CreateNodeCommand{
		Name:    sig.Name,
		Host:    sig.Host,
		IsLocal: sig.IsLocal,
		TLSCert: []byte(sig.TLSCert),
		TLSKey:  []byte(sig.TLSKey),
		TLSCA:   []byte(sig.TLSCA),
		Notes:   sig.Notes,
	})
	if err != nil {
		slog.Error("docker: create node", "err", err)
		sse.PatchElementTempl(DockerNodeFormError(err.Error()))
		return
	}
	sse.Redirect("/docker/nodes")
}

func (h *Handlers) nodeUpdateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var sig struct {
		Name    string `json:"name"`
		Host    string `json:"host"`
		IsLocal bool   `json:"isLocal"`
		TLSCert string `json:"tlsCert"`
		TLSKey  string `json:"tlsKey"`
		TLSCA   string `json:"tlsCa"`
		Notes   string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	_, err := h.updateNode.Handle(r.Context(), commands.UpdateNodeCommand{
		ID:      id,
		Name:    sig.Name,
		Host:    sig.Host,
		IsLocal: sig.IsLocal,
		TLSCert: []byte(sig.TLSCert),
		TLSKey:  []byte(sig.TLSKey),
		TLSCA:   []byte(sig.TLSCA),
		Notes:   sig.Notes,
	})
	if err != nil {
		slog.Error("docker: update node", "err", err)
		sse.PatchElementTempl(DockerNodeFormError(err.Error()))
		return
	}
	sse.Redirect("/docker/nodes")
}

func (h *Handlers) nodeDeleteSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteNode.Handle(r.Context(), commands.DeleteNodeCommand{ID: id}); err != nil {
		slog.Error("docker: delete node", "err", err)
		sse.PatchElementTempl(DockerInlineError("docker-node-error", err.Error()))
	}
}

func (h *Handlers) nodePingSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)

	node, err := h.nodeRepo.FindByID(r.Context(), id)
	if err != nil {
		sse.PatchElementTempl(DockerPingResult(id, false, "node not found"))
		return
	}
	client, err := h.clientFactory.ForNode(node)
	if err != nil {
		sse.PatchElementTempl(DockerPingResult(id, false, err.Error()))
		return
	}
	if err := client.Ping(r.Context()); err != nil {
		sse.PatchElementTempl(DockerPingResult(id, false, err.Error()))
		return
	}
	sse.PatchElementTempl(DockerPingResult(id, true, ""))
}

// ── Service page handlers ─────────────────────────────────────────────────────

func (h *Handlers) serviceListPage(w http.ResponseWriter, r *http.Request) {
	DockerServicesPage().Render(r.Context(), w)
}

func (h *Handlers) serviceCreatePage(w http.ResponseWriter, r *http.Request) {
	nodes, _ := h.listNodes.Handle(r.Context())
	DockerServiceFormPage(nodes).Render(r.Context(), w)
}

func (h *Handlers) serviceDetailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	svc, err := h.getService.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	DockerServiceDetailPage(svc).Render(r.Context(), w)
}

func (h *Handlers) serviceEditPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	svc, err := h.getService.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	DockerServiceEditPage(svc).Render(r.Context(), w)
}

func (h *Handlers) serviceLogsPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	svc, err := h.getService.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	DockerLogPage(svc).Render(r.Context(), w)
}

// ── Service SSE handlers ──────────────────────────────────────────────────────

func (h *Handlers) serviceListSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(events.DockerServiceAll.String())
	defer cancel()

	current, _ := h.listServices.Handle(r.Context())
	sse.PatchElementTempl(DockerServiceTable(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listServices.Handle(r.Context())
			if err != nil {
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(DockerServiceTable(next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) serviceDeploySSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var sig struct {
		NodeID      string `json:"nodeId"`
		Name        string `json:"name"`
		ComposeYAML string `json:"composeYaml"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	// Stream "deploying" notice — connection stays open while Docker runs
	sse.PatchElementTempl(DockerServiceDeployingNotice("Deploying… this may take a minute"))
	svc, err := h.deployService.Handle(r.Context(), commands.DeployServiceCommand{
		NodeID:      sig.NodeID,
		Name:        sig.Name,
		ComposeYAML: sig.ComposeYAML,
	})
	if err != nil {
		slog.Error("docker: deploy service", "err", err)
		sse.PatchElementTempl(DockerServiceFormError(err.Error()))
		return
	}
	if svc.ErrorMsg != "" {
		sse.PatchElementTempl(DockerServiceFormError(svc.ErrorMsg))
		return
	}
	sse.Redirect("/docker/services/" + svc.ID)
}

func (h *Handlers) serviceUpdateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var sig struct {
		ComposeYAML string `json:"composeYaml"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	// Stream "updating" notice — connection stays open while Docker re-deploys
	sse.PatchElementTempl(DockerServiceDeployingNotice("Updating… redeploying containers"))
	if err := h.updateService.Handle(r.Context(), commands.UpdateServiceCommand{
		ID:          id,
		ComposeYAML: sig.ComposeYAML,
	}); err != nil {
		slog.Error("docker: update service", "err", err)
		sse.PatchElementTempl(DockerServiceFormError(err.Error()))
		return
	}
	sse.Redirect("/docker/services/" + id)
}

func (h *Handlers) serviceContainersSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.DockerServiceStatusChanged.String())
	defer cancel()

	renderContainers := func() {
		svc, err := h.serviceRepo.FindByID(r.Context(), id)
		if err != nil {
			return
		}
		node, err := h.nodeRepo.FindByID(r.Context(), svc.NodeID)
		if err != nil {
			return
		}
		client, err := h.clientFactory.ForNode(node)
		if err != nil {
			return
		}
		containers, err := client.ListContainers(r.Context(), svc.Name)
		if err != nil {
			return
		}
		sse.PatchElementTempl(DockerContainerTable(id, containers))
	}

	renderContainers()

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			renderContainers()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) serviceStopSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.stopService.Handle(r.Context(), commands.StopServiceCommand{ID: id}); err != nil {
		slog.Error("docker: stop service", "err", err)
		sse.PatchElementTempl(DockerInlineError("docker-service-error-"+id, err.Error()))
		return
	}
	svc, err := h.getService.Handle(r.Context(), id)
	if err != nil {
		return
	}
	sse.PatchElementTempl(DockerServiceStatusBadge(svc))
	sse.PatchElementTempl(DockerServiceActions(svc))
}

func (h *Handlers) serviceStartSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.startService.Handle(r.Context(), commands.StartServiceCommand{ID: id}); err != nil {
		slog.Error("docker: start service", "err", err)
		sse.PatchElementTempl(DockerInlineError("docker-service-error-"+id, err.Error()))
		return
	}
	svc, err := h.getService.Handle(r.Context(), id)
	if err != nil {
		return
	}
	sse.PatchElementTempl(DockerServiceStatusBadge(svc))
	sse.PatchElementTempl(DockerServiceActions(svc))
}

func (h *Handlers) serviceRemoveSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.removeService.Handle(r.Context(), commands.RemoveServiceCommand{ID: id}); err != nil {
		slog.Error("docker: remove service", "err", err)
		sse.PatchElementTempl(DockerInlineError("docker-service-error-"+id, err.Error()))
		return
	}
	sse.Redirect("/docker/services")
}

// serviceLogsSSE streams live logs for all containers in a service directly from Docker.
// Each container is streamed in its own goroutine; lines are fan-in to SSE via a shared channel.
func (h *Handlers) serviceLogsSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	svc, err := h.serviceRepo.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	node, err := h.nodeRepo.FindByID(r.Context(), svc.NodeID)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	client, err := h.clientFactory.ForNode(node)
	if err != nil {
		http.Error(w, "cannot connect to docker node", http.StatusServiceUnavailable)
		return
	}
	containers, err := client.ListContainers(r.Context(), svc.Name)
	if err != nil || len(containers) == 0 {
		http.Error(w, "no containers found", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Fan-in: one line channel shared by all container goroutines
	logCh := make(chan string, 128)

	for _, c := range containers {
		containerID := c.ID
		name := c.Name
		go func() {
			rc, err := client.StreamLogs(ctx, containerID, true)
			if err != nil {
				return
			}
			defer rc.Close()
			br := bufio.NewReader(rc)
			for {
				line, err := dockerclient.ReadMultiplexLine(br)
				if err != nil {
					return
				}
				if line == "" {
					continue
				}
				select {
				case logCh <- fmt.Sprintf("[%s] %s", name, line):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	sse := datastar.NewSSE(w, r)
	for {
		select {
		case line := <-logCh:
			sse.PatchElementTempl(DockerLogLine(line))
		case <-ctx.Done():
			return
		}
	}
}
