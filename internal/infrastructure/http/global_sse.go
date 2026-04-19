package http

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/chat"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/metrics"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/notifications"
	authhttp "github.com/atvirokodosprendimai/vvs/internal/modules/auth/adapters/http"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// GlobalHandler serves the single /sse endpoint that merges clock,
// notifications, and chat-widget streams into one HTTP/1.1 connection.
type GlobalHandler struct {
	notifStore *notifications.Store
	chatStore  *chat.Store
	subscriber events.EventSubscriber
}

func NewGlobalHandler(notifStore *notifications.Store, chatStore *chat.Store, sub events.EventSubscriber) *GlobalHandler {
	return &GlobalHandler{notifStore: notifStore, chatStore: chatStore, subscriber: sub}
}

func (h *GlobalHandler) globalSSE(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	metrics.ActiveSSEConns.Inc()
	defer metrics.ActiveSSEConns.Dec()

	sse := datastar.NewSSE(w, r)

	clockTicker := time.NewTicker(time.Second)
	defer clockTicker.Stop()

	notifCh, cancelNotif := h.subscriber.ChanSubscription(events.Notifications.String())
	defer cancelNotif()

	chatCh, cancelChat := h.subscriber.ChanSubscription(events.ChatMessageGeneral.String())
	defer cancelChat()

	// Initial state — inject user role so templates can show viewer badge
	sse.PatchSignals([]byte(`{"_userRole":"` + string(user.Role) + `"}`))
	sse.PatchElementTempl(ClockFragment(time.Now().Format("15:04:05")))
	h.sendNotif(r, sse, user.ID)
	msgs, err := h.chatStore.Recent(r.Context(), "general", chatHistoryLimit)
	if err != nil {
		log.Printf("globalSSE: Recent: %v", err)
	}
	sse.PatchElementTempl(ChatWidgetMessages(msgs, user.ID))
	widgetScrollToBottom(sse)

	for {
		select {
		case t, ok := <-clockTicker.C:
			if !ok || sse.IsClosed() {
				return
			}
			sse.PatchElementTempl(ClockFragment(t.Format("15:04:05")))
		case _, ok := <-notifCh:
			if !ok {
				return
			}
			h.sendNotif(r, sse, user.ID)
		case event, ok := <-chatCh:
			if !ok {
				return
			}
			var msg chat.Message
			if err := json.Unmarshal(event.Data, &msg); err == nil {
				var buf bytes.Buffer
				ChatMessageItem(msg, user.ID).Render(r.Context(), &buf)
				sse.PatchElements(buf.String(),
					datastar.WithSelector("#widget-messages"),
					datastar.WithMode(datastar.ElementPatchModeAppend),
				)
				widgetScrollToBottom(sse)
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (h *GlobalHandler) sendNotif(r *http.Request, sse *datastar.ServerSentEventGenerator, userID string) {
	count, err := h.notifStore.UnreadCount(r.Context(), userID)
	if err != nil {
		log.Printf("globalSSE: UnreadCount: %v", err)
	}
	notifs, err := h.notifStore.List(r.Context(), userID, notifListLimit)
	if err != nil {
		log.Printf("globalSSE: List notifications: %v", err)
	}
	sse.PatchElementTempl(NotifBadge(count))
	sse.PatchElementTempl(NotifList(notifs))
}

func widgetScrollToBottom(sse *datastar.ServerSentEventGenerator) {
	sse.ExecuteScript(
		`(function(){var el=document.getElementById('widget-messages');if(el)el.scrollTop=el.scrollHeight})()`,
	)
}
