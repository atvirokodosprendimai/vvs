package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
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
	if err := h.store.EnsurePublicMembership(r.Context(), user.ID); err != nil {
		log.Printf("chatPage: EnsurePublicMembership: %v", err)
	}
	threadID := r.URL.Query().Get("thread")
	threads, err := h.store.ListThreadsForUser(r.Context(), user.ID)
	if err != nil {
		log.Printf("chatPage: ListThreadsForUser: %v", err)
	}
	ChatPage(threads, user.ID, threadID).Render(r.Context(), w)
}

// chatPageSSE is the single consolidated SSE for the full /chat page.
// It multiplexes the thread list (isp.chat.>) and messages (isp.chat.message.{threadID})
// over one connection. When the user selects a thread, the client reconnects with the
// updated $threadid signal and this handler restarts with the new thread.
func (h *ChatHandler) chatPageSSE(w http.ResponseWriter, r *http.Request) {
	user := authhttp.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.store.EnsurePublicMembership(r.Context(), user.ID); err != nil {
		log.Printf("chatPageSSE: EnsurePublicMembership: %v", err)
	}

	var signals struct {
		ThreadID string `json:"threadid"`
	}
	_ = datastar.ReadSignals(r, &signals)
	threadID := signals.ThreadID

	// Ensure membership + mark read for the selected thread.
	if threadID != "" {
		member, err := h.store.IsMember(r.Context(), threadID, user.ID)
		if err == nil && !member {
			if channels, err := h.store.ListPublicChannels(r.Context()); err == nil {
				for _, c := range channels {
					if c.ID == threadID {
						_ = h.store.AddMember(r.Context(), threadID, user.ID)
						break
					}
				}
			}
		}
		if err := h.store.MarkRead(r.Context(), threadID, user.ID); err == nil {
			h.publisher.Publish(r.Context(), events.ChatRead.Format(threadID), events.DomainEvent{
				ID:          uuid.Must(uuid.NewV7()).String(),
				Type:        "chat.read",
				AggregateID: threadID,
				OccurredAt:  time.Now().UTC(),
			})
		}
	}

	sse := datastar.NewSSE(w, r)

	// Subscribe to thread list events.
	chatCh, cancelChat := h.subscriber.ChanSubscription(events.ChatAll.String())
	defer cancelChat()

	// Subscribe to messages for the selected thread (nil channel = never fires).
	var msgCh <-chan events.DomainEvent
	cancelMsg := func() {}
	defer func() { cancelMsg() }()
	if threadID != "" {
		ch, cancel := h.subscriber.ChanSubscription(events.ChatMessage.Format(threadID))
		msgCh = ch
		cancelMsg = cancel
	}

	// Initial render: thread list.
	currentThreads, err := h.store.ListThreadsForUser(r.Context(), user.ID)
	if err != nil {
		log.Printf("chatPageSSE: ListThreadsForUser: %v", err)
	}
	sse.PatchElementTempl(ChatThreadList(currentThreads, user.ID))

	// Initial render: messages for selected thread.
	if threadID != "" {
		msgs, err := h.store.Recent(r.Context(), threadID, chatHistoryLimit)
		if err != nil {
			log.Printf("chatPageSSE: Recent: %v", err)
		}
		sse.PatchElementTempl(ChatMessages(msgs, user.ID))
		scrollToBottom(sse)
	}

	for {
		select {
		case event, ok := <-chatCh:
			if !ok {
				return
			}
			if event.Type == "chat.thread.created" {
				if err := h.store.EnsurePublicMembership(r.Context(), user.ID); err != nil {
					log.Printf("chatPageSSE: EnsurePublicMembership: %v", err)
				}
			}
			next, err := h.store.ListThreadsForUser(r.Context(), user.ID)
			if err != nil {
				log.Printf("chatPageSSE: ListThreadsForUser: %v", err)
				continue
			}
			if !reflect.DeepEqual(currentThreads, next) {
				sse.PatchElementTempl(ChatThreadList(next, user.ID))
				currentThreads = next
			}
		case event, ok := <-msgCh:
			if !ok {
				return
			}
			var msg chat.Message
			if err := json.Unmarshal(event.Data, &msg); err != nil {
				log.Printf("chatPageSSE: unmarshal message: %v", err)
				continue
			}
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
		log.Printf("chatSend: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
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
		log.Printf("chatSend: Save: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data, _ := json.Marshal(msg)
	h.publisher.Publish(r.Context(), events.ChatMessage.Format(threadID), events.DomainEvent{
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
		log.Printf("createDirect: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
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
			log.Printf("createDirect: CreateThread: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if err := h.store.AddMember(r.Context(), thread.ID, user.ID); err != nil {
			log.Printf("createDirect: AddMember (self): %v", err)
		}
		if err := h.store.AddMember(r.Context(), thread.ID, signals.TargetUserID); err != nil {
			log.Printf("createDirect: AddMember (target): %v", err)
		}
		h.publisher.Publish(r.Context(), events.ChatThreadCreated.String(), events.DomainEvent{
			ID:          uuid.Must(uuid.NewV7()).String(),
			Type:        "chat.thread.created",
			AggregateID: thread.ID,
			OccurredAt:  thread.CreatedAt,
		})
	} else if err != nil {
		log.Printf("createDirect: FindDirectThread: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
		log.Printf("createChannel: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
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
		log.Printf("createChannel: CreateThread: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.store.AddMember(r.Context(), thread.ID, user.ID); err != nil {
		log.Printf("createChannel: AddMember: %v", err)
	}

	h.publisher.Publish(r.Context(), events.ChatThreadCreated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "chat.thread.created",
		AggregateID: thread.ID,
		OccurredAt:  thread.CreatedAt,
	})

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
	member, err := h.store.IsMember(r.Context(), threadID, user.ID)
	if err != nil {
		log.Printf("addMember: IsMember: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !member {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var signals struct {
		UserID string `json:"userid"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("addMember: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.store.AddMember(r.Context(), threadID, signals.UserID); err != nil {
		log.Printf("addMember: AddMember: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
	if err := h.store.MarkRead(r.Context(), threadID, user.ID); err != nil {
		log.Printf("markRead: %v", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func scrollToBottom(sse *datastar.ServerSentEventGenerator) {
	sse.ExecuteScript(
		`(function(){var el=document.getElementById('chat-messages');if(el)el.scrollTop=el.scrollHeight})()`,
	)
}
