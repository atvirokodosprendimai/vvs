package app

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/shared/events"

	emailhttp "github.com/vvs/isp/internal/modules/email/adapters/http"
	imapAdapter "github.com/vvs/isp/internal/modules/email/adapters/imap"
	emailpersistence "github.com/vvs/isp/internal/modules/email/adapters/persistence"
	smtpAdapter "github.com/vvs/isp/internal/modules/email/adapters/smtp"
	emailcommands "github.com/vvs/isp/internal/modules/email/app/commands"
	emailqueries "github.com/vvs/isp/internal/modules/email/app/queries"
	emaildomain "github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/modules/email/worker"
)

type emailWired struct {
	accountRepo          *emailpersistence.GormEmailAccountRepository
	smtpSender           emaildomain.EmailSender
	listEmailForCustomer *emailqueries.ListThreadsForCustomerHandler
	worker               *worker.SyncWorker
	routes               *emailhttp.Handlers
}

func wireEmail(
	gdb *gormsqlite.DB,
	pub events.EventPublisher,
	sub events.EventSubscriber,
	cust *customerWired,
	cfg Config,
) *emailWired {
	emailAccountRepo    := emailpersistence.NewGormEmailAccountRepository(gdb)
	emailThreadRepo     := emailpersistence.NewGormEmailThreadRepository(gdb)
	emailMessageRepo    := emailpersistence.NewGormEmailMessageRepository(gdb)
	emailAttachmentRepo := emailpersistence.NewGormEmailAttachmentRepository(gdb)
	emailTagRepo        := emailpersistence.NewGormEmailTagRepository(gdb)
	emailFolderRepo     := emailpersistence.NewGormEmailFolderRepository(gdb)

	emailEncKey := []byte(cfg.EmailEncKey)

	configureAccountCmd := emailcommands.NewConfigureAccountHandler(emailAccountRepo, pub, emailEncKey)
	deleteAccountCmd    := emailcommands.NewDeleteAccountHandler(emailAccountRepo, pub)
	pauseAccountCmd     := emailcommands.NewPauseAccountHandler(emailAccountRepo, pub)
	resumeAccountCmd    := emailcommands.NewResumeAccountHandler(emailAccountRepo, pub)
	applyTagCmd         := emailcommands.NewApplyTagHandler(emailThreadRepo, emailTagRepo, pub)
	removeTagCmd        := emailcommands.NewRemoveTagHandler(emailTagRepo, pub)
	markReadCmd         := emailcommands.NewMarkReadHandler(emailTagRepo, pub)
	linkCustomerCmd     := emailcommands.NewLinkCustomerHandler(emailThreadRepo, pub)
	toggleStarCmd       := emailcommands.NewToggleStarHandler(emailTagRepo, pub)

	var smtpSender emaildomain.EmailSender
	if cfg.DemoMode {
		smtpSender = smtpAdapter.NewNoopSender()
	} else {
		smtpSender = smtpAdapter.NewSender(emailEncKey)
	}
	imapAppender := imapAdapter.NewAppender(emailEncKey)

	sendReplyCmd    := emailcommands.NewSendReplyHandler(emailThreadRepo, emailMessageRepo, emailAccountRepo, smtpSender, pub)
	composeEmailCmd := emailcommands.NewComposeEmailHandler(emailAccountRepo, emailThreadRepo, emailMessageRepo, smtpSender, imapAppender, pub)

	listEmailThreadsQuery     := emailqueries.NewListThreadsHandler(emailThreadRepo, emailTagRepo).WithFolderRepo(emailFolderRepo)
	getEmailThreadQuery       := emailqueries.NewGetThreadHandler(emailThreadRepo, emailMessageRepo, emailAttachmentRepo, emailTagRepo)
	listEmailForCustomerQuery := emailqueries.NewListThreadsForCustomerHandler(emailThreadRepo, emailTagRepo)
	listEmailAccountsQuery    := emailqueries.NewListAccountsHandler(emailAccountRepo)
	listFoldersQuery          := emailqueries.NewListFoldersHandler(emailFolderRepo)
	searchAttachmentsQuery    := emailqueries.NewSearchAttachmentsHandler(emailAttachmentRepo)

	emailRepos := imapAdapter.Repos{
		DB:          gdb,
		Accounts:    emailAccountRepo,
		Folders:     emailFolderRepo,
		Threads:     emailThreadRepo,
		Messages:    emailMessageRepo,
		Attachments: emailAttachmentRepo,
		Tags:        emailTagRepo,
		EncKey:      emailEncKey,
	}
	discoverFn := func(ctx context.Context, accountID string) ([]emailqueries.FolderReadModel, error) {
		acc, err := emailAccountRepo.FindByID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		newID := func() string { return uuid.Must(uuid.NewV7()).String() }
		if _, err := imapAdapter.DiscoverFolders(ctx, acc, emailRepos, newID); err != nil {
			return nil, err
		}
		return listFoldersQuery.Handle(ctx, accountID)
	}

	routes := emailhttp.NewHandlers(
		configureAccountCmd, deleteAccountCmd, pauseAccountCmd, resumeAccountCmd,
		applyTagCmd, removeTagCmd, markReadCmd, linkCustomerCmd, sendReplyCmd,
		listEmailThreadsQuery, getEmailThreadQuery, listEmailForCustomerQuery,
		listEmailAccountsQuery, listFoldersQuery, emailFolderRepo, discoverFn,
		emailAttachmentRepo,
		sub, pub,
	).
		WithPageSize(cfg.EmailPageSize).
		WithSearchAttachments(searchAttachmentsQuery).
		WithComposeCmd(composeEmailCmd).
		WithCustomerInfo(&emailCustomerInfoBridge{repo: cust.repo}).
		WithContactEmailLookup(&emailContactLookupBridge{db: gdb}).
		WithStarToggler(toggleStarCmd)

	if cust.routes != nil {
		cust.routes.WithEmailThreadsQuery(listEmailForCustomerQuery)
	}

	syncInterval := time.Duration(cfg.EmailSyncIntervalSecs) * time.Second
	emailWorker := worker.NewSyncWorker(emailRepos, pub, sub, syncInterval)
	emailWorker.Start()
	log.Printf("module wired: email")

	return &emailWired{
		accountRepo:          emailAccountRepo,
		smtpSender:           smtpSender,
		listEmailForCustomer: listEmailForCustomerQuery,
		worker:               emailWorker,
		routes:               routes,
	}
}
