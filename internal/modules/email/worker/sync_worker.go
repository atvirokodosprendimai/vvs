package worker

import (
	"context"
	"log/slog"
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
	slog.Info("email sync worker started", "interval", w.interval)
	go w.run()
}

// Stop signals the worker to exit.
func (w *SyncWorker) Stop() {
	close(w.stop)
}

func (w *SyncWorker) run() {
	// Subscribe to per-account manual sync trigger: isp.email.sync_requested.{accountID}
	var manualTrigger <-chan events.DomainEvent
	var cancelSub func()
	if w.sub != nil {
		manualTrigger, cancelSub = w.sub.ChanSubscription("isp.email.sync_requested.*")
		defer cancelSub()
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Sync once on startup.
	w.syncAll()

	for {
		select {
		case <-ticker.C:
			slog.Debug("email sync: scheduled tick")
			w.syncAll()
		case evt, ok := <-manualTrigger:
			if !ok {
				return
			}
			slog.Info("email sync: manual trigger", "account_id", evt.AggregateID)
			w.syncOne(evt.AggregateID)
		case <-w.stop:
			slog.Info("email sync worker stopped")
			return
		}
	}
}

func (w *SyncWorker) syncOne(accountID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	account, err := w.repos.Accounts.FindByID(ctx, accountID)
	if err != nil {
		slog.Error("email sync: find account", "account_id", accountID, "err", err)
		return
	}
	if account.Status == domain.AccountStatusPaused {
		slog.Debug("email sync: account paused, skipping", "account", account.Name)
		return
	}
	w.syncAccount(ctx, account)
}

func (w *SyncWorker) syncAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	accounts, err := w.repos.Accounts.ListActive(ctx)
	if err != nil {
		slog.Error("email sync: list active accounts", "err", err)
		return
	}

	slog.Info("email sync: starting", "accounts", len(accounts))
	for _, account := range accounts {
		w.syncAccount(ctx, account)
	}
	slog.Info("email sync: done")
}

func (w *SyncWorker) syncAccount(ctx context.Context, account *domain.EmailAccount) {
	slog.Info("email sync: syncing account", "account", account.Name, "host", account.Host, "last_uid", account.LastUID)
	err := imapAdapter.Fetch(ctx, account, w.repos, newID, w.pub)
	if err != nil {
		slog.Error("email sync: account failed", "account", account.Name, "err", err)
		account.SetError(err.Error())
		if saveErr := w.repos.Accounts.Save(ctx, account); saveErr != nil {
			slog.Error("email sync: save error state", "account", account.Name, "err", saveErr)
		}
		return
	}
	slog.Info("email sync: account done", "account", account.Name, "last_uid", account.LastUID)
}

func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// Repos re-exports the type alias for wiring convenience.
type Repos = imapAdapter.Repos

// GormEmailAccountRepository is re-exported for wiring convenience.
type GormEmailAccountRepository = persistence.GormEmailAccountRepository
