package http

import (
	"log/slog"
	"net/http"
	"reflect"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	authdomain "github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	authhttp "github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/http"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// Handlers wires all Proxmox HTTP endpoints.
type Handlers struct {
	// Node commands
	createNode *commands.CreateNodeHandler
	updateNode *commands.UpdateNodeHandler
	deleteNode *commands.DeleteNodeHandler
	// VM commands
	createVM        *commands.CreateVMHandler
	suspendVM       *commands.SuspendVMHandler
	resumeVM        *commands.ResumeVMHandler
	restartVM       *commands.RestartVMHandler
	deleteVM        *commands.DeleteVMHandler
	assignVMCustomer *commands.AssignVMCustomerHandler
	// Queries
	listNodes          *queries.ListNodesHandler
	getNode            *queries.GetNodeHandler
	listVMs            *queries.ListVMsHandler
	getVM              *queries.GetVMHandler
	listVMsForCustomer *queries.ListVMsForCustomerHandler
	// Infra
	subscriber events.EventSubscriber
}

func NewHandlers(
	createNode *commands.CreateNodeHandler,
	updateNode *commands.UpdateNodeHandler,
	deleteNode *commands.DeleteNodeHandler,
	createVM *commands.CreateVMHandler,
	suspendVM *commands.SuspendVMHandler,
	resumeVM *commands.ResumeVMHandler,
	restartVM *commands.RestartVMHandler,
	deleteVM *commands.DeleteVMHandler,
	assignVMCustomer *commands.AssignVMCustomerHandler,
	listNodes *queries.ListNodesHandler,
	getNode *queries.GetNodeHandler,
	listVMs *queries.ListVMsHandler,
	getVM *queries.GetVMHandler,
	listVMsForCustomer *queries.ListVMsForCustomerHandler,
	subscriber events.EventSubscriber,
) *Handlers {
	return &Handlers{
		createNode: createNode, updateNode: updateNode, deleteNode: deleteNode,
		createVM: createVM, suspendVM: suspendVM, resumeVM: resumeVM,
		restartVM: restartVM, deleteVM: deleteVM, assignVMCustomer: assignVMCustomer,
		listNodes: listNodes, getNode: getNode,
		listVMs: listVMs, getVM: getVM, listVMsForCustomer: listVMsForCustomer,
		subscriber: subscriber,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// ── Node pages ──────────────────────────────────────────────────────────────
	r.Get("/proxmox/nodes", h.nodeListPage)
	r.Get("/proxmox/nodes/new", h.nodeCreatePage)
	r.Get("/proxmox/nodes/{id}", h.nodeDetailPage)
	r.Get("/proxmox/nodes/{id}/edit", h.nodeEditPage)

	// ── Node SSE / API ─────────────────────────────────────────────────────────
	r.Get("/api/proxmox/nodes", h.nodeListSSE)
	r.Post("/api/proxmox/nodes", h.nodeCreateSSE)
	r.Put("/api/proxmox/nodes/{id}", h.nodeUpdateSSE)
	r.Delete("/api/proxmox/nodes/{id}", h.nodeDeleteSSE)

	// ── VM pages ────────────────────────────────────────────────────────────────
	r.Get("/proxmox/vms", h.vmListPage)
	r.Get("/proxmox/vms/new", h.vmCreatePage)
	r.Get("/proxmox/vms/{id}", h.vmDetailPage)

	// ── VM SSE / API ───────────────────────────────────────────────────────────
	r.Get("/api/proxmox/vms", h.vmListSSE)
	r.Post("/api/proxmox/vms", h.vmCreateSSE)
	r.Post("/api/proxmox/vms/{id}/suspend", h.vmSuspendSSE)
	r.Post("/api/proxmox/vms/{id}/resume", h.vmResumeSSE)
	r.Post("/api/proxmox/vms/{id}/restart", h.vmRestartSSE)
	r.Delete("/api/proxmox/vms/{id}", h.vmDeleteSSE)
	r.Put("/api/proxmox/vms/{id}/customer", h.vmAssignCustomerSSE)
}

// ModuleName satisfies the ModuleNamed interface for permission checks.
func (h *Handlers) ModuleName() string { return "proxmox" }

// requireAdmin enforces admin-only access for write operations.
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
	ProxmoxNodesPage().Render(r.Context(), w)
}

func (h *Handlers) nodeCreatePage(w http.ResponseWriter, r *http.Request) {
	NodeFormPage(nil).Render(r.Context(), w)
}

func (h *Handlers) nodeDetailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	node, err := h.getNode.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	NodeDetailPage(node).Render(r.Context(), w)
}

func (h *Handlers) nodeEditPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	node, err := h.getNode.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}
	NodeFormPage(node).Render(r.Context(), w)
}

// ── Node SSE handlers ─────────────────────────────────────────────────────────

func (h *Handlers) nodeListSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(events.ProxmoxNodeAll.String())
	defer cancel()

	current, _ := h.listNodes.Handle(r.Context())
	sse.PatchElementTempl(NodeTable(current))

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
				sse.PatchElementTempl(NodeTable(next))
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
		Name        string `json:"name"`
		NodeName    string `json:"nodeName"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		User        string `json:"user"`
		TokenID     string `json:"tokenId"`
		TokenSecret string `json:"tokenSecret"`
		InsecureTLS bool   `json:"insecureTls"`
		Notes       string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	_, err := h.createNode.Handle(r.Context(), commands.CreateNodeCommand{
		Name: sig.Name, NodeName: sig.NodeName, Host: sig.Host, Port: sig.Port,
		User: sig.User, TokenID: sig.TokenID, TokenSecret: sig.TokenSecret,
		InsecureTLS: sig.InsecureTLS, Notes: sig.Notes,
	})
	if err != nil {
		slog.Error("proxmox: create node", "err", err)
		sse.PatchElementTempl(NodeFormError(err.Error()))
		return
	}
	sse.Redirect("/proxmox/nodes")
}

func (h *Handlers) nodeUpdateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var sig struct {
		Name        string `json:"name"`
		NodeName    string `json:"nodeName"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		User        string `json:"user"`
		TokenID     string `json:"tokenId"`
		TokenSecret string `json:"tokenSecret"`
		InsecureTLS bool   `json:"insecureTls"`
		Notes       string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	_, err := h.updateNode.Handle(r.Context(), commands.UpdateNodeCommand{
		ID: id, Name: sig.Name, NodeName: sig.NodeName, Host: sig.Host, Port: sig.Port,
		User: sig.User, TokenID: sig.TokenID, TokenSecret: sig.TokenSecret,
		InsecureTLS: sig.InsecureTLS, Notes: sig.Notes,
	})
	if err != nil {
		slog.Error("proxmox: update node", "err", err)
		sse.PatchElementTempl(NodeFormError(err.Error()))
		return
	}
	sse.Redirect("/proxmox/nodes")
}

func (h *Handlers) nodeDeleteSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteNode.Handle(r.Context(), commands.DeleteNodeCommand{ID: id}); err != nil {
		slog.Error("proxmox: delete node", "err", err)
		sse.PatchElementTempl(NodeListError(err.Error()))
		return
	}
	sse.Redirect("/proxmox/nodes")
}

// ── VM page handlers ──────────────────────────────────────────────────────────

func (h *Handlers) vmListPage(w http.ResponseWriter, r *http.Request) {
	ProxmoxVMsPage().Render(r.Context(), w)
}

func (h *Handlers) vmCreatePage(w http.ResponseWriter, r *http.Request) {
	nodes, _ := h.listNodes.Handle(r.Context())
	VMCreateFormPage(nodes).Render(r.Context(), w)
}

func (h *Handlers) vmDetailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vm, err := h.getVM.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "VM not found", http.StatusNotFound)
		return
	}
	VMDetailPage(vm).Render(r.Context(), w)
}

// ── VM SSE handlers ───────────────────────────────────────────────────────────

func (h *Handlers) vmListSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(events.ProxmoxVMAll.String())
	defer cancel()

	current, _ := h.listVMs.Handle(r.Context())
	sse.PatchElementTempl(VMListTable(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listVMs.Handle(r.Context())
			if err != nil {
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(VMListTable(next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *Handlers) vmCreateSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var sig struct {
		NodeID       string `json:"nodeId"`
		CustomerID   string `json:"customerId"`
		Name         string `json:"name"`
		TemplateVMID int    `json:"templateVmid"`
		Storage      string `json:"storage"`
		Cores        int    `json:"cores"`
		MemoryMB     int    `json:"memoryMb"`
		DiskGB       int    `json:"diskGb"`
		FullClone    bool   `json:"fullClone"`
		Notes        string `json:"notes"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	_, err := h.createVM.Handle(r.Context(), commands.CreateVMCommand{
		NodeID: sig.NodeID, CustomerID: sig.CustomerID, Name: sig.Name,
		TemplateVMID: sig.TemplateVMID, Storage: sig.Storage,
		Cores: sig.Cores, MemoryMB: sig.MemoryMB, DiskGB: sig.DiskGB,
		FullClone: sig.FullClone, Notes: sig.Notes,
	})
	if err != nil {
		slog.Error("proxmox: create VM", "err", err)
		sse.PatchElementTempl(VMFormError(err.Error()))
		return
	}
	sse.Redirect("/proxmox/vms")
}

func (h *Handlers) vmSuspendSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.suspendVM.Handle(r.Context(), commands.SuspendVMCommand{ID: id}); err != nil {
		slog.Error("proxmox: suspend VM", "err", err)
		sse.PatchElementTempl(VMActionError(id, err.Error()))
		return
	}
	// Status update arrives via SSE on ProxmoxVMAll subscription — no explicit patch needed.
}

func (h *Handlers) vmResumeSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.resumeVM.Handle(r.Context(), commands.ResumeVMCommand{ID: id}); err != nil {
		slog.Error("proxmox: resume VM", "err", err)
		sse.PatchElementTempl(VMActionError(id, err.Error()))
		return
	}
}

func (h *Handlers) vmRestartSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.restartVM.Handle(r.Context(), commands.RestartVMCommand{ID: id}); err != nil {
		slog.Error("proxmox: restart VM", "err", err)
		sse.PatchElementTempl(VMActionError(id, err.Error()))
		return
	}
}

func (h *Handlers) vmDeleteSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteVM.Handle(r.Context(), commands.DeleteVMCommand{ID: id}); err != nil {
		slog.Error("proxmox: delete VM", "err", err)
		sse.PatchElementTempl(VMActionError(id, err.Error()))
		return
	}
}

func (h *Handlers) vmAssignCustomerSSE(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var sig struct {
		CustomerID string `json:"customerId"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.assignVMCustomer.Handle(r.Context(), commands.AssignVMCustomerCommand{VMID: id, CustomerID: sig.CustomerID}); err != nil {
		slog.Error("proxmox: assign VM customer", "err", err)
		sse.PatchElementTempl(VMActionError(id, err.Error()))
	}
}
