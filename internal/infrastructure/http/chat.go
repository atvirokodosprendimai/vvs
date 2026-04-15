package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/starfederation/datastar-go/datastar"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	"github.com/vvs/isp/internal/infrastructure/chat"
	"github.com/vvs/isp/internal/shared/events"
)

const chatHistoryLimit = 100

// ChatHandler serves the chat SSE stream and message-send endpoint.
type ChatHandler struct {
	store      *chat.Store
	subscriber events.EventSubscriber
	publisher  events.EventPublisher
}

// NewChatHandler creates a ChatHandler.
func NewChatHandler(store *chat.Store, sub events.EventSubscriber, pub events.EventPublisher) *ChatHandler {
	return &ChatHandler{store: store, subscriber: sub, publisher: pub}
}

// chatSSE streams the initial history then appends each new message in real time.
func (h *ChatHandler) chatSSE(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription("isp.chat.message")
	defer cancel()

	// Initial history — full replace of #chat-messages
	msgs, _ := h.store.Recent(r.Context(), chatHistoryLimit)
	sse.PatchElementTempl(ChatMessages(msgs, user.ID))
	scrollToBottom(sse)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			var msg chat.Message
			if err := json.Unmarshal(event.Data, &msg); err != nil {
				continue
			}
			// Append single message item without re-fetching history.
			var buf bytes.Buffer
			ChatMessageItem(msg, user.ID).Render(r.Context(), &buf)
			sse.PatchElements(buf.String(),
				datastar.WithSelector("#chat-messages"),
				datastar.WithMode(datastar.ElementPatchModeAppend),
			)
			scrollToBottom(sse)
		case <-r.Context().Done():
			return
		}
	}
}

// chatSend saves the message and publishes it to all connected SSE clients.
func (h *ChatHandler) chatSend(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var signals struct {
		ChatMsg string `json:"chatmsg"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body := strings.TrimSpace(signals.ChatMsg)
	if body == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	msg := chat.Message{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    user.ID,
		Username:  user.Username,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}

	if err := h.store.Save(r.Context(), msg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, _ := json.Marshal(msg)
	h.publisher.Publish(r.Context(), "isp.chat.message", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "chat.message",
		AggregateID: msg.ID,
		OccurredAt:  msg.CreatedAt,
		Data:        data,
	})

	w.WriteHeader(http.StatusNoContent)
}

func scrollToBottom(sse *datastar.ServerSentEventGenerator) {
	sse.ExecuteScript(
		`(function(){var el=document.getElementById('chat-messages');if(el)el.scrollTop=el.scrollHeight})()`,
	)
}
