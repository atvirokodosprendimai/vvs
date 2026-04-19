package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/emailcrypto"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// ConfigureAccountCommand creates or updates an IMAP account.
// ID: empty = create new, non-empty = update existing.
type ConfigureAccountCommand struct {
	ID       string // empty = create
	Name     string
	Host     string
	Port     int
	Username string
	Password string // plaintext — will be encrypted
	TLS      string
	Folder   string
	SMTPHost string
	SMTPPort int
	SMTPTLS  string
}

type ConfigureAccountHandler struct {
	repo      domain.EmailAccountRepository
	publisher events.EventPublisher
	encKey    []byte // 32 bytes AES-256 key
}

func NewConfigureAccountHandler(repo domain.EmailAccountRepository, pub events.EventPublisher, encKey []byte) *ConfigureAccountHandler {
	return &ConfigureAccountHandler{repo: repo, publisher: pub, encKey: encKey}
}

func (h *ConfigureAccountHandler) Handle(ctx context.Context, cmd ConfigureAccountCommand) (*domain.EmailAccount, error) {
	enc, err := emailcrypto.EncryptPassword(h.encKey, []byte(cmd.Password))
	if err != nil {
		return nil, err
	}

	if cmd.ID == "" {
		// Create new.
		a, err := domain.NewEmailAccount(
			uuid.Must(uuid.NewV7()).String(),
			cmd.Name, cmd.Host, cmd.Port, cmd.Username, enc, cmd.TLS, cmd.Folder,
		)
		if err != nil {
			return nil, err
		}
		a.SMTPHost = cmd.SMTPHost
		a.SMTPPort = cmd.SMTPPort
		a.SMTPTLS = cmd.SMTPTLS
		if err := h.repo.Save(ctx, a); err != nil {
			return nil, err
		}
		h.publisher.Publish(ctx, events.EmailAccountConfigured.String(), events.DomainEvent{
			ID: uuid.Must(uuid.NewV7()).String(), Type: "email.account_configured", AggregateID: a.ID,
		})
		return a, nil
	}

	// Update existing.
	a, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	a.Name = cmd.Name
	a.Host = cmd.Host
	a.Port = cmd.Port
	a.Username = cmd.Username
	if cmd.Password != "" {
		a.PasswordEnc = enc
	}
	if cmd.TLS != "" {
		a.TLS = cmd.TLS
	}
	if cmd.Folder != "" {
		a.Folder = cmd.Folder
	}
	a.SMTPHost = cmd.SMTPHost
	a.SMTPPort = cmd.SMTPPort
	if cmd.SMTPTLS != "" {
		a.SMTPTLS = cmd.SMTPTLS
	}
	if err := h.repo.Save(ctx, a); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, events.EmailAccountConfigured.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.account_configured", AggregateID: a.ID,
	})
	return a, nil
}

// DecryptPassword decrypts an AES-256-GCM encrypted password.
// Kept for backward compatibility; delegates to emailcrypto.DecryptPassword.
func DecryptPassword(key, ciphertext []byte) ([]byte, error) {
	return emailcrypto.DecryptPassword(key, ciphertext)
}
