package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	authdomain "github.com/vvs/isp/internal/modules/auth/domain"
	"github.com/vvs/isp/internal/modules/iptv/adapters/persistence"
)

// IPTVHandlers serves the IPTV admin module routes.
type IPTVHandlers struct {
	channels      *persistence.ChannelRepository
	packages      *persistence.PackageRepository
	subscriptions *persistence.SubscriptionRepository
	stbs          *persistence.STBRepository
	keys          *persistence.SubscriptionKeyRepository
}

func NewIPTVHandlers(
	channels *persistence.ChannelRepository,
	packages *persistence.PackageRepository,
	subscriptions *persistence.SubscriptionRepository,
	stbs *persistence.STBRepository,
	keys *persistence.SubscriptionKeyRepository,
) *IPTVHandlers {
	return &IPTVHandlers{
		channels:      channels,
		packages:      packages,
		subscriptions: subscriptions,
		stbs:          stbs,
		keys:          keys,
	}
}

func (h *IPTVHandlers) ModuleName() authdomain.Module { return authdomain.ModuleIPTV }

func (h *IPTVHandlers) RegisterRoutes(r chi.Router) {
	r.Get("/iptv", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/iptv/channels", http.StatusFound)
	})
	r.Get("/iptv/channels", h.listChannels)
	r.Get("/iptv/packages", h.listPackages)
	r.Get("/iptv/subscriptions", h.listSubscriptions)
	r.Get("/iptv/stbs", h.listSTBs)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *IPTVHandlers) listChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.channels.FindAll(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	IPTVChannelListPage(channels).Render(r.Context(), w)
}

func (h *IPTVHandlers) listPackages(w http.ResponseWriter, r *http.Request) {
	packages, err := h.packages.FindAll(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	IPTVPackageListPage(packages).Render(r.Context(), w)
}

func (h *IPTVHandlers) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs, err := h.subscriptions.ListActive(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	IPTVSubscriptionListPage(subs).Render(r.Context(), w)
}

func (h *IPTVHandlers) listSTBs(w http.ResponseWriter, r *http.Request) {
	stbs, err := h.stbs.ListAll(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	IPTVSTBListPage(stbs).Render(r.Context(), w)
}
