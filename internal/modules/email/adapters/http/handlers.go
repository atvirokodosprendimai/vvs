package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"reflect"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	emailcommands "github.com/vvs/isp/internal/modules/email/app/commands"
	emailqueries "github.com/vvs/isp/internal/modules/email/app/queries"
	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// attachmentFinder fetches a single attachment by ID.
type attachmentFinder interface {
	FindByID(ctx context.Context, id string) (*domain.EmailAttachment, error)
}

// Handlers wires together email HTTP handlers.
type Handlers struct {
	configureCmd  *emailcommands.ConfigureAccountHandler
	applyTagCmd   *emailcommands.ApplyTagHandler
	removeTagCmd  *emailcommands.RemoveTagHandler
	markReadCmd   *emailcommands.MarkReadHandler
	linkCmd       *emailcommands.LinkCustomerHandler
	listThreads   *emailqueries.ListThreadsHandler
	getThread     *emailqueries.GetThreadHandler
	listForCust   *emailqueries.ListThreadsForCustomerHandler
	listAccounts  *emailqueries.ListAccountsHandler
	attachments   attachmentFinder
	subscriber    events.EventSubscriber
	publisher     events.EventPublisher
}

func NewHandlers(
	configureCmd *emailcommands.ConfigureAccountHandler,
	applyTagCmd *emailcommands.ApplyTagHandler,
	removeTagCmd *emailcommands.RemoveTagHandler,
	markReadCmd *emailcommands.MarkReadHandler,
	linkCmd *emailcommands.LinkCustomerHandler,
	listThreads *emailqueries.ListThreadsHandler,
	getThread *emailqueries.GetThreadHandler,
	listForCust *emailqueries.ListThreadsForCustomerHandler,
	listAccounts *emailqueries.ListAccountsHandler,
	attachments attachmentFinder,
	subscriber events.EventSubscriber,
	publisher events.EventPublisher,
) *Handlers {
	return &Handlers{
		configureCmd: configureCmd,
		applyTagCmd:  applyTagCmd,
		removeTagCmd: removeTagCmd,
		markReadCmd:  markReadCmd,
		linkCmd:      linkCmd,
		listThreads:  listThreads,
		getThread:    getThread,
		listForCust:  listForCust,
		listAccounts: listAccounts,
		attachments:  attachments,
		subscriber:   subscriber,
		publisher:    publisher,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/emails", h.emailPage)
	r.Get("/sse/emails", h.listSSE)
	r.Get("/sse/emails/{threadID}", h.threadSSE)
	r.Post("/api/email-accounts", h.configureAccountSSE)
	r.Post("/api/email-threads/{threadID}/tags", h.applyTagSSE)
	r.Delete("/api/email-threads/{threadID}/tags/{tagID}", h.removeTagSSE)
	r.Post("/api/email-threads/{threadID}/read", h.markReadSSE)
	r.Post("/api/email-threads/{threadID}/link", h.linkCustomerSSE)
	r.Get("/api/email-attachments/{id}", h.downloadAttachment)
}

// emailPage renders the full inbox page.
func (h *Handlers) emailPage(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.listAccounts.Handle(r.Context())
	if err != nil {
		log.Printf("email: emailPage: %v", err)
		accounts = []emailqueries.AccountReadModel{}
	}
	component := EmailPage(accounts, "")
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("email: emailPage render: %v", err)
	}
}

// listSSE streams the thread list, refreshing on isp.email.* events.
func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		AccountID string `json:"_emailAccountID"`
		TagFilter string `json:"_emailTagFilter"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("email: listSSE ReadSignals: %v", err)
	}

	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription("isp.email.*")
	defer cancel()

	q := emailqueries.ListThreadsQuery{AccountID: signals.AccountID, TagFilter: signals.TagFilter}

	current, err := h.listThreads.Handle(r.Context(), q)
	if err != nil {
		log.Printf("email: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(ThreadList(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listThreads.Handle(r.Context(), q)
			if err != nil {
				log.Printf("email: listSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(ThreadList(next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// threadSSE streams a thread detail view.
func (h *Handlers) threadSSE(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	if threadID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription("isp.email.*")
	defer cancel()

	current, err := h.getThread.Handle(r.Context(), threadID)
	if err != nil {
		log.Printf("email: threadSSE: %v", err)
		return
	}
	sse.PatchElementTempl(ThreadDetail(*current))

	// Auto mark read on open.
	_ = h.markReadCmd.Handle(r.Context(), emailcommands.MarkReadCommand{ThreadID: threadID})

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.getThread.Handle(r.Context(), threadID)
			if err != nil {
				log.Printf("email: threadSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(ThreadDetail(*next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// configureAccountSSE creates a new IMAP account from form signals.
func (h *Handlers) configureAccountSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Name     string `json:"emailName"`
		Host     string `json:"emailHost"`
		Port     int    `json:"emailPort"`
		Username string `json:"emailUser"`
		Password string `json:"emailPass"`
		TLS      string `json:"emailTLS"`
		Folder   string `json:"emailFolder"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("email: configureAccountSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if signals.Port == 0 {
		signals.Port = 993
	}

	sse := datastar.NewSSE(w, r)
	_, err := h.configureCmd.Handle(r.Context(), emailcommands.ConfigureAccountCommand{
		Name:     signals.Name,
		Host:     signals.Host,
		Port:     signals.Port,
		Username: signals.Username,
		Password: signals.Password,
		TLS:      signals.TLS,
		Folder:   signals.Folder,
	})
	if err != nil {
		log.Printf("email: configureAccountSSE: %v", err)
		sse.PatchSignals([]byte(`{"emailError":"` + err.Error() + `"}`))
		return
	}
	// Close form by setting account ID to a non-empty sentinel.
	sse.PatchSignals([]byte(`{"_emailAccountID":"new"}`))
}

func (h *Handlers) applyTagSSE(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	var signals struct {
		TagID string `json:"tagID"`
	}
	datastar.ReadSignals(r, &signals)

	if err := h.applyTagCmd.Handle(r.Context(), emailcommands.ApplyTagCommand{
		ThreadID: threadID, TagID: signals.TagID,
	}); err != nil {
		log.Printf("email: applyTagSSE: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) removeTagSSE(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	tagID := chi.URLParam(r, "tagID")
	if err := h.removeTagCmd.Handle(r.Context(), emailcommands.RemoveTagCommand{
		ThreadID: threadID, TagID: tagID,
	}); err != nil {
		log.Printf("email: removeTagSSE: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) markReadSSE(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	if err := h.markReadCmd.Handle(r.Context(), emailcommands.MarkReadCommand{ThreadID: threadID}); err != nil {
		log.Printf("email: markReadSSE: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) linkCustomerSSE(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	var signals struct {
		CustomerID string `json:"customerID"`
	}
	datastar.ReadSignals(r, &signals)

	if err := h.linkCmd.Handle(r.Context(), emailcommands.LinkCustomerCommand{
		ThreadID: threadID, CustomerID: signals.CustomerID,
	}); err != nil {
		log.Printf("email: linkCustomerSSE: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// downloadAttachment streams an attachment file.
func (h *Handlers) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	att, err := h.attachments.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+att.Filename+"\"")
	w.Header().Set("Content-Type", att.MIMEType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", att.Size))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(att.Data)
}
