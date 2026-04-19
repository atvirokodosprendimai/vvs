package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	customerpersistence "github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	emailpersistence "github.com/vvs/isp/internal/modules/email/adapters/persistence"
	smtpAdapter "github.com/vvs/isp/internal/modules/email/adapters/smtp"
	invoicecommands "github.com/vvs/isp/internal/modules/invoice/app/commands"
	invoicepersistence "github.com/vvs/isp/internal/modules/invoice/adapters/persistence"
)

// RegisterDunningActions wires dunning dependencies and registers the
// "send-dunning-reminders" cron action. Call before RunDueJobs.
func RegisterDunningActions(gdb *gormsqlite.DB, emailEncKey []byte) {
	invoiceRepo := invoicepersistence.NewInvoiceRepository(gdb)
	customerRepo := customerpersistence.NewGormCustomerRepository(gdb)
	emailAccountRepo := emailpersistence.NewGormEmailAccountRepository(gdb)
	smtp := smtpAdapter.NewSender(emailEncKey)

	mailer := &dunningMailerBridge{accounts: emailAccountRepo, smtp: smtp}
	handler := invoicecommands.NewSendDunningRemindersHandler(invoiceRepo, customerRepo, mailer)

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
