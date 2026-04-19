package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	authdomain "github.com/vvs/isp/internal/modules/auth/domain"
	"github.com/vvs/isp/internal/modules/iptv/app/commands"
	"github.com/vvs/isp/internal/modules/iptv/app/queries"
)

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
	// Queries
	listChannels *queries.ListChannelsHandler
	listPackages *queries.ListPackagesHandler
	listSubs     *queries.ListSubscriptionsHandler
	listSTBs     *queries.ListSTBsHandler
	// EPG
	importEPG *commands.ImportEPGHandler
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
	listChannels *queries.ListChannelsHandler,
	listPackages *queries.ListPackagesHandler,
	listSubs *queries.ListSubscriptionsHandler,
	listSTBs *queries.ListSTBsHandler,
	importEPG *commands.ImportEPGHandler,
) *IPTVHandlers {
	return &IPTVHandlers{
		createChannel:   createChannel,
		updateChannel:   updateChannel,
		deleteChannel:   deleteChannel,
		createPackage:   createPackage,
		updatePackage:   updatePackage,
		deletePackage:   deletePackage,
		addChToPkg:      addChToPkg,
		removeChFromPkg: removeChFromPkg,
		createSub:       createSub,
		suspendSub:      suspendSub,
		reactivateSub:   reactivateSub,
		cancelSub:       cancelSub,
		revokeKey:       revokeKey,
		reissueKey:      reissueKey,
		assignSTB:       assignSTB,
		deleteSTB:       deleteSTB,
		listChannels:    listChannels,
		listPackages:    listPackages,
		listSubs:        listSubs,
		listSTBs:        listSTBs,
		importEPG:       importEPG,
	}
}

func (h *IPTVHandlers) ModuleName() authdomain.Module { return authdomain.ModuleIPTV }

func (h *IPTVHandlers) RegisterRoutes(r chi.Router) {
	// Pages
	r.Get("/iptv", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/iptv/channels", http.StatusFound)
	})
	r.Get("/iptv/channels", h.channelsPage)
	r.Get("/iptv/packages", h.packagesPage)
	r.Get("/iptv/subscriptions", h.subscriptionsPage)
	r.Get("/iptv/stbs", h.stbsPage)

	// SSE data endpoints
	r.Get("/sse/iptv/channels", h.channelsSSE)
	r.Get("/sse/iptv/packages", h.packagesSSE)
	r.Get("/sse/iptv/subscriptions", h.subscriptionsSSE)
	r.Get("/sse/iptv/stbs", h.stbsSSE)

	// Channel mutations
	r.Post("/api/iptv/channels", h.createChannelSSE)
	r.Put("/api/iptv/channels/{id}", h.updateChannelSSE)
	r.Delete("/api/iptv/channels/{id}", h.deleteChannelSSE)

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

	// EPG
	r.Post("/api/iptv/epg/import", h.epgImport)
}

// ── Page handlers ──────────────────────────────────────────────────────────────

func (h *IPTVHandlers) channelsPage(w http.ResponseWriter, r *http.Request) {
	IPTVChannelListPage().Render(r.Context(), w)
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
		Name      string `json:"iptv_ch_name"`
		LogoURL   string `json:"iptv_ch_logo"`
		StreamURL string `json:"iptv_ch_stream"`
		Category  string `json:"iptv_ch_category"`
		EPGSource string `json:"iptv_ch_epg"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
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
		"_iptvChOpen": false, "iptv_ch_name": "", "iptv_ch_logo": "",
		"iptv_ch_stream": "", "iptv_ch_category": "", "iptv_ch_epg": "",
	})
}

func (h *IPTVHandlers) updateChannelSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sig struct {
		Name      string `json:"iptv_ch_name"`
		LogoURL   string `json:"iptv_ch_logo"`
		StreamURL string `json:"iptv_ch_stream"`
		Category  string `json:"iptv_ch_category"`
		EPGSource string `json:"iptv_ch_epg"`
		Active    bool   `json:"iptv_ch_active"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
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
		Name        string `json:"iptv_pkg_name"`
		Price       string `json:"iptv_pkg_price"`
		Description string `json:"iptv_pkg_desc"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
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
		"_iptvPkgOpen": false, "iptv_pkg_name": "", "iptv_pkg_price": "", "iptv_pkg_desc": "",
	})
}

func (h *IPTVHandlers) updatePackageSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var sig struct {
		Name        string `json:"iptv_pkg_name"`
		Price       string `json:"iptv_pkg_price"`
		Description string `json:"iptv_pkg_desc"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
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
		CustomerID string `json:"iptv_sub_customer"`
		PackageID  string `json:"iptv_sub_package"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
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
		"_iptvSubOpen": false, "iptv_sub_customer": "", "iptv_sub_package": "",
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
		CustomerID string `json:"iptv_sub_customer_id"`
		PackageID  string `json:"iptv_sub_package_id"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
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
		MAC        string `json:"iptv_stb_mac"`
		Model      string `json:"iptv_stb_model"`
		CustomerID string `json:"iptv_stb_customer"`
		Notes      string `json:"iptv_stb_notes"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &sig); err != nil {
		sse.PatchElementTempl(iptvFormError("Invalid input"))
		return
	}
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
		"_iptvStbOpen": false, "iptv_stb_mac": "", "iptv_stb_model": "",
		"iptv_stb_customer": "", "iptv_stb_notes": "",
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

func clearSignals(sse *datastar.ServerSentEventGenerator, vals map[string]any) {
	b, _ := json.Marshal(vals)
	sse.PatchSignals(b)
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
