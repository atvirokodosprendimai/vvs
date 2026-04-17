package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

// emailComposer is the port for composing and sending new emails.
type emailComposer interface {
	Handle(ctx context.Context, cmd emailcommands.ComposeEmailCommand) error
}

// CustomerSearchResult is a minimal customer record for the picker dropdown.
type CustomerSearchResult struct {
	ID          string
	Code        string
	CompanyName string
}

// customerSearcher searches customers by text for the link-customer picker.
type customerSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]CustomerSearchResult, error)
}

// autoLinker runs the auto-link pass matching thread participants to customer emails.
type autoLinker interface {
	AutoLink(ctx context.Context) (int64, error)
}

// ticketOpener creates a ticket from an email thread.
type ticketOpener interface {
	Open(ctx context.Context, customerID, subject, body, priority string) (ticketID string, err error)
}

// folderToggler is the minimal interface for folder enable/disable.
type folderToggler interface {
	FindByID(ctx context.Context, id string) (*domain.EmailFolder, error)
	FindByAccountAndName(ctx context.Context, accountID, name string) (*domain.EmailFolder, error)
	Save(ctx context.Context, f *domain.EmailFolder) error
	ListForAccount(ctx context.Context, accountID string) ([]*domain.EmailFolder, error)
}

// Handlers wires together email HTTP handlers.
type Handlers struct {
	configureCmd  *emailcommands.ConfigureAccountHandler
	deleteCmd     *emailcommands.DeleteAccountHandler
	pauseCmd      *emailcommands.PauseAccountHandler
	resumeCmd     *emailcommands.ResumeAccountHandler
	applyTagCmd   *emailcommands.ApplyTagHandler
	removeTagCmd  *emailcommands.RemoveTagHandler
	markReadCmd   *emailcommands.MarkReadHandler
	linkCmd       *emailcommands.LinkCustomerHandler
	sendReplyCmd  *emailcommands.SendReplyHandler
	listThreads   *emailqueries.ListThreadsHandler
	getThread     *emailqueries.GetThreadHandler
	listForCust   *emailqueries.ListThreadsForCustomerHandler
	listAccounts  *emailqueries.ListAccountsHandler
	listFolders   *emailqueries.ListFoldersHandler
	folderRepo    folderToggler
	discoverFn    func(ctx context.Context, accountID string) ([]emailqueries.FolderReadModel, error)
	attachments        attachmentFinder
	searchAttachments  *emailqueries.SearchAttachmentsHandler
	composeCmd         emailComposer
	customerSearch     customerSearcher
	autoLinker         autoLinker
	ticketOpener       ticketOpener
	subscriber         events.EventSubscriber
	publisher          events.EventPublisher
	pageSize           int // threads per inbox page; 0 → DefaultPageSize
}

func NewHandlers(
	configureCmd *emailcommands.ConfigureAccountHandler,
	deleteCmd *emailcommands.DeleteAccountHandler,
	pauseCmd *emailcommands.PauseAccountHandler,
	resumeCmd *emailcommands.ResumeAccountHandler,
	applyTagCmd *emailcommands.ApplyTagHandler,
	removeTagCmd *emailcommands.RemoveTagHandler,
	markReadCmd *emailcommands.MarkReadHandler,
	linkCmd *emailcommands.LinkCustomerHandler,
	sendReplyCmd *emailcommands.SendReplyHandler,
	listThreads *emailqueries.ListThreadsHandler,
	getThread *emailqueries.GetThreadHandler,
	listForCust *emailqueries.ListThreadsForCustomerHandler,
	listAccounts *emailqueries.ListAccountsHandler,
	listFolders *emailqueries.ListFoldersHandler,
	folderRepo folderToggler,
	discoverFn func(ctx context.Context, accountID string) ([]emailqueries.FolderReadModel, error),
	attachments attachmentFinder,
	subscriber events.EventSubscriber,
	publisher events.EventPublisher,
) *Handlers {
	return &Handlers{
		configureCmd: configureCmd,
		deleteCmd:    deleteCmd,
		pauseCmd:     pauseCmd,
		resumeCmd:    resumeCmd,
		applyTagCmd:  applyTagCmd,
		removeTagCmd: removeTagCmd,
		markReadCmd:  markReadCmd,
		linkCmd:      linkCmd,
		sendReplyCmd: sendReplyCmd,
		listThreads:  listThreads,
		getThread:    getThread,
		listForCust:  listForCust,
		listAccounts: listAccounts,
		listFolders:  listFolders,
		folderRepo:   folderRepo,
		discoverFn:   discoverFn,
		attachments:  attachments,
		subscriber:   subscriber,
		publisher:    publisher,
	}
}

// WithPageSize sets the number of email threads per inbox page.
func (h *Handlers) WithPageSize(n int) *Handlers {
	h.pageSize = n
	return h
}

// WithSearchAttachments injects the attachment search query handler.
func (h *Handlers) WithSearchAttachments(q *emailqueries.SearchAttachmentsHandler) *Handlers {
	h.searchAttachments = q
	return h
}

// WithComposeCmd injects the compose email command handler.
func (h *Handlers) WithComposeCmd(c emailComposer) *Handlers {
	h.composeCmd = c
	return h
}

// WithCustomerSearch injects the customer search for the link-customer picker.
func (h *Handlers) WithCustomerSearch(cs customerSearcher) *Handlers {
	h.customerSearch = cs
	return h
}

// WithAutoLinker injects the auto-link capability.
func (h *Handlers) WithAutoLinker(al autoLinker) *Handlers {
	h.autoLinker = al
	return h
}

// WithTicketOpener injects the ticket creation capability.
func (h *Handlers) WithTicketOpener(to ticketOpener) *Handlers {
	h.ticketOpener = to
	return h
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/emails", h.emailPage)
	r.Get("/emails/threads/{threadID}", h.threadPage)
	r.Get("/emails/settings", h.settingsPage)
	r.Get("/sse/emails", h.listSSE)
	r.Get("/sse/emails/{threadID}", h.threadSSE)
	r.Get("/sse/email-accounts", h.accountListSSE)
	r.Post("/api/email-accounts", h.configureAccountSSE)
	r.Put("/api/email-accounts/{id}", h.updateAccountSSE)
	r.Delete("/api/email-accounts/{id}", h.deleteAccountSSE)
	r.Post("/api/email-accounts/{id}/pause", h.pauseAccountSSE)
	r.Post("/api/email-accounts/{id}/resume", h.resumeAccountSSE)
	r.Post("/api/email-sync/{accountID}", h.triggerSyncSSE)
	r.Post("/api/email-threads/{threadID}/tags", h.applyTagSSE)
	r.Delete("/api/email-threads/{threadID}/tags/{tagID}", h.removeTagSSE)
	r.Post("/api/email-threads/{threadID}/read", h.markReadSSE)
	r.Post("/api/email-threads/{threadID}/reply", h.replySSE)
	r.Post("/api/email-threads/{threadID}/link", h.linkCustomerSSE)
	r.Post("/api/email-threads/{threadID}/to-ticket", h.convertToTicketSSE)
	r.Get("/sse/email-customer-picker/{threadID}", h.customerPickerSSE)
	r.Get("/attachments", h.attachmentsPage)
	r.Get("/sse/attachments", h.attachmentSearchSSE)
	r.Get("/api/email-attachments/{id}", h.downloadAttachment)
	r.Post("/api/emails/compose", h.composeSSE)
	r.Post("/api/emails/auto-link", h.autoLinkSSE)
	r.Post("/api/email-accounts/{id}/discover-folders", h.discoverFoldersSSE)
	r.Put("/api/email-folders/{folderID}/toggle", h.toggleFolderSSE)
	r.Get("/sse/email-folders/{accountID}", h.folderListSSE)
}

// emailPage renders the full inbox page.
func (h *Handlers) emailPage(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.listAccounts.Handle(r.Context())
	if err != nil {
		log.Printf("email: emailPage: %v", err)
		accounts = []emailqueries.AccountReadModel{}
	}

	q := r.URL.Query()
	accountID := q.Get("account")
	folder := q.Get("folder")

	var folders []emailqueries.FolderReadModel
	if accountID != "" {
		folders, _ = h.listFolders.Handle(r.Context(), accountID)
	}

	if err := EmailPage(accounts, folders, accountID, folder).Render(r.Context(), w); err != nil {
		log.Printf("email: emailPage render: %v", err)
	}
}

// listSSE streams the thread list, refreshing on isp.email.* events.
func (h *Handlers) listSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		AccountID    string `json:"emailAccountID"`
		TagFilter    string `json:"emailTagFilter"`
		Search       string `json:"emailSearch"`
		FolderFilter string `json:"emailFolder"`
		Page         int    `json:"emailPage"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("email: listSSE ReadSignals: %v", err)
	}

	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(events.EmailAll.String())
	defer cancel()

	q := emailqueries.ListThreadsQuery{
		AccountID:    signals.AccountID,
		TagFilter:    signals.TagFilter,
		Search:       signals.Search,
		FolderFilter: signals.FolderFilter,
		Page:         signals.Page,
		PageSize:     h.pageSize,
	}

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

// threadPage renders the full thread detail page.
func (h *Handlers) threadPage(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	thread, err := h.getThread.Handle(r.Context(), threadID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Mark as read on open.
	_ = h.markReadCmd.Handle(r.Context(), emailcommands.MarkReadCommand{ThreadID: threadID})
	if err := EmailThreadPage(*thread).Render(r.Context(), w); err != nil {
		log.Printf("email: threadPage render: %v", err)
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
	ch, cancel := h.subscriber.ChanSubscription(events.EmailAll.String())
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

// replySSE sends a plain-text reply and refreshes the thread detail.
func (h *Handlers) replySSE(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	var signals struct {
		Body string `json:"emailReplyBody"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("email: replySSE ReadSignals: %v", err)
	}
	sse := datastar.NewSSE(w, r)
	if err := h.sendReplyCmd.Handle(r.Context(), emailcommands.SendReplyCommand{
		ThreadID: threadID, Body: signals.Body,
	}); err != nil {
		slog.Error("email: replySSE", "err", err)
		sse.PatchSignals([]byte(`{"emailReplyError":"` + smtpErrMsg(err) + `"}`))
		return
	}
	thread, err := h.getThread.Handle(r.Context(), threadID)
	if err == nil {
		sse.PatchElementTempl(ThreadDetail(*thread))
	}
	sse.PatchSignals([]byte(`{"emailReplyBody":"","emailReplyError":""}`))
}

// composeSSE sends a new email (compose) from form signals.
func (h *Handlers) composeSSE(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account")
	var signals struct {
		To      string `json:"composeTo"`
		Subject string `json:"composeSubject"`
		Body    string `json:"composeBody"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("email: composeSSE ReadSignals: %v", err)
	}
	sse := datastar.NewSSE(w, r)
	if h.composeCmd == nil {
		sse.PatchSignals([]byte(`{"composeError":"compose not configured"}`))
		return
	}
	if err := h.composeCmd.Handle(r.Context(), emailcommands.ComposeEmailCommand{
		AccountID: accountID,
		To:        signals.To,
		Subject:   signals.Subject,
		Body:      signals.Body,
	}); err != nil {
		slog.Error("email: composeSSE", "err", err)
		sse.PatchSignals([]byte(`{"composeError":"` + smtpErrMsg(err) + `"}`))
		return
	}
	sse.PatchSignals([]byte(`{"composeTo":"","composeSubject":"","composeBody":"","composeError":"","composeOpen":false}`))
}

// configureAccountSSE creates a new IMAP account from form signals.
func (h *Handlers) configureAccountSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Name     string `json:"emailName"`
		Host     string `json:"emailHost"`
		Port     string `json:"emailPort"`
		Username string `json:"emailUser"`
		Password string `json:"emailPass"`
		TLS      string `json:"emailTLS"`
		Folder   string `json:"emailFolder"`
		SMTPHost string `json:"emailSMTPHost"`
		SMTPPort string `json:"emailSMTPPort"`
		SMTPTLS  string `json:"emailSMTPTLS"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("email: configureAccountSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	port, _ := strconv.Atoi(signals.Port)
	if port == 0 {
		port = 993
	}
	smtpPort, _ := strconv.Atoi(signals.SMTPPort)

	sse := datastar.NewSSE(w, r)
	_, err := h.configureCmd.Handle(r.Context(), emailcommands.ConfigureAccountCommand{
		Name:     signals.Name,
		Host:     signals.Host,
		Port:     port,
		Username: signals.Username,
		Password: signals.Password,
		TLS:      signals.TLS,
		Folder:   signals.Folder,
		SMTPHost: signals.SMTPHost,
		SMTPPort: smtpPort,
		SMTPTLS:  signals.SMTPTLS,
	})
	if err != nil {
		log.Printf("email: configureAccountSSE: %v", err)
		sse.PatchSignals([]byte(`{"emailError":"` + err.Error() + `"}`))
		return
	}
	sse.PatchSignals([]byte(`{"_showAdd":false}`))
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

	sse := datastar.NewSSE(w, r)

	// Look up customer name for display
	label := ""
	if signals.CustomerID != "" && h.customerSearch != nil {
		results, err := h.customerSearch.Search(r.Context(), signals.CustomerID, 1)
		if err == nil && len(results) > 0 {
			label = results[0].Code + " — " + results[0].CompanyName
		}
	}
	sse.PatchElementTempl(customerLinkBadge(threadID, signals.CustomerID, label))
}

func (h *Handlers) customerPickerSSE(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	if h.customerSearch == nil {
		http.Error(w, "customer search not configured", http.StatusNotImplemented)
		return
	}

	var signals struct {
		LinkSearch string `json:"linkSearch"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	search := strings.TrimSpace(signals.LinkSearch)
	if search == "" {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(customerPickerResults(threadID, nil))
		return
	}

	results, err := h.customerSearch.Search(r.Context(), search, 8)
	if err != nil {
		log.Printf("email: customerPickerSSE: %v", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(customerPickerResults(threadID, results))
}

func (h *Handlers) convertToTicketSSE(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "threadID")
	if h.ticketOpener == nil {
		http.Error(w, "ticket creation not configured", http.StatusNotImplemented)
		return
	}

	thread, err := h.getThread.Handle(r.Context(), threadID)
	if err != nil {
		log.Printf("email: convertToTicket getThread: %v", err)
		http.Error(w, "thread not found", http.StatusNotFound)
		return
	}

	if thread.CustomerID == "" {
		sse := datastar.NewSSE(w, r)
		signalData, _ := json.Marshal(map[string]any{
			"toastMessage": "Link a customer first before converting to ticket",
			"toastType":    "warning",
			"toastVisible": true,
		})
		sse.PatchSignals(signalData)
		return
	}

	// Build ticket body from thread messages (newest first, max 5)
	var body strings.Builder
	count := len(thread.Messages)
	if count > 5 {
		count = 5
	}
	for i := 0; i < count; i++ {
		m := thread.Messages[i]
		body.WriteString(fmt.Sprintf("From: %s\nDate: %s\n\n%s\n\n---\n\n",
			m.FromAddr, m.ReceivedAt.Format("2006-01-02 15:04"), m.TextBody))
	}

	ticketID, err := h.ticketOpener.Open(r.Context(), thread.CustomerID, thread.Subject, body.String(), "normal")
	if err != nil {
		log.Printf("email: convertToTicket open: %v", err)
		sse := datastar.NewSSE(w, r)
		signalData, _ := json.Marshal(map[string]any{
			"toastMessage": "Failed to create ticket",
			"toastType":    "error",
			"toastVisible": true,
		})
		sse.PatchSignals(signalData)
		return
	}

	_ = ticketID
	sse := datastar.NewSSE(w, r)
	signalData, _ := json.Marshal(map[string]any{
		"toastMessage": "Ticket created from email thread",
		"toastType":    "info",
		"toastVisible": true,
	})
	sse.PatchSignals(signalData)
}

func (h *Handlers) autoLinkSSE(w http.ResponseWriter, r *http.Request) {
	if h.autoLinker == nil {
		http.Error(w, "auto-link not configured", http.StatusNotImplemented)
		return
	}

	linked, err := h.autoLinker.AutoLink(r.Context())
	if err != nil {
		log.Printf("email: autoLinkSSE: %v", err)
		http.Error(w, "auto-link failed", http.StatusInternalServerError)
		return
	}

	if linked > 0 {
		h.publisher.Publish(r.Context(), events.EmailCustomerLinked.String(), events.DomainEvent{
			Type:       "email.customers_auto_linked",
			OccurredAt: time.Now().UTC(),
		})
	}

	sse := datastar.NewSSE(w, r)
	msg := fmt.Sprintf("Linked %d threads to customers", linked)
	if linked == 0 {
		msg = "No new threads to link"
	}
	signalData, _ := json.Marshal(map[string]any{
		"toastMessage": msg,
		"toastType":    "info",
		"toastVisible": true,
	})
	sse.PatchSignals(signalData)
}

// settingsPage renders the IMAP account settings page.
func (h *Handlers) settingsPage(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.listAccounts.Handle(r.Context())
	if err != nil {
		log.Printf("email: settingsPage: %v", err)
		accounts = []emailqueries.AccountReadModel{}
	}
	if err := EmailSettingsPage(accounts).Render(r.Context(), w); err != nil {
		log.Printf("email: settingsPage render: %v", err)
	}
}

// accountListSSE streams the account list, refreshing on isp.email.* events.
func (h *Handlers) accountListSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(events.EmailAll.String())
	defer cancel()

	current, err := h.listAccounts.Handle(r.Context())
	if err != nil {
		log.Printf("email: accountListSSE: %v", err)
		return
	}
	sse.PatchElementTempl(EmailAccountList(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listAccounts.Handle(r.Context())
			if err != nil {
				log.Printf("email: accountListSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(EmailAccountList(next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// updateAccountSSE updates an existing IMAP account.
func (h *Handlers) updateAccountSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var signals struct {
		Name     string `json:"emailName"`
		Host     string `json:"emailHost"`
		Port     string `json:"emailPort"`
		Username string `json:"emailUser"`
		Password string `json:"emailPass"`
		TLS      string `json:"emailTLS"`
		Folder   string `json:"emailFolder"`
		SMTPHost string `json:"emailSMTPHost"`
		SMTPPort string `json:"emailSMTPPort"`
		SMTPTLS  string `json:"emailSMTPTLS"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("email: updateAccountSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	port, _ := strconv.Atoi(signals.Port)
	if port == 0 {
		port = 993
	}
	smtpPort, _ := strconv.Atoi(signals.SMTPPort)
	sse := datastar.NewSSE(w, r)
	_, err := h.configureCmd.Handle(r.Context(), emailcommands.ConfigureAccountCommand{
		ID:       id,
		Name:     signals.Name,
		Host:     signals.Host,
		Port:     port,
		Username: signals.Username,
		Password: signals.Password,
		TLS:      signals.TLS,
		Folder:   signals.Folder,
		SMTPHost: signals.SMTPHost,
		SMTPPort: smtpPort,
		SMTPTLS:  signals.SMTPTLS,
	})
	if err != nil {
		log.Printf("email: updateAccountSSE: %v", err)
		sse.PatchSignals([]byte(`{"emailError":"` + err.Error() + `"}`))
		return
	}
	sse.PatchSignals([]byte(`{"emailSettingsEdit":""}`))
}

// deleteAccountSSE deletes an IMAP account.
func (h *Handlers) deleteAccountSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.deleteCmd.Handle(r.Context(), id); err != nil {
		log.Printf("email: deleteAccountSSE: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pauseAccountSSE pauses an IMAP account.
func (h *Handlers) pauseAccountSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.pauseCmd.Handle(r.Context(), id); err != nil {
		log.Printf("email: pauseAccountSSE: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resumeAccountSSE resumes a paused IMAP account.
func (h *Handlers) resumeAccountSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.resumeCmd.Handle(r.Context(), id); err != nil {
		log.Printf("email: resumeAccountSSE: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// triggerSyncSSE publishes a manual sync request for a specific account.
func (h *Handlers) triggerSyncSSE(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	h.publisher.Publish(r.Context(), events.EmailSyncRequested.Format(accountID), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.sync_requested", AggregateID: accountID,
	})
	w.WriteHeader(http.StatusNoContent)
}

// discoverFoldersSSE runs IMAP LIST, upserts folders, and patches the folder list.
func (h *Handlers) discoverFoldersSSE(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	folders, err := h.discoverFn(r.Context(), accountID)
	if err != nil {
		log.Printf("email: discoverFoldersSSE: %v", err)
		sse.PatchSignals([]byte(`{"emailError":"folder discovery failed"}`))
		return
	}
	sse.PatchElementTempl(EmailFolderList(accountID, folders))
}

// toggleFolderSSE flips the Enabled flag on a folder and patches the folder list.
func (h *Handlers) toggleFolderSSE(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "folderID")
	sse := datastar.NewSSE(w, r)
	f, err := h.folderRepo.FindByID(r.Context(), folderID)
	if err != nil {
		log.Printf("email: toggleFolderSSE: %v", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f.Enabled = !f.Enabled
	if err := h.folderRepo.Save(r.Context(), f); err != nil {
		log.Printf("email: toggleFolderSSE save: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	folders, err := h.listFolders.Handle(r.Context(), f.AccountID)
	if err != nil {
		log.Printf("email: toggleFolderSSE list: %v", err)
		return
	}
	sse.PatchElementTempl(EmailFolderList(f.AccountID, folders))
}

// folderListSSE streams folder list for an account, refreshing on isp.email.* events.
func (h *Handlers) folderListSSE(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(events.EmailAll.String())
	defer cancel()

	folders, err := h.listFolders.Handle(r.Context(), accountID)
	if err != nil {
		log.Printf("email: folderListSSE: %v", err)
		return
	}
	sse.PatchElementTempl(EmailFolderList(accountID, folders))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listFolders.Handle(r.Context(), accountID)
			if err != nil {
				log.Printf("email: folderListSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(folders, next) {
				sse.PatchElementTempl(EmailFolderList(accountID, next))
				folders = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// smtpErrMsg maps a send error to a safe user-facing message.
func smtpErrMsg(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "auth"):
		return "SMTP authentication failed — check password in settings"
	case strings.Contains(s, "connect") || strings.Contains(s, "dial"):
		return "Cannot connect to SMTP server — check host and port in settings"
	case strings.Contains(s, "RCPT") || strings.Contains(s, "recipient"):
		return "Invalid recipient address"
	case strings.Contains(s, "empty"):
		return "Reply body is empty"
	case strings.Contains(s, "decrypt"):
		return "Cannot decrypt SMTP password — re-enter password in settings"
	default:
		return "Send failed — check server logs for details"
	}
}

// attachmentsPage renders the attachment search page.
func (h *Handlers) attachmentsPage(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.listAccounts.Handle(r.Context())
	q := r.URL.Query()
	accountID := q.Get("account")
	query := q.Get("q")
	AttachmentsPage(accounts, accountID, query).Render(r.Context(), w)
}

// attachmentSearchSSE runs an attachment filename search and patches results.
func (h *Handlers) attachmentSearchSSE(w http.ResponseWriter, r *http.Request) {
	if h.searchAttachments == nil {
		return
	}
	accountID := r.URL.Query().Get("account") // baked into the @get URL
	var signals struct {
		Q string `json:"q"`
	}
	_ = datastar.ReadSignals(r, &signals) // $q signal sent by Datastar
	sse := datastar.NewSSE(w, r)
	results, err := h.searchAttachments.Handle(r.Context(), emailqueries.SearchAttachmentsQuery{
		AccountID: accountID, Query: signals.Q,
	})
	if err != nil {
		slog.Error("email: attachmentSearchSSE", "err", err)
		sse.PatchElementTempl(AttachmentResults(nil, accountID))
		return
	}
	sse.PatchElementTempl(AttachmentResults(results, accountID))
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
