package http

import (
	"log"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	"github.com/vvs/isp/internal/infrastructure/notifications"
	"github.com/vvs/isp/internal/shared/events"
)

const notifListLimit = 30

// NotifHandler serves the notification SSE stream and mark-as-read endpoint.
type NotifHandler struct {
	store      *notifications.Store
	subscriber events.EventSubscriber
}

// NewNotifHandler creates a NotifHandler.
func NewNotifHandler(store *notifications.Store, sub events.EventSubscriber) *NotifHandler {
	return &NotifHandler{store: store, subscriber: sub}
}

// notificationsSSE streams badge count and list patches to the browser.
// Subscribed via data-init="@get('/sse/notifications')" in the sidebar.
func (h *NotifHandler) notificationsSSE(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription("isp.notifications")
	defer cancel()

	// Initial render
	h.patch(r, sse, user.ID)

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			h.patch(r, sse, user.ID)
		case <-r.Context().Done():
			return
		}
	}
}

// markRead records all current notifications as read for the user and patches
// the badge to 0. The list is also re-rendered to update read styling.
func (h *NotifHandler) markRead(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.store.MarkAllRead(r.Context(), user.ID); err != nil {
		log.Printf("markRead: MarkAllRead: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)
	h.patch(r, sse, user.ID)
}

// patch queries the DB and sends badge + list updates to the client.
func (h *NotifHandler) patch(r *http.Request, sse *datastar.ServerSentEventGenerator, userID string) {
	count, err := h.store.UnreadCount(r.Context(), userID)
	if err != nil {
		log.Printf("notif patch: UnreadCount: %v", err)
	}
	notifs, err := h.store.List(r.Context(), userID, notifListLimit)
	if err != nil {
		log.Printf("notif patch: List: %v", err)
	}
	sse.PatchElementTempl(NotifBadge(count))
	sse.PatchElementTempl(NotifList(notifs))
}
