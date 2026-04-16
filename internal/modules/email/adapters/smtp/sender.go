package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/email/emailcrypto"
	"github.com/vvs/isp/internal/modules/email/domain"
)

// Sender implements domain.EmailSender using stdlib net/smtp.
type Sender struct {
	encKey []byte
}

func NewSender(encKey []byte) *Sender {
	return &Sender{encKey: encKey}
}

// BuildMessage constructs a RFC 2822 message and returns its Message-ID and raw bytes.
// Exported so command handlers can reuse the same bytes for IMAP APPEND.
func BuildMessage(account *domain.EmailAccount, to, subject, body, inReplyTo, references string) (msgID string, raw []byte) {
	smtpHost := account.SMTPHost
	if smtpHost == "" {
		smtpHost = account.Host
	}
	msgID = fmt.Sprintf("<%s@%s>", uuid.Must(uuid.NewV7()).String(), smtpHost)
	var buf strings.Builder
	buf.WriteString("From: " + account.Username + "\r\n")
	buf.WriteString("To: " + to + "\r\n")
	buf.WriteString("Subject: " + subject + "\r\n")
	buf.WriteString("Message-ID: " + msgID + "\r\n")
	buf.WriteString("Date: " + time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000") + "\r\n")
	if inReplyTo != "" {
		buf.WriteString("In-Reply-To: " + inReplyTo + "\r\n")
	}
	if references != "" {
		buf.WriteString("References: " + references + "\r\n")
	}
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	return msgID, []byte(buf.String())
}

func (s *Sender) Send(_ context.Context, account *domain.EmailAccount, to, subject, body, inReplyTo, references string) error {
	password, err := emailcrypto.DecryptPassword(s.encKey, account.PasswordEnc)
	if err != nil {
		return fmt.Errorf("smtp: decrypt password: %w", err)
	}

	smtpHost := account.SMTPHost
	if smtpHost == "" {
		smtpHost = account.Host
	}
	smtpPort := account.SMTPPort
	if smtpPort == 0 {
		smtpPort = 587
	}
	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)

	_, msgBytes := BuildMessage(account, to, subject, body, inReplyTo, references)

	tlsMode := account.SMTPTLS
	if tlsMode == "" {
		tlsMode = domain.TLSStartTLS
	}

	client, err := s.dial(tlsMode, addr, smtpHost)
	if err != nil {
		return fmt.Errorf("smtp: connect: %w", err)
	}
	defer client.Close()

	auth := smtp.PlainAuth("", account.Username, string(password), smtpHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp: auth: %w", err)
	}
	return send(client, account.Username, to, msgBytes)
}

func (s *Sender) dial(tlsMode, addr, host string) (*smtp.Client, error) {
	switch tlsMode {
	case domain.TLSTLS:
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, host)
	default:
		client, err := smtp.Dial(addr)
		if err != nil {
			return nil, err
		}
		if tlsMode == domain.TLSStartTLS {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				client.Close()
				return nil, err
			}
		}
		return client, nil
	}
}

func send(client *smtp.Client, from, to string, msg []byte) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp: RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: close data: %w", err)
	}
	return client.Quit()
}
