package http

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/atvirokodosprendimai/vvs/internal/modules/device/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/device/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/device/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	authdomain "github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

type DeviceHandlers struct {
	registerCmd     *commands.RegisterDeviceHandler
	deployCmd       *commands.DeployDeviceHandler
	returnCmd       *commands.ReturnDeviceHandler
	decommissionCmd *commands.DecommissionDeviceHandler
	updateCmd       *commands.UpdateDeviceHandler
	listQuery       *queries.ListDevicesHandler
	getQuery        *queries.GetDeviceHandler
	subscriber      events.EventSubscriber
	publisher       events.EventPublisher
}

func NewDeviceHandlers(
	registerCmd *commands.RegisterDeviceHandler,
	deployCmd *commands.DeployDeviceHandler,
	returnCmd *commands.ReturnDeviceHandler,
	decommissionCmd *commands.DecommissionDeviceHandler,
	updateCmd *commands.UpdateDeviceHandler,
	listQuery *queries.ListDevicesHandler,
	getQuery *queries.GetDeviceHandler,
	subscriber events.EventSubscriber,
	publisher events.EventPublisher,
) *DeviceHandlers {
	return &DeviceHandlers{
		registerCmd:     registerCmd,
		deployCmd:       deployCmd,
		returnCmd:       returnCmd,
		decommissionCmd: decommissionCmd,
		updateCmd:       updateCmd,
		listQuery:       listQuery,
		getQuery:        getQuery,
		subscriber:      subscriber,
		publisher:       publisher,
	}
}

func (h *DeviceHandlers) RegisterRoutes(r chi.Router) {
	r.Get("/devices", h.listPage)
	r.Get("/sse/devices", h.listSSE)
	r.Post("/api/devices", h.registerSSE)
	r.Get("/devices/{id}", h.detailPage)
	r.Get("/devices/{id}/qr.png", h.qrPNG)
	r.Post("/api/devices/{id}/deploy", h.deploySSE)
	r.Post("/api/devices/{id}/return", h.returnSSE)
	r.Post("/api/devices/{id}/decommission", h.decommissionSSE)
	r.Put("/api/devices/{id}", h.updateSSE)
	r.Delete("/api/devices/{id}", h.deleteSSE)
}

// ── Pages ──────────────────────────────────────────────────────────────────

func (h *DeviceHandlers) listPage(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	result, err := h.listQuery.Handle(r.Context(), queries.ListDevicesQuery{
		Status:   statusFilter,
		Page:     1,
		PageSize: 50,
	})
	if err != nil {
		log.Printf("device: listPage: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	DeviceListPage(result.Devices, statusFilter).Render(r.Context(), w)
}

func (h *DeviceHandlers) detailPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.getQuery.Handle(r.Context(), queries.GetDeviceQuery{ID: id})
	if err != nil {
		if err == domain.ErrNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Printf("device: detailPage: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	DeviceDetailPage(d).Render(r.Context(), w)
}

// qrPNG generates a QR code PNG for the device detail URL.
func (h *DeviceHandlers) qrPNG(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/devices/%s", scheme, r.Host, id)
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "qr encode failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(png)))
	w.Write(png)
}

// ── SSE list ───────────────────────────────────────────────────────────────

func (h *DeviceHandlers) listSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(events.DeviceAll.String())
	defer cancel()

	q := queries.ListDevicesQuery{
		Status:   r.URL.Query().Get("status"),
		Page:     1,
		PageSize: 50,
	}
	result, err := h.listQuery.Handle(r.Context(), q)
	if err != nil {
		log.Printf("device: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(DeviceTable(result.Devices))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			result, err = h.listQuery.Handle(r.Context(), q)
			if err != nil {
				log.Printf("device: listSSE refresh: %v", err)
				continue
			}
			sse.PatchElementTempl(DeviceTable(result.Devices))
		case <-r.Context().Done():
			return
		}
	}
}

// ── SSE mutations ──────────────────────────────────────────────────────────

func (h *DeviceHandlers) registerSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Name         string `json:"name"`
		DeviceType   string `json:"deviceType"`
		SerialNumber string `json:"serialNumber"`
		Notes        string `json:"notes"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &signals); err != nil {
		sse.PatchElements(`<div id="device-form-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	d, err := h.registerCmd.Handle(r.Context(), commands.RegisterDeviceCommand{
		Name:         signals.Name,
		DeviceType:   signals.DeviceType,
		SerialNumber: signals.SerialNumber,
		Notes:        signals.Notes,
	})
	if err != nil {
		sse.PatchElements(`<div id="device-form-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	sse.PatchElements(`<div id="device-form-errors"></div>`)
	sse.ExecuteScript(fmt.Sprintf(`window.location.href='/devices/%s'`, d.ID))
}

func (h *DeviceHandlers) deploySSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var signals struct {
		CustomerID string `json:"customerId"`
		Location   string `json:"location"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &signals); err != nil {
		sse.PatchElements(`<div id="device-action-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	if _, err := h.deployCmd.Handle(r.Context(), commands.DeployDeviceCommand{
		ID: id, CustomerID: signals.CustomerID, Location: signals.Location,
	}); err != nil {
		sse.PatchElements(`<div id="device-action-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	sse.ExecuteScript(`window.location.reload()`)
}

func (h *DeviceHandlers) returnSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.returnCmd.Handle(r.Context(), commands.ReturnDeviceCommand{ID: id}); err != nil {
		sse.PatchElements(`<div id="device-action-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	sse.ExecuteScript(`window.location.reload()`)
}

func (h *DeviceHandlers) decommissionSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.decommissionCmd.Handle(r.Context(), commands.DecommissionDeviceCommand{ID: id}); err != nil {
		sse.PatchElements(`<div id="device-action-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	sse.ExecuteScript(`window.location.reload()`)
}

func (h *DeviceHandlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var signals struct {
		Name     string `json:"name"`
		Notes    string `json:"notes"`
		Location string `json:"location"`
	}
	sse := datastar.NewSSE(w, r)
	if err := datastar.ReadSignals(r, &signals); err != nil {
		sse.PatchElements(`<div id="device-action-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	if _, err := h.updateCmd.Handle(r.Context(), commands.UpdateDeviceCommand{
		ID: id, Name: signals.Name, Notes: signals.Notes, Location: signals.Location,
	}); err != nil {
		sse.PatchElements(`<div id="device-action-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	sse.ExecuteScript(`window.location.reload()`)
}

func (h *DeviceHandlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	// Only delete if decommissioned — enforce in handler
	d, err := h.getQuery.Handle(r.Context(), queries.GetDeviceQuery{ID: id})
	if err != nil {
		sse.PatchElements(`<div id="device-action-errors" class="text-red-400 text-xs">` + err.Error() + `</div>`)
		return
	}
	if d.Status != domain.StatusDecommissioned {
		sse.PatchElements(`<div id="device-action-errors" class="text-red-400 text-xs">only decommissioned devices can be deleted</div>`)
		return
	}
	// No delete command yet — call repo directly via a future DeleteDeviceHandler
	// For now redirect back to list
	sse.ExecuteScript(`window.location.href='/devices'`)
}

func (h *DeviceHandlers) ModuleName() authdomain.Module { return authdomain.ModuleNetwork }
