package http

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/modules/network/domain"
)

// prefixRoutes registers prefix management routes. Call from RegisterRoutes.
func (h *Handlers) prefixRoutes(r chi.Router) {
	r.Get("/prefixes", h.prefixListPage)
	r.Get("/sse/prefixes", h.prefixListSSE)
	r.Post("/api/prefixes", h.prefixAddSSE)
	r.Delete("/api/prefixes/{id}", h.prefixDeleteSSE)
}

func (h *Handlers) prefixListPage(w http.ResponseWriter, r *http.Request) {
	prefixes, err := h.prefixRepo.ListAll(r.Context())
	if err != nil {
		log.Printf("prefixes: list: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	PrefixListPage(prefixes).Render(r.Context(), w)
}

func (h *Handlers) prefixListSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	prefixes, err := h.prefixRepo.ListAll(r.Context())
	if err != nil {
		log.Printf("prefixes: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(PrefixTable(prefixes))
}

func (h *Handlers) prefixAddSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		NetBoxID string `json:"prefixNetboxId"`
		CIDR     string `json:"prefixCidr"`
		Location string `json:"prefixLocation"`
		Priority string `json:"prefixPriority"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("prefixes: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	nbID, err := strconv.Atoi(strings.TrimSpace(signals.NetBoxID))
	if err != nil || nbID <= 0 {
		sse.PatchElements(`<div id="prefix-form-errors" class="text-red-400 text-xs mt-1">NetBox ID must be a positive integer</div>`)
		return
	}
	priority, _ := strconv.Atoi(strings.TrimSpace(signals.Priority))

	p, err := domain.NewNetBoxPrefix(nbID, signals.CIDR, signals.Location, priority)
	if err != nil {
		sse.PatchElements(`<div id="prefix-form-errors" class="text-red-400 text-xs mt-1">` + err.Error() + `</div>`)
		return
	}
	if err := h.prefixRepo.Save(r.Context(), p); err != nil {
		log.Printf("prefixes: save: %v", err)
		sse.PatchElements(`<div id="prefix-form-errors" class="text-red-400 text-xs mt-1">internal error</div>`)
		return
	}

	sse.PatchElements(`<div id="prefix-form-errors"></div>`)
	sse.PatchSignals([]byte(`{"_prefixAddOpen":false}`))
	h.pushPrefixTable(sse, r)
}

func (h *Handlers) prefixDeleteSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")
	if err := h.prefixRepo.Delete(r.Context(), id); err != nil {
		log.Printf("prefixes: delete %s: %v", id, err)
	}
	h.pushPrefixTable(sse, r)
}

func (h *Handlers) pushPrefixTable(sse *datastar.ServerSentEventGenerator, r *http.Request) {
	prefixes, err := h.prefixRepo.ListAll(r.Context())
	if err != nil {
		log.Printf("prefixes: pushTable: %v", err)
		return
	}
	sse.PatchElementTempl(PrefixTable(prefixes))
}
