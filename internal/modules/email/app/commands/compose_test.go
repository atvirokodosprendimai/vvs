package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/vvs/isp/internal/modules/email/app/commands"
	"github.com/vvs/isp/internal/modules/email/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// ── stubs ────────────────────────────────────────────────────────────────────

type stubSender struct{ calls int }

func (s *stubSender) Send(_ context.Context, _ *domain.EmailAccount, _, _, _, _, _ string) error {
	s.calls++
	return nil
}

type stubAppender struct{ calls int; lastFolder string }

func (a *stubAppender) AppendToFolder(_ context.Context, _ *domain.EmailAccount, folder string, _ []byte) error {
	a.calls++
	a.lastFolder = folder
	return nil
}

type stubThreadRepo struct{ saved []*domain.EmailThread }

func (r *stubThreadRepo) Save(_ context.Context, t *domain.EmailThread) error {
	for i, existing := range r.saved {
		if existing.ID == t.ID {
			r.saved[i] = t
			return nil
		}
	}
	r.saved = append(r.saved, t)
	return nil
}
func (r *stubThreadRepo) FindByID(_ context.Context, id string) (*domain.EmailThread, error) {
	for _, t := range r.saved {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, domain.ErrThreadNotFound
}
func (r *stubThreadRepo) FindByMessageID(_ context.Context, _, _ string) (*domain.EmailThread, error) {
	return nil, domain.ErrThreadNotFound
}
func (r *stubThreadRepo) FindBySubject(_ context.Context, _, _ string) (*domain.EmailThread, error) {
	return nil, domain.ErrThreadNotFound
}
func (r *stubThreadRepo) ListAll(_ context.Context) ([]*domain.EmailThread, error) { return nil, nil }
func (r *stubThreadRepo) ListForAccount(_ context.Context, _ string) ([]*domain.EmailThread, error) {
	return nil, nil
}
func (r *stubThreadRepo) ListForCustomer(_ context.Context, _ string) ([]*domain.EmailThread, error) {
	return nil, nil
}

type stubMsgRepo struct{ saved []*domain.EmailMessage }

func (r *stubMsgRepo) Save(_ context.Context, m *domain.EmailMessage) error {
	r.saved = append(r.saved, m)
	return nil
}
func (r *stubMsgRepo) FindByUID(_ context.Context, _, _ string, _ uint32) (*domain.EmailMessage, error) {
	return nil, domain.ErrMessageNotFound
}
func (r *stubMsgRepo) FindByMessageID(_ context.Context, _, _ string) (*domain.EmailMessage, error) {
	return nil, domain.ErrMessageNotFound
}
func (r *stubMsgRepo) ListForThread(_ context.Context, _ string) ([]*domain.EmailMessage, error) {
	return nil, nil
}

type stubAccountRepo struct{ account *domain.EmailAccount }

func (r *stubAccountRepo) FindByID(_ context.Context, _ string) (*domain.EmailAccount, error) {
	if r.account != nil {
		return r.account, nil
	}
	return nil, domain.ErrAccountNotFound
}
func (r *stubAccountRepo) Save(_ context.Context, _ *domain.EmailAccount) error    { return nil }
func (r *stubAccountRepo) ListActive(_ context.Context) ([]*domain.EmailAccount, error) {
	return nil, nil
}
func (r *stubAccountRepo) List(_ context.Context) ([]*domain.EmailAccount, error) { return nil, nil }
func (r *stubAccountRepo) Delete(_ context.Context, _ string) error               { return nil }

type stubPublisher struct{}

func (p *stubPublisher) Publish(_ context.Context, _ string, _ events.DomainEvent) error {
	return nil
}

func testAccount() *domain.EmailAccount {
	return &domain.EmailAccount{
		ID:          "acc1",
		Username:    "test@example.com",
		Host:        "imap.example.com",
		Port:        993,
		TLS:         domain.TLSTLS,
		SentFolder:  "Sent",
		PasswordEnc: []byte("fake-enc"),
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestComposeEmail_SendsViaSMTP(t *testing.T) {
	sender := &stubSender{}
	appender := &stubAppender{}
	threads := &stubThreadRepo{}
	msgs := &stubMsgRepo{}
	accounts := &stubAccountRepo{account: testAccount()}

	h := commands.NewComposeEmailHandler(accounts, threads, msgs, sender, appender, &stubPublisher{})
	err := h.Handle(context.Background(), commands.ComposeEmailCommand{
		AccountID: "acc1",
		To:        "bob@example.com",
		Subject:   "Hello",
		Body:      "Hi Bob",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("want sender called once, got %d", sender.calls)
	}
}

func TestComposeEmail_CreatesThread(t *testing.T) {
	threads := &stubThreadRepo{}
	msgs := &stubMsgRepo{}
	accounts := &stubAccountRepo{account: testAccount()}

	h := commands.NewComposeEmailHandler(accounts, threads, msgs, &stubSender{}, &stubAppender{}, &stubPublisher{})
	_ = h.Handle(context.Background(), commands.ComposeEmailCommand{
		AccountID: "acc1", To: "bob@example.com", Subject: "Test subject", Body: "body",
	})

	if len(threads.saved) != 1 {
		t.Fatalf("want 1 thread saved, got %d", len(threads.saved))
	}
	if threads.saved[0].Subject != "Test subject" {
		t.Fatalf("wrong subject: %q", threads.saved[0].Subject)
	}
	if threads.saved[0].MessageCount != 1 {
		t.Fatalf("want MessageCount=1, got %d", threads.saved[0].MessageCount)
	}
}

func TestComposeEmail_SavesOutboundMessage(t *testing.T) {
	threads := &stubThreadRepo{}
	msgs := &stubMsgRepo{}
	accounts := &stubAccountRepo{account: testAccount()}

	h := commands.NewComposeEmailHandler(accounts, threads, msgs, &stubSender{}, &stubAppender{}, &stubPublisher{})
	_ = h.Handle(context.Background(), commands.ComposeEmailCommand{
		AccountID: "acc1", To: "bob@example.com", Subject: "Hello", Body: "Hi",
	})

	if len(msgs.saved) != 1 {
		t.Fatalf("want 1 message saved, got %d", len(msgs.saved))
	}
	m := msgs.saved[0]
	if m.Direction != "out" {
		t.Fatalf("want direction=out, got %q", m.Direction)
	}
	if m.ToAddrs != "bob@example.com" {
		t.Fatalf("wrong to: %q", m.ToAddrs)
	}
	if m.ReceivedAt.IsZero() {
		t.Fatal("ReceivedAt should not be zero")
	}
}

func TestComposeEmail_AppendsToSentFolder(t *testing.T) {
	appender := &stubAppender{}
	accounts := &stubAccountRepo{account: testAccount()}

	h := commands.NewComposeEmailHandler(accounts, &stubThreadRepo{}, &stubMsgRepo{}, &stubSender{}, appender, &stubPublisher{})
	_ = h.Handle(context.Background(), commands.ComposeEmailCommand{
		AccountID: "acc1", To: "bob@example.com", Subject: "Hello", Body: "Hi",
	})

	if appender.calls != 1 {
		t.Fatalf("want appender called once, got %d", appender.calls)
	}
	if appender.lastFolder != "Sent" {
		t.Fatalf("want folder=Sent, got %q", appender.lastFolder)
	}
}

func TestComposeEmail_EmptyBody_ReturnsError(t *testing.T) {
	h := commands.NewComposeEmailHandler(
		&stubAccountRepo{account: testAccount()},
		&stubThreadRepo{}, &stubMsgRepo{}, &stubSender{}, &stubAppender{}, &stubPublisher{},
	)
	err := h.Handle(context.Background(), commands.ComposeEmailCommand{
		AccountID: "acc1", To: "bob@example.com", Subject: "Hello", Body: "",
	})
	if err == nil {
		t.Fatal("want error for empty body")
	}
}

func TestComposeEmail_EmptyTo_ReturnsError(t *testing.T) {
	h := commands.NewComposeEmailHandler(
		&stubAccountRepo{account: testAccount()},
		&stubThreadRepo{}, &stubMsgRepo{}, &stubSender{}, &stubAppender{}, &stubPublisher{},
	)
	err := h.Handle(context.Background(), commands.ComposeEmailCommand{
		AccountID: "acc1", To: "", Subject: "Hello", Body: "body",
	})
	if err == nil {
		t.Fatal("want error for empty to")
	}
}

var _ = time.Now // keep time import used
