package imap

import (
	"context"
	"fmt"
	"time"

	imaplib "github.com/emersion/go-imap/v2"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
)

// Appender implements domain.EmailFolderAppender via IMAP APPEND.
type Appender struct {
	encKey []byte
}

// NewAppender creates an Appender using the given AES encryption key.
func NewAppender(encKey []byte) *Appender {
	return &Appender{encKey: encKey}
}

// AppendToFolder connects to the account's IMAP server and appends raw to folder.
// The message is marked \Seen so it doesn't appear as unread in the Sent folder.
// Errors are returned but callers should treat this as best-effort.
func (a *Appender) AppendToFolder(_ context.Context, account *domain.EmailAccount, folder string, raw []byte) error {
	password, err := decryptPassword(a.encKey, account.PasswordEnc)
	if err != nil {
		return fmt.Errorf("imap append: decrypt password: %w", err)
	}

	c, err := dial(account)
	if err != nil {
		return fmt.Errorf("imap append: connect: %w", err)
	}
	defer c.Close()

	if err := c.Login(account.Username, string(password)).Wait(); err != nil {
		return fmt.Errorf("imap append: login: %w", err)
	}

	appendCmd := c.Append(folder, int64(len(raw)), &imaplib.AppendOptions{
		Flags: []imaplib.Flag{imaplib.FlagSeen},
		Time:  time.Now(),
	})
	if _, err := appendCmd.Write(raw); err != nil {
		return fmt.Errorf("imap append: write %q: %w", folder, err)
	}
	if err := appendCmd.Close(); err != nil {
		return fmt.Errorf("imap append: close %q: %w", folder, err)
	}
	if _, err := appendCmd.Wait(); err != nil {
		return fmt.Errorf("imap append: APPEND %q: %w", folder, err)
	}
	return nil
}
