package commands

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"
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
	enc, err := encryptAES(h.encKey, []byte(cmd.Password))
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
		if err := h.repo.Save(ctx, a); err != nil {
			return nil, err
		}
		h.publisher.Publish(ctx, "isp.email.account_configured", events.DomainEvent{
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
	if err := h.repo.Save(ctx, a); err != nil {
		return nil, err
	}
	h.publisher.Publish(ctx, "isp.email.account_configured", events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.account_configured", AggregateID: a.ID,
	})
	return a, nil
}

// DecryptPassword decrypts an AES-256-GCM encrypted password.
func DecryptPassword(key, ciphertext []byte) ([]byte, error) {
	return decryptAES(key, ciphertext)
}

func encryptAES(key, plaintext []byte) ([]byte, error) {
	if len(key) == 0 {
		return plaintext, nil // no key configured — store as-is (dev mode)
	}
	if len(key) != 32 {
		return nil, errors.New("email: AES key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptAES(key, ciphertext []byte) ([]byte, error) {
	if len(key) == 0 {
		return ciphertext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("email: ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
