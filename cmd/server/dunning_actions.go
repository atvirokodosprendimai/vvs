package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	customerpersistence "github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	emailpersistence "github.com/vvs/isp/internal/modules/email/adapters/persistence"
	smtpAdapter "github.com/vvs/isp/internal/modules/email/adapters/smtp"
	invoicecommands "github.com/vvs/isp/internal/modules/invoice/app/commands"
	invoicepersistence "github.com/vvs/isp/internal/modules/invoice/adapters/persistence"
	portalpersistence "github.com/vvs/isp/internal/modules/portal/adapters/persistence"
	portaldomain "github.com/vvs/isp/internal/modules/portal/domain"
)

// RegisterDunningActions wires dunning dependencies and registers the
// "send-dunning-reminders" cron action. Call before RunDueJobs.
func RegisterDunningActions(gdb *gormsqlite.DB, emailEncKey []byte, baseURL string) {
	emailAccountRepo := emailpersistence.NewGormEmailAccountRepository(gdb)
	smtp := smtpAdapter.NewSender(emailEncKey)
	var portalGen invoicecommands.PortalLinkGenerator
	if baseURL != "" {
		portalGen = &dunningPortalBridge{
			repo:    portalpersistence.NewGormPortalTokenRepository(gdb),
			baseURL: baseURL,
		}
	}
	RegisterDunningActionsWithMailer(gdb, &dunningMailerBridge{accounts: emailAccountRepo, smtp: smtp}, portalGen)
}

// RegisterDunningActionsWithMailer is the testable variant — accepts a pre-built mailer.
func RegisterDunningActionsWithMailer(gdb *gormsqlite.DB, mailer invoicecommands.EmailSender, portalGen invoicecommands.PortalLinkGenerator) {
	invoiceRepo := invoicepersistence.NewInvoiceRepository(gdb)
	customerRepo := customerpersistence.NewGormCustomerRepository(gdb)
	handler := invoicecommands.NewSendDunningRemindersHandler(invoiceRepo, customerRepo, mailer)
	if portalGen != nil {
		handler.WithPortalAccess(portalGen)
	}

	RegisterAction("send-dunning-reminders", func(ctx context.Context) error {
		result, err := handler.Handle(ctx, invoicecommands.SendDunningRemindersCommand{})
		if err != nil {
			return err
		}
		if len(result.Sent) > 0 {
			log.Printf("dunning: sent reminders for %d invoice(s): %v", len(result.Sent), result.Sent)
		}
		for _, e := range result.Errors {
			log.Printf("dunning: %s", e)
		}
		return nil
	})
}

// dunningPortalBridge implements invoicecommands.PortalLinkGenerator using the portal token repo.
type dunningPortalBridge struct {
	repo    portaldomain.PortalTokenRepository
	baseURL string
}

func (b *dunningPortalBridge) GeneratePortalLink(ctx context.Context, customerID string, ttl time.Duration) (string, error) {
	tok, plain, err := portaldomain.NewPortalToken(customerID, ttl)
	if err != nil {
		return "", err
	}
	if err := b.repo.Save(ctx, tok); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/portal/auth?token=%s", b.baseURL, plain), nil
}

// dunningMailerBridge implements invoicecommands.EmailSender using the first active email account.
type dunningMailerBridge struct {
	accounts *emailpersistence.GormEmailAccountRepository
	smtp     *smtpAdapter.Sender
}

func (b *dunningMailerBridge) SendPlain(ctx context.Context, to, subject, body string) error {
	accounts, err := b.accounts.ListActive(ctx)
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("dunning: no active email account")
	}
	return b.smtp.Send(ctx, accounts[0], to, subject, body, "", "")
}
