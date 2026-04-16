package worker

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	imapAdapter "github.com/vvs/isp/internal/modules/email/adapters/imap"
	"github.com/vvs/isp/internal/modules/email/adapters/persistence"
	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"
)

const defaultInterval = 5 * time.Minute

// SyncWorker polls active IMAP accounts and fetches new messages.
type SyncWorker struct {
	repos    imapAdapter.Repos
	pub      events.EventPublisher
	sub      events.EventSubscriber
	interval time.Duration
	stop     chan struct{}
}

func NewSyncWorker(
	repos imapAdapter.Repos,
	pub events.EventPublisher,
	sub events.EventSubscriber,
	interval time.Duration,
) *SyncWorker {
	if interval <= 0 {
		interval = defaultInterval
	}
	return &SyncWorker{
		repos:    repos,
		pub:      pub,
		sub:      sub,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start launches the background sync loop. Call Stop to shut down.
func (w *SyncWorker) Start() {
	go w.run()
}

// Stop signals the worker to exit.
func (w *SyncWorker) Stop() {
	close(w.stop)
}

func (w *SyncWorker) run() {
	// Subscribe to manual sync trigger events.
	var manualTrigger <-chan events.DomainEvent
	var cancelSub func()
	if w.sub != nil {
		manualTrigger, cancelSub = w.sub.ChanSubscription("isp.email.sync_requested")
		defer cancelSub()
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Sync once on startup.
	w.syncAll()

	for {
		select {
		case <-ticker.C:
			w.syncAll()
		case <-manualTrigger:
			w.syncAll()
		case <-w.stop:
			return
		}
	}
}

func (w *SyncWorker) syncAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	accounts, err := w.repos.Accounts.ListActive(ctx)
	if err != nil {
		log.Printf("email sync: list active accounts: %v", err)
		return
	}

	for _, account := range accounts {
		w.syncAccount(ctx, account)
	}
}

func (w *SyncWorker) syncAccount(ctx context.Context, account *domain.EmailAccount) {
	// Decrypt password before passing to fetcher.
	// (password stored encrypted; pass as-is here since fetcher uses it as []byte)
	err := imapAdapter.Fetch(ctx, account, w.repos, newID, w.pub)
	if err != nil {
		log.Printf("email sync: account %s (%s): %v", account.Name, account.ID, err)
		account.SetError(err.Error())
		if saveErr := w.repos.Accounts.Save(ctx, account); saveErr != nil {
			log.Printf("email sync: save account error state: %v", saveErr)
		}
		return
	}
}

func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// Repos re-exports the type alias for wiring convenience.
type Repos = imapAdapter.Repos

// GormEmailAccountRepository is re-exported for wiring convenience.
type GormEmailAccountRepository = persistence.GormEmailAccountRepository
