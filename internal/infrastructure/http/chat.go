package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/vvs/isp/internal/infrastructure/chat"
	"github.com/vvs/isp/internal/shared/events"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
)

const chatHistoryLimit = 100

// ChatHandler serves all chat SSE streams and API endpoints.
type ChatHandler struct {
	store      *chat.Store
	subscriber events.EventSubscriber
	publisher  events.EventPublisher
}

// NewChatHandler creates a ChatHandler.
func NewChatHandler(store *chat.Store, sub events.EventSubscriber, pub events.EventPublisher) *ChatHandler {
	return &ChatHandler{store: store, subscriber: sub, publisher: pub}
}

// chatPage renders the full /chat page.
func (h *ChatHandler) chatPage(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	_ = h.store.EnsurePublicMembership(r.Context(), user.ID)
	threadID := r.URL.Query().Get("thread")
	threads, _ := h.store.ListThreadsForUser(r.Context(), user.ID)
	ChatPage(threads, user.ID, threadID).Render(r.Context(), w)
}

// chatSSE is the widget SSE — always streams the #general thread.
func (h *ChatHandler) chatSSE(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	h.streamMessages(w, r, "general", user.ID)
}

// threadMessagesSSE streams messages for a specific thread (used by /chat page).
func (h *ChatHandler) threadMessagesSSE(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	threadID := chi.URLParam(r, "threadID")
	if threadID == "" {
		http.Error(w, "missing thread id", http.StatusBadRequest)
		return
	}

	// For public channels, auto-join on first access.
	member, err := h.store.IsMember(r.Context(), threadID, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !member {
		// Check if it's a public channel; if so, add as member.
		channels, _ := h.store.ListPublicChannels(r.Context())
		isPublic := false
		for _, c := range channels {
			if c.ID == threadID {
				isPublic = true
				break
			}
		}
		if !isPublic {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_ = h.store.AddMember(r.Context(), threadID, user.ID)
	}

	// Mark thread as read on connect.
	_ = h.store.MarkRead(r.Context(), threadID, user.ID)

	h.streamMessages(w, r, threadID, user.ID)
}

// streamMessages is the shared SSE loop for a given thread.
func (h *ChatHandler) streamMessages(w http.ResponseWriter, r *http.Request, threadID, userID string) {
	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription("isp.chat.message." + threadID)
	defer cancel()

	msgs, _ := h.store.Recent(r.Context(), threadID, chatHistoryLimit)
	sse.PatchElementTempl(ChatMessages(msgs, userID))
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
			var buf bytes.Buffer
			ChatMessageItem(msg, userID).Render(r.Context(), &buf)
			sse.PatchElements(buf.String(),
				datastar.WithSelector("#chat-messages"),
				datastar.WithMode(datastar.ElementPatchModeAppend),
			)
			scrollToBottom(sse)
			_ = h.store.MarkRead(r.Context(), threadID, userID)
		case <-r.Context().Done():
			return
		}
	}
}

// threadsSSE streams the thread list for the current user.
func (h *ChatHandler) threadsSSE(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	_ = h.store.EnsurePublicMembership(r.Context(), user.ID)

	sse := datastar.NewSSE(w, r)

	// Subscribe to all chat events — any message in any thread may affect the list.
	ch, cancel := h.subscriber.ChanSubscription("isp.chat.>")
	defer cancel()

	current, _ := h.store.ListThreadsForUser(r.Context(), user.ID)
	sse.PatchElementTempl(ChatThreadList(current, user.ID))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			// Re-ensure membership so newly created public channels appear immediately.
			_ = h.store.EnsurePublicMembership(r.Context(), user.ID)
			next, err := h.store.ListThreadsForUser(r.Context(), user.ID)
			if err != nil {
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(ChatThreadList(next, user.ID))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// chatSend saves and broadcasts a message to a thread.
func (h *ChatHandler) chatSend(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var signals struct {
		ChatMsg  string `json:"chatmsg"`
		ThreadID string `json:"threadid"`
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
	threadID := signals.ThreadID
	if threadID == "" {
		threadID = "general"
	}

	msg := chat.Message{
		ID:        uuid.Must(uuid.NewV7()).String(),
		ThreadID:  threadID,
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
	h.publisher.Publish(r.Context(), "isp.chat.message."+threadID, events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "chat.message",
		AggregateID: msg.ID,
		OccurredAt:  msg.CreatedAt,
		Data:        data,
	})

	w.WriteHeader(http.StatusNoContent)
}

// createDirect finds or creates a 1:1 DM thread, then redirects to /chat.
func (h *ChatHandler) createDirect(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var signals struct {
		TargetUserID string `json:"targetuserid"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if signals.TargetUserID == "" || signals.TargetUserID == user.ID {
		http.Error(w, "invalid target user", http.StatusBadRequest)
		return
	}

	thread, err := h.store.FindDirectThread(r.Context(), user.ID, signals.TargetUserID)
	if errors.Is(err, chat.ErrNotFound) {
		thread = chat.Thread{
			ID:        uuid.Must(uuid.NewV7()).String(),
			Type:      "direct",
			CreatedBy: user.ID,
			CreatedAt: time.Now().UTC(),
		}
		if err := h.store.CreateThread(r.Context(), thread); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = h.store.AddMember(r.Context(), thread.ID, user.ID)
		_ = h.store.AddMember(r.Context(), thread.ID, signals.TargetUserID)
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.Redirect("/chat?thread=" + thread.ID)
}

// createChannel creates a new named channel.
func (h *ChatHandler) createChannel(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var signals struct {
		ChannelName string `json:"channelname"`
		IsPrivate   bool   `json:"isprivate"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(signals.ChannelName)
	if name == "" {
		http.Error(w, "channel name required", http.StatusBadRequest)
		return
	}

	thread := chat.Thread{
		ID:        uuid.Must(uuid.NewV7()).String(),
		Type:      "channel",
		Name:      "#" + strings.TrimPrefix(name, "#"),
		IsPrivate: signals.IsPrivate,
		CreatedBy: user.ID,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.CreateThread(r.Context(), thread); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.store.AddMember(r.Context(), thread.ID, user.ID)

	sse := datastar.NewSSE(w, r)
	sse.Redirect("/chat?thread=" + thread.ID)
}

// addMember adds a user to a channel.
func (h *ChatHandler) addMember(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := chi.URLParam(r, "threadID")
	member, _ := h.store.IsMember(r.Context(), threadID, user.ID)
	if !member {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var signals struct {
		UserID string `json:"userid"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.AddMember(r.Context(), threadID, signals.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// markRead marks a thread as read for the current user.
func (h *ChatHandler) markRead(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	threadID := chi.URLParam(r, "threadID")
	_ = h.store.MarkRead(r.Context(), threadID, user.ID)
	w.WriteHeader(http.StatusNoContent)
}

func scrollToBottom(sse *datastar.ServerSentEventGenerator) {
	sse.ExecuteScript(
		`(function(){var el=document.getElementById('chat-messages');if(el)el.scrollTop=el.scrollHeight})()`,
	)
}
