package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/infrastructure/chat"
	"github.com/vvs/isp/internal/infrastructure/notifications"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	"github.com/vvs/isp/internal/shared/events"
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

	sse := datastar.NewSSE(w, r)

	clockTicker := time.NewTicker(time.Second)
	defer clockTicker.Stop()

	notifCh, cancelNotif := h.subscriber.ChanSubscription("isp.notifications")
	defer cancelNotif()

	chatCh, cancelChat := h.subscriber.ChanSubscription("isp.chat.message.general")
	defer cancelChat()

	// Initial state
	sse.PatchElementTempl(ClockFragment(time.Now().Format("15:04:05")))
	h.sendNotif(r, sse, user.ID)
	msgs, _ := h.chatStore.Recent(r.Context(), "general", chatHistoryLimit)
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
	count, _ := h.notifStore.UnreadCount(r.Context(), userID)
	notifs, _ := h.notifStore.List(r.Context(), userID, notifListLimit)
	sse.PatchElementTempl(NotifBadge(count))
	sse.PatchElementTempl(NotifList(notifs))
}

func widgetScrollToBottom(sse *datastar.ServerSentEventGenerator) {
	sse.ExecuteScript(
		`(function(){var el=document.getElementById('widget-messages');if(el)el.scrollTop=el.scrollHeight})()`,
	)
}
