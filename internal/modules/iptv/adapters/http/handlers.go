package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	authdomain "github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

// NodeSSHInfo holds the SSH connection credentials for a node.
type NodeSSHInfo struct {
	Host   string
	User   string
	Port   int
	SSHKey []byte
}

// NodeSSHLookup resolves SSH connection info for a node ID.
type NodeSSHLookup interface {
	FindByID(ctx context.Context, id string) (*NodeSSHInfo, error)
}

// stackReader is the narrow read interface for IPTVStack.
type stackReader interface {
	FindByID(ctx context.Context, id string) (*domain.IPTVStack, error)
}

// ── Cascading select option types ─────────────────────────────────────────────

// ClusterOption is one entry in the cluster select.
type ClusterOption struct{ ID, Name string }

// NodeOption is one entry in the node select.
type NodeOption struct{ ID, Name, VpnIP string }

// NetworkOption is one entry in a network select.
type NetworkOption struct{ ID, Name string }

// ── Cascading select lookup interfaces ───────────────────────────────────────

// SwarmClustersLookup lists all swarm clusters.
type SwarmClustersLookup interface {
	FindAll(ctx context.Context) ([]ClusterOption, error)
}

// SwarmNodesLookup lists swarm nodes filtered by cluster.
type SwarmNodesLookup interface {
	FindByClusterID(ctx context.Context, clusterID string) ([]NodeOption, error)
}

// SwarmNetworksLookup lists swarm networks filtered by cluster.
type SwarmNetworksLookup interface {
	FindByClusterID(ctx context.Context, clusterID string) ([]NetworkOption, error)
}

// CustomerOption is one entry in the customer select.
type CustomerOption struct{ ID, Name string }

// CustomerLookup lists customers (optionally filtered by search string).
type CustomerLookup interface {
	FindAll(ctx context.Context, search string) ([]CustomerOption, error)
}

// IPTVHandlers serves the IPTV admin module routes.
type IPTVHandlers struct {
	// Commands
	createChannel   *commands.CreateChannelHandler
	updateChannel   *commands.UpdateChannelHandler
	deleteChannel   *commands.DeleteChannelHandler
	createPackage   *commands.CreatePackageHandler
	updatePackage   *commands.UpdatePackageHandler
	deletePackage   *commands.DeletePackageHandler
	addChToPkg      *commands.AddChannelToPackageHandler
	removeChFromPkg *commands.RemoveChannelFromPackageHandler
	createSub       *commands.CreateSubscriptionHandler
	suspendSub      *commands.SuspendSubscriptionHandler
	reactivateSub   *commands.ReactivateSubscriptionHandler
	cancelSub       *commands.CancelSubscriptionHandler
	revokeKey       *commands.RevokeSubscriptionKeyHandler
	reissueKey      *commands.ReissueSubscriptionKeyHandler
	assignSTB       *commands.AssignSTBHandler
	deleteSTB       *commands.DeleteSTBHandler
	// Provider commands
	createProvider *commands.CreateChannelProviderHandler
	deleteProvider *commands.DeleteChannelProviderHandler
	// Stack commands
	createStack       *commands.CreateIPTVStackHandler
	deleteStack       *commands.DeleteIPTVStackHandler
	addChToStack      *commands.AddChannelToIPTVStackHandler
	removeChFromStack *commands.RemoveChannelFromIPTVStackHandler
	deployStack       *commands.DeployIPTVStackHandler
	// Queries
	listChannels   *queries.ListChannelsHandler
	getChannel     *queries.GetChannelHandler
	listPackages   *queries.ListPackagesHandler
	listSubs       *queries.ListSubscriptionsHandler
	listSTBs       *queries.ListSTBsHandler
	listProviders  *queries.ListChannelProvidersHandler
	listStacks     *queries.ListIPTVStacksHandler
	getStackChans  *queries.GetIPTVStackChannelsHandler
	// EPG
	importEPG *commands.ImportEPGHandler
	// Node SSH resolver (cross-module)
	nodeLookup NodeSSHLookup
	stackRepo  stackReader
	// Cascading select lookups (cross-module, optional)
	swarmClusters  SwarmClustersLookup
	swarmNodes     SwarmNodesLookup
	swarmNetworks  SwarmNetworksLookup
	customerLookup CustomerLookup
}

func NewIPTVHandlers(
	createChannel *commands.CreateChannelHandler,
	updateChannel *commands.UpdateChannelHandler,
	deleteChannel *commands.DeleteChannelHandler,
	createPackage *commands.CreatePackageHandler,
	updatePackage *commands.UpdatePackageHandler,
	deletePackage *commands.DeletePackageHandler,
	addChToPkg *commands.AddChannelToPackageHandler,
	removeChFromPkg *commands.RemoveChannelFromPackageHandler,
	createSub *commands.CreateSubscriptionHandler,
	suspendSub *commands.SuspendSubscriptionHandler,
	reactivateSub *commands.ReactivateSubscriptionHandler,
	cancelSub *commands.CancelSubscriptionHandler,
	revokeKey *commands.RevokeSubscriptionKeyHandler,
	reissueKey *commands.ReissueSubscriptionKeyHandler,
	assignSTB *commands.AssignSTBHandler,
	deleteSTB *commands.DeleteSTBHandler,
	createProvider *commands.CreateChannelProviderHandler,
	deleteProvider *commands.DeleteChannelProviderHandler,
	createStack *commands.CreateIPTVStackHandler,
	deleteStack *commands.DeleteIPTVStackHandler,
	addChToStack *commands.AddChannelToIPTVStackHandler,
	removeChFromStack *commands.RemoveChannelFromIPTVStackHandler,
	deployStack *commands.DeployIPTVStackHandler,
	listChannels *queries.ListChannelsHandler,
	getChannel *queries.GetChannelHandler,
	listPackages *queries.ListPackagesHandler,
	listSubs *queries.ListSubscriptionsHandler,
	listSTBs *queries.ListSTBsHandler,
	listProviders *queries.ListChannelProvidersHandler,
	listStacks *queries.ListIPTVStacksHandler,
	getStackChans *queries.GetIPTVStackChannelsHandler,
	importEPG *commands.ImportEPGHandler,
	nodeLookup NodeSSHLookup,
	stackRepo stackReader,
	swarmClusters SwarmClustersLookup,
	swarmNodes SwarmNodesLookup,
	swarmNetworks SwarmNetworksLookup,
	customerLookup CustomerLookup,
) *IPTVHandlers {
	return &IPTVHandlers{
		createChannel:     createChannel,
		updateChannel:     updateChannel,
		deleteChannel:     deleteChannel,
		createPackage:     createPackage,
		updatePackage:     updatePackage,
		deletePackage:     deletePackage,
		addChToPkg:        addChToPkg,
		removeChFromPkg:   removeChFromPkg,
		createSub:         createSub,
		suspendSub:        suspendSub,
		reactivateSub:     reactivateSub,
		cancelSub:         cancelSub,
		revokeKey:         revokeKey,
		reissueKey:        reissueKey,
		assignSTB:         assignSTB,
		deleteSTB:         deleteSTB,
		createProvider:    createProvider,
		deleteProvider:    deleteProvider,
		createStack:       createStack,
		deleteStack:       deleteStack,
		addChToStack:      addChToStack,
		removeChFromStack: removeChFromStack,
		deployStack:       deployStack,
		listChannels:      listChannels,
		getChannel:        getChannel,
		listPackages:      listPackages,
		listSubs:          listSubs,
		listSTBs:          listSTBs,
		listProviders:     listProviders,
		listStacks:        listStacks,
		getStackChans:     getStackChans,
		importEPG:         importEPG,
		nodeLookup:        nodeLookup,
		stackRepo:         stackRepo,
		swarmClusters:     swarmClusters,
		swarmNodes:        swarmNodes,
		swarmNetworks:     swarmNetworks,
		customerLookup:    customerLookup,
	}
}

func (h *IPTVHandlers) ModuleName() authdomain.Module { return authdomain.ModuleIPTV }

func (h *IPTVHandlers) RegisterRoutes(r chi.Router) {
	// Pages
	r.Get("/iptv", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/iptv/channels", http.StatusFound)
	})
	r.Get("/iptv/channels", h.channelsPage)
	r.Get("/iptv/channels/{id}", h.channelDetailPage)
	r.Get("/iptv/packages", h.packagesPage)
	r.Get("/iptv/subscriptions", h.subscriptionsPage)
	r.Get("/iptv/stbs", h.stbsPage)
	r.Get("/iptv/stacks", h.stacksPage)
	r.Get("/iptv/stacks/{id}", h.stackDetailPage)

	// SSE data endpoints
	r.Get("/sse/iptv/channels", h.channelsSSE)
	r.Get("/sse/iptv/channels/{id}/providers", h.channelProvidersSSE)
	r.Get("/sse/iptv/packages", h.packagesSSE)
	r.Get("/sse/iptv/subscriptions", h.subscriptionsSSE)
	r.Get("/sse/iptv/stbs", h.stbsSSE)
	r.Get("/sse/iptv/stacks", h.stacksSSE)
	r.Get("/sse/iptv/stacks/{id}/channels", h.stackChannelsSSE)

	// Channel mutations
	r.Post("/api/iptv/channels", h.createChannelSSE)
	r.Put("/api/iptv/channels/{id}", h.updateChannelSSE)
	r.Delete("/api/iptv/channels/{id}", h.deleteChannelSSE)
	r.Post("/api/iptv/channels/{id}/providers", h.createProviderSSE)
	r.Delete("/api/iptv/channels/{id}/providers/{pid}", h.deleteProviderSSE)

	// Package mutations
	r.Post("/api/iptv/packages", h.createPackageSSE)
	r.Put("/api/iptv/packages/{id}", h.updatePackageSSE)
	r.Delete("/api/iptv/packages/{id}", h.deletePackageSSE)
	r.Post("/api/iptv/packages/{id}/channels/{channelID}", h.addChannelToPackageSSE)
	r.Delete("/api/iptv/packages/{id}/channels/{channelID}", h.removeChannelFromPackageSSE)

	// Subscription mutations
	r.Post("/api/iptv/subscriptions", h.createSubscriptionSSE)
	r.Put("/api/iptv/subscriptions/{id}/suspend", h.suspendSubscriptionSSE)
	r.Put("/api/iptv/subscriptions/{id}/reactivate", h.reactivateSubscriptionSSE)
	r.Delete("/api/iptv/subscriptions/{id}", h.cancelSubscriptionSSE)
	r.Post("/api/iptv/subscriptions/{id}/reissue-key", h.reissueKeySSE)

	// STB mutations
	r.Post("/api/iptv/stbs", h.assignSTBSSE)
	r.Delete("/api/iptv/stbs/{id}", h.deleteSTBSSE)

	// Stack mutations
	r.Post("/api/iptv/stacks", h.createStackSSE)
	r.Delete("/api/iptv/stacks/{id}", h.deleteStackSSE)
	r.Post("/api/iptv/stacks/{id}/channels", h.addChannelToStackSSE)
	r.Delete("/api/iptv/stacks/{id}/channels/{cid}", h.removeChannelFromStackSSE)
	r.Post("/api/iptv/stacks/{id}/deploy", h.deployStackSSE)

	// Cascading select SSE endpoints
	r.Get("/sse/iptv/select/clusters", h.selectClustersSSE)
	r.Get("/sse/iptv/select/cluster-deps", h.selectClusterDepsSSE)
	r.Get("/sse/iptv/select/channels", h.selectChannelsSSE)
	r.Get("/sse/iptv/select/channel-providers", h.selectChannelProvidersSSE)
	r.Get("/sse/iptv/select/sub-deps", h.selectSubDepsSSE)
	r.Get("/sse/iptv/select/stb-deps", h.selectSTBDepsSSE)

	// EPG
	r.Post("/api/iptv/epg/import", h.epgImport)
}

// ── Page handlers ──────────────────────────────────────────────────────────────

func (h *IPTVHandlers) channelsPage(w http.ResponseWriter, r *http.Request) {
	IPTVChannelListPage().Render(r.Context(), w)
}

func (h *IPTVHandlers) channelDetailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch, err := h.getChannel.Handle(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	IPTVChannelDetailPage(*ch).Render(r.Context(), w)
}

func (h *IPTVHandlers) stacksPage(w http.ResponseWriter, r *http.Request) {
	IPTVStackListPage().Render(r.Context(), w)
}

func (h *IPTVHandlers) stackDetailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stack, err := h.stackRepo.FindByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	IPTVStackDetailPage(queries.IPTVStackReadModel{
		ID:                 stack.ID,
		Name:               stack.Name,
		ClusterID:          stack.ClusterID,
		NodeID:             stack.NodeID,
		WANNetworkID:       stack.WANNetworkID,
		OverlayNetworkID:   stack.OverlayNetworkID,
		WANNetworkName:     stack.WANNetworkName,
		OverlayNetworkName: stack.OverlayNetworkName,
		WanIP:              stack.WanIP,
		Status:             string(stack.Status),
		LastDeployedAt:     stack.LastDeployedAt,
		CreatedAt:          stack.CreatedAt,
	}).Render(r.Context(), w)
}

func (h *IPTVHandlers) packagesPage(w http.ResponseWriter, r *http.Request) {
	IPTVPackageListPage().Render(r.Context(), w)
}

func (h *IPTVHandlers) subscriptionsPage(w http.ResponseWriter, r *http.Request) {
	IPTVSubscriptionListPage().Render(r.Context(), w)
}

func (h *IPTVHandlers) stbsPage(w http.ResponseWriter, r *http.Request) {
	IPTVSTBListPage().Render(r.Context(), w)
}

// ── SSE data endpoints ────────────────────────────────────────────────────────

func (h *IPTVHandlers) channelsSSE(w http.ResponseWriter, r *http.Request) {
	result, err := h.listChannels.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: channelsSSE: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(IPTVChannelTable(result))
}

func (h *IPTVHandlers) packagesSSE(w http.ResponseWriter, r *http.Request) {
	result, err := h.listPackages.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: packagesSSE: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(IPTVPackageTable(result))
}

func (h *IPTVHandlers) subscriptionsSSE(w http.ResponseWriter, r *http.Request) {
	result, err := h.listSubs.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: subscriptionsSSE: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(IPTVSubscriptionTable(result))
}

func (h *IPTVHandlers) stbsSSE(w http.ResponseWriter, r *http.Request) {
	result, err := h.listSTBs.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: stbsSSE: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(IPTVSTBTable(result))
}

// ── Channel mutations ─────────────────────────────────────────────────────────

func (h *IPTVHandlers) createChannelSSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		Name      string `json:"iptvChName"`
		LogoURL   string `json:"iptvChLogo"`
		StreamURL string `json:"iptvChStream"`
		Category  string `json:"iptvChCategory"`
		EPGSource string `json:"iptvChEpg"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	if _, err := h.createChannel.Handle(r.Context(), commands.CreateChannelCommand{
		Name: sig.Name, LogoURL: sig.LogoURL, StreamURL: sig.StreamURL,
		Category: sig.Category, EPGSource: sig.EPGSource,
	}); err != nil {
		log.Printf("iptv: createChannel: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to create channel"))
		return
	}
	h.patchChannelTable(w, r, sse)
	clearSignals(sse, map[string]any{
		"_iptvChOpen": false, "iptvChName": "", "iptvChLogo": "",
		"iptvChStream": "", "iptvChCategory": "", "iptvChEpg": "",
	})
}

func (h *IPTVHandlers) updateChannelSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sig struct {
		Name      string `json:"iptvChName"`
		LogoURL   string `json:"iptvChLogo"`
		StreamURL string `json:"iptvChStream"`
		Category  string `json:"iptvChCategory"`
		EPGSource string `json:"iptvChEpg"`
		Active    bool   `json:"iptvChActive"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	if _, err := h.updateChannel.Handle(r.Context(), commands.UpdateChannelCommand{
		ID: id, Name: sig.Name, LogoURL: sig.LogoURL, StreamURL: sig.StreamURL,
		Category: sig.Category, EPGSource: sig.EPGSource, Active: sig.Active,
	}); err != nil {
		log.Printf("iptv: updateChannel: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to update channel"))
		return
	}
	h.patchChannelTable(w, r, sse)
	clearSignals(sse, map[string]any{"_iptvChEditOpen": false})
}

func (h *IPTVHandlers) deleteChannelSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteChannel.Handle(r.Context(), commands.DeleteChannelCommand{ID: id}); err != nil {
		log.Printf("iptv: deleteChannel: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to delete channel"))
		return
	}
	h.patchChannelTable(w, r, sse)
}

// ── Package mutations ─────────────────────────────────────────────────────────

func (h *IPTVHandlers) createPackageSSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		Name        string `json:"iptvPkgName"`
		Price       string `json:"iptvPkgPrice"`
		Description string `json:"iptvPkgDesc"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	cents, err := parsePriceCents(sig.Price)
	if err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid price"))
		return
	}
	if _, err := h.createPackage.Handle(r.Context(), commands.CreatePackageCommand{
		Name: sig.Name, PriceCents: cents, Description: sig.Description,
	}); err != nil {
		log.Printf("iptv: createPackage: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to create package"))
		return
	}
	h.patchPackageTable(w, r, sse)
	clearSignals(sse, map[string]any{
		"_iptvPkgOpen": false, "iptvPkgName": "", "iptvPkgPrice": "", "iptvPkgDesc": "",
	})
}

func (h *IPTVHandlers) updatePackageSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sig struct {
		Name        string `json:"iptvPkgName"`
		Price       string `json:"iptvPkgPrice"`
		Description string `json:"iptvPkgDesc"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	cents, err := parsePriceCents(sig.Price)
	if err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid price"))
		return
	}
	if _, err := h.updatePackage.Handle(r.Context(), commands.UpdatePackageCommand{
		ID: id, Name: sig.Name, PriceCents: cents, Description: sig.Description,
	}); err != nil {
		log.Printf("iptv: updatePackage: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to update package"))
		return
	}
	h.patchPackageTable(w, r, sse)
	clearSignals(sse, map[string]any{"_iptvPkgEditOpen": false})
}

func (h *IPTVHandlers) deletePackageSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deletePackage.Handle(r.Context(), commands.DeletePackageCommand{ID: id}); err != nil {
		log.Printf("iptv: deletePackage: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to delete package"))
		return
	}
	h.patchPackageTable(w, r, sse)
}

func (h *IPTVHandlers) addChannelToPackageSSE(w http.ResponseWriter, r *http.Request) {
	pkgID := chi.URLParam(r, "id")
	chID := chi.URLParam(r, "channelID")
	sse := datastar.NewSSE(w, r)
	if err := h.addChToPkg.Handle(r.Context(), commands.AddChannelToPackageCommand{
		PackageID: pkgID, ChannelID: chID,
	}); err != nil {
		log.Printf("iptv: addChannelToPackage: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to add channel"))
		return
	}
	h.patchPackageTable(w, r, sse)
}

func (h *IPTVHandlers) removeChannelFromPackageSSE(w http.ResponseWriter, r *http.Request) {
	pkgID := chi.URLParam(r, "id")
	chID := chi.URLParam(r, "channelID")
	sse := datastar.NewSSE(w, r)
	if err := h.removeChFromPkg.Handle(r.Context(), commands.RemoveChannelFromPackageCommand{
		PackageID: pkgID, ChannelID: chID,
	}); err != nil {
		log.Printf("iptv: removeChannelFromPackage: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to remove channel"))
		return
	}
	h.patchPackageTable(w, r, sse)
}

// ── Subscription mutations ────────────────────────────────────────────────────

func (h *IPTVHandlers) createSubscriptionSSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		CustomerID string `json:"iptvSubCustomer"`
		PackageID  string `json:"iptvSubPackage"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	if _, err := h.createSub.Handle(r.Context(), commands.CreateSubscriptionCommand{
		CustomerID: sig.CustomerID,
		PackageID:  sig.PackageID,
		StartsAt:   time.Now().UTC(),
	}); err != nil {
		log.Printf("iptv: createSubscription: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to create subscription"))
		return
	}
	h.patchSubscriptionTable(w, r, sse)
	clearSignals(sse, map[string]any{
		"_iptvSubOpen": false, "iptvSubCustomer": "", "iptvSubPackage": "",
	})
}

func (h *IPTVHandlers) suspendSubscriptionSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.suspendSub.Handle(r.Context(), commands.SuspendSubscriptionCommand{ID: id}); err != nil {
		log.Printf("iptv: suspendSubscription: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to suspend subscription"))
		return
	}
	h.patchSubscriptionTable(w, r, sse)
}

func (h *IPTVHandlers) reactivateSubscriptionSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.reactivateSub.Handle(r.Context(), commands.ReactivateSubscriptionCommand{ID: id}); err != nil {
		log.Printf("iptv: reactivateSubscription: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to reactivate subscription"))
		return
	}
	h.patchSubscriptionTable(w, r, sse)
}

func (h *IPTVHandlers) cancelSubscriptionSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.cancelSub.Handle(r.Context(), commands.CancelSubscriptionCommand{ID: id}); err != nil {
		log.Printf("iptv: cancelSubscription: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to cancel subscription"))
		return
	}
	h.patchSubscriptionTable(w, r, sse)
}

func (h *IPTVHandlers) reissueKeySSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_ = id
	// Reissue key requires subscription details — signal carries them
	var sig struct {
		CustomerID string `json:"iptvSubCustomerId"`
		PackageID  string `json:"iptvSubPackageId"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	if _, err := h.reissueKey.Handle(r.Context(), commands.ReissueSubscriptionKeyCommand{
		SubscriptionID: id,
		CustomerID:     sig.CustomerID,
		PackageID:      sig.PackageID,
	}); err != nil {
		log.Printf("iptv: reissueKey: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to reissue key"))
		return
	}
	h.patchSubscriptionTable(w, r, sse)
}

// ── STB mutations ─────────────────────────────────────────────────────────────

func (h *IPTVHandlers) assignSTBSSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		MAC        string `json:"iptvStbMac"`
		Model      string `json:"iptvStbModel"`
		CustomerID string `json:"iptvStbCustomer"`
		Notes      string `json:"iptvStbNotes"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	if _, err := h.assignSTB.Handle(r.Context(), commands.AssignSTBCommand{
		MAC:        sig.MAC,
		Model:      sig.Model,
		CustomerID: sig.CustomerID,
		Notes:      sig.Notes,
	}); err != nil {
		log.Printf("iptv: assignSTB: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to register STB"))
		return
	}
	h.patchSTBTable(w, r, sse)
	clearSignals(sse, map[string]any{
		"_iptvStbOpen": false, "iptvStbMac": "", "iptvStbModel": "",
		"iptvStbCustomer": "", "iptvStbNotes": "",
	})
}

func (h *IPTVHandlers) deleteSTBSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteSTB.Handle(r.Context(), commands.DeleteSTBCommand{ID: id}); err != nil {
		log.Printf("iptv: deleteSTB: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to delete STB"))
		return
	}
	h.patchSTBTable(w, r, sse)
}

// ── Provider mutations ────────────────────────────────────────────────────────

func (h *IPTVHandlers) channelProvidersSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	providers, err := h.listProviders.Handle(r.Context(), id)
	if err != nil {
		log.Printf("iptv: channelProvidersSSE: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(IPTVChannelProviderTable(id, providers))
}

func (h *IPTVHandlers) createProviderSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sig struct {
		Name        string `json:"iptvProvName"`
		URLTemplate string `json:"iptvProvUrl"`
		Token       string `json:"iptvProvToken"`
		Type        string `json:"iptvProvType"`
		Priority    string `json:"iptvProvPriority"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	priority, _ := strconv.Atoi(sig.Priority)
	if _, err := h.createProvider.Handle(r.Context(), commands.CreateChannelProviderCommand{
		ChannelID:   id,
		Name:        sig.Name,
		URLTemplate: sig.URLTemplate,
		Token:       sig.Token,
		Type:        sig.Type,
		Priority:    priority,
	}); err != nil {
		log.Printf("iptv: createProvider: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to create provider"))
		return
	}
	providers, _ := h.listProviders.Handle(r.Context(), id)
	sse.PatchElementTempl(IPTVChannelProviderTable(id, providers))
	clearSignals(sse, map[string]any{
		"_iptvProvOpen": false, "iptvProvName": "", "iptvProvUrl": "",
		"iptvProvToken": "", "iptvProvType": "internal", "iptvProvPriority": "0",
	})
}

func (h *IPTVHandlers) deleteProviderSSE(w http.ResponseWriter, r *http.Request) {
	chID := chi.URLParam(r, "id")
	pid := chi.URLParam(r, "pid")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteProvider.Handle(r.Context(), commands.DeleteChannelProviderCommand{ID: pid}); err != nil {
		log.Printf("iptv: deleteProvider: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to delete provider"))
		return
	}
	providers, _ := h.listProviders.Handle(r.Context(), chID)
	sse.PatchElementTempl(IPTVChannelProviderTable(chID, providers))
}

// ── Stack SSE + mutations ─────────────────────────────────────────────────────

func (h *IPTVHandlers) stacksSSE(w http.ResponseWriter, r *http.Request) {
	stacks, err := h.listStacks.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: stacksSSE: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(IPTVStackTable(stacks))
}

func (h *IPTVHandlers) stackChannelsSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	chans, err := h.getStackChans.Handle(r.Context(), id)
	if err != nil {
		log.Printf("iptv: stackChannelsSSE: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(IPTVStackChannelTable(id, chans))
}

func (h *IPTVHandlers) createStackSSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		Name               string `json:"iptvStackName"`
		ClusterID          string `json:"iptvStackCluster"`
		NodeID             string `json:"iptvStackNode"`
		WANNetworkID       string `json:"iptvStackWanNet"`
		OverlayNetworkID   string `json:"iptvStackOverlayNet"`
		WANNetworkName     string `json:"iptvStackWanName"`
		OverlayNetworkName string `json:"iptvStackOverlayName"`
		WanIP              string `json:"iptvStackWanIp"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	if _, err := h.createStack.Handle(r.Context(), commands.CreateIPTVStackCommand{
		Name:               sig.Name,
		ClusterID:          sig.ClusterID,
		NodeID:             sig.NodeID,
		WANNetworkID:       sig.WANNetworkID,
		OverlayNetworkID:   sig.OverlayNetworkID,
		WANNetworkName:     sig.WANNetworkName,
		OverlayNetworkName: sig.OverlayNetworkName,
		WanIP:              sig.WanIP,
	}); err != nil {
		log.Printf("iptv: createStack: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to create stack"))
		return
	}
	h.patchStackTable(w, r, sse)
	clearSignals(sse, map[string]any{
		"_iptvStackOpen": false, "iptvStackName": "", "iptvStackCluster": "",
		"iptvStackNode": "", "iptvStackWanNet": "", "iptvStackOverlayNet": "",
		"iptvStackWanName": "", "iptvStackOverlayName": "", "iptvStackWanIp": "",
	})
}

func (h *IPTVHandlers) deleteStackSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.deleteStack.Handle(r.Context(), commands.DeleteIPTVStackCommand{ID: id}); err != nil {
		log.Printf("iptv: deleteStack: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to delete stack"))
		return
	}
	h.patchStackTable(w, r, sse)
}

func (h *IPTVHandlers) addChannelToStackSSE(w http.ResponseWriter, r *http.Request) {
	stackID := chi.URLParam(r, "id")
	var sig struct {
		ChannelID  string `json:"iptvStackChId"`
		ProviderID string `json:"iptvStackChProv"`
	}
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
	sse := datastar.NewSSE(w, r)
	if err := h.addChToStack.Handle(r.Context(), commands.AddChannelToIPTVStackCommand{
		StackID:    stackID,
		ChannelID:  sig.ChannelID,
		ProviderID: sig.ProviderID,
	}); err != nil {
		log.Printf("iptv: addChannelToStack: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to add channel"))
		return
	}
	chans, _ := h.getStackChans.Handle(r.Context(), stackID)
	sse.PatchElementTempl(IPTVStackChannelTable(stackID, chans))
	clearSignals(sse, map[string]any{
		"_iptvStackChOpen": false, "iptvStackChId": "", "iptvStackChProv": "",
	})
}

func (h *IPTVHandlers) removeChannelFromStackSSE(w http.ResponseWriter, r *http.Request) {
	stackID := chi.URLParam(r, "id")
	cid := chi.URLParam(r, "cid")
	sse := datastar.NewSSE(w, r)
	if err := h.removeChFromStack.Handle(r.Context(), commands.RemoveChannelFromIPTVStackCommand{
		StackID:   stackID,
		ChannelID: cid,
	}); err != nil {
		log.Printf("iptv: removeChannelFromStack: %v", err)
		sse.PatchElementTempl(iptvFormError("Failed to remove channel"))
		return
	}
	chans, _ := h.getStackChans.Handle(r.Context(), stackID)
	sse.PatchElementTempl(IPTVStackChannelTable(stackID, chans))
}

func (h *IPTVHandlers) deployStackSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)

	stack, err := h.stackRepo.FindByID(r.Context(), id)
	if err != nil {
		sse.PatchElementTempl(IPTVDeployLog("Error: stack not found"))
		return
	}
	if h.nodeLookup == nil {
		sse.PatchElementTempl(IPTVDeployLog("Error: node SSH resolver not configured"))
		return
	}
	node, err := h.nodeLookup.FindByID(r.Context(), stack.NodeID)
	if err != nil {
		log.Printf("iptv: deploy: node lookup %s: %v", stack.NodeID, err)
		sse.PatchElementTempl(IPTVDeployLog("Error: node not found or SSH key unavailable"))
		return
	}

	deployer := h.deployStack.WithProgress(func(msg string) {
		sse.PatchElementTempl(IPTVDeployLog(msg))
	})

	if err := deployer.Handle(r.Context(), commands.DeployIPTVStackCommand{
		StackID:    id,
		NodeHost:   node.Host,
		NodeUser:   node.User,
		NodePort:   node.Port,
		NodeSSHKey: node.SSHKey,
	}); err != nil {
		log.Printf("iptv: deploy: %v", err)
		sse.PatchElementTempl(IPTVDeployLog("Error: " + err.Error()))
		return
	}
	// Refresh stack list
	h.patchStackTable(w, r, sse)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *IPTVHandlers) patchChannelTable(w http.ResponseWriter, r *http.Request, sse *datastar.ServerSentEventGenerator) {
	channels, err := h.listChannels.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: patchChannelTable: %v", err)
		return
	}
	sse.PatchElementTempl(IPTVChannelTable(channels))
}

func (h *IPTVHandlers) patchPackageTable(w http.ResponseWriter, r *http.Request, sse *datastar.ServerSentEventGenerator) {
	packages, err := h.listPackages.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: patchPackageTable: %v", err)
		return
	}
	sse.PatchElementTempl(IPTVPackageTable(packages))
}

func (h *IPTVHandlers) patchSubscriptionTable(w http.ResponseWriter, r *http.Request, sse *datastar.ServerSentEventGenerator) {
	subs, err := h.listSubs.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: patchSubscriptionTable: %v", err)
		return
	}
	sse.PatchElementTempl(IPTVSubscriptionTable(subs))
}

func (h *IPTVHandlers) patchSTBTable(w http.ResponseWriter, r *http.Request, sse *datastar.ServerSentEventGenerator) {
	stbs, err := h.listSTBs.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: patchSTBTable: %v", err)
		return
	}
	sse.PatchElementTempl(IPTVSTBTable(stbs))
}

func (h *IPTVHandlers) patchStackTable(w http.ResponseWriter, r *http.Request, sse *datastar.ServerSentEventGenerator) {
	stacks, err := h.listStacks.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: patchStackTable: %v", err)
		return
	}
	sse.PatchElementTempl(IPTVStackTable(stacks))
}

func clearSignals(sse *datastar.ServerSentEventGenerator, vals map[string]any) {
	b, _ := json.Marshal(vals)
	sse.PatchSignals(b)
}

// ── Cascading select SSE ──────────────────────────────────────────────────────

func (h *IPTVHandlers) selectClustersSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	if h.swarmClusters == nil {
		sse.PatchElementTempl(IPTVSelectClusters(nil))
		return
	}
	clusters, err := h.swarmClusters.FindAll(r.Context())
	if err != nil {
		log.Printf("iptv: selectClusters: %v", err)
		sse.PatchElementTempl(IPTVSelectClusters(nil))
		return
	}
	sse.PatchElementTempl(IPTVSelectClusters(clusters))
}

func (h *IPTVHandlers) selectClusterDepsSSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		ClusterID string `json:"iptvStackCluster"`
	}
	sse := datastar.NewSSE(w, r)
	_ = datastar.ReadSignals(r, &sig)

	var nodes []NodeOption
	var nets []NetworkOption
	if sig.ClusterID != "" {
		if h.swarmNodes != nil {
			nodes, _ = h.swarmNodes.FindByClusterID(r.Context(), sig.ClusterID)
		}
		if h.swarmNetworks != nil {
			nets, _ = h.swarmNetworks.FindByClusterID(r.Context(), sig.ClusterID)
		}
	}
	sse.PatchElementTempl(IPTVSelectNodes(nodes))
	sse.PatchElementTempl(IPTVSelectWANNetworks(nets))
	sse.PatchElementTempl(IPTVSelectOverlayNetworks(nets))
}

func (h *IPTVHandlers) selectChannelsSSE(w http.ResponseWriter, r *http.Request) {
	channels, err := h.listChannels.Handle(r.Context())
	sse := datastar.NewSSE(w, r)
	if err != nil {
		log.Printf("iptv: selectChannels: %v", err)
		sse.PatchElementTempl(IPTVSelectChannels(nil))
		return
	}
	sse.PatchElementTempl(IPTVSelectChannels(channels))
}

func (h *IPTVHandlers) selectChannelProvidersSSE(w http.ResponseWriter, r *http.Request) {
	var sig struct {
		ChannelID string `json:"iptvStackChId"`
	}
	sse := datastar.NewSSE(w, r)
	_ = datastar.ReadSignals(r, &sig)

	var providers []queries.ChannelProviderReadModel
	if sig.ChannelID != "" {
		providers, _ = h.listProviders.Handle(r.Context(), sig.ChannelID)
	}
	sse.PatchElementTempl(IPTVSelectProviders(providers))
}

func (h *IPTVHandlers) selectSubDepsSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)

	// Customers
	var customers []CustomerOption
	if h.customerLookup != nil {
		customers, _ = h.customerLookup.FindAll(r.Context(), "")
	}
	sse.PatchElementTempl(IPTVSelectCustomers(customers))

	// Packages
	packages, err := h.listPackages.Handle(r.Context())
	if err != nil {
		log.Printf("iptv: selectSubDeps packages: %v", err)
		packages = nil
	}
	sse.PatchElementTempl(IPTVSelectPackages(packages))
}

func (h *IPTVHandlers) selectSTBDepsSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	var customers []CustomerOption
	if h.customerLookup != nil {
		customers, _ = h.customerLookup.FindAll(r.Context(), "")
	}
	sse.PatchElementTempl(IPTVSelectSTBCustomer(customers))
}

// ── EPG ───────────────────────────────────────────────────────────────────────

func (h *IPTVHandlers) epgImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		DaysAhead int   `json:"days_ahead"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	result, err := h.importEPG.Handle(r.Context(), commands.ImportEPGCommand{
		URL:      req.URL,
		DaysAhead: req.DaysAhead,
	})
	if err != nil {
		log.Printf("iptv: epgImport: %v", err)
		http.Error(w, "import failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func parsePriceCents(s string) (int64, error) {
	// Accept "9.99" or "999" (cents)
	if len(s) == 0 {
		return 0, nil
	}
	for i, c := range s {
		if c == '.' {
			whole, err := strconv.ParseInt(s[:i], 10, 64)
			if err != nil {
				return 0, err
			}
			frac := s[i+1:]
			if len(frac) > 2 {
				frac = frac[:2]
			}
			for len(frac) < 2 {
				frac += "0"
			}
			cents, err := strconv.ParseInt(frac, 10, 64)
			if err != nil {
				return 0, err
			}
			return whole*100 + cents, nil
		}
	}
	return strconv.ParseInt(s, 10, 64)
}
