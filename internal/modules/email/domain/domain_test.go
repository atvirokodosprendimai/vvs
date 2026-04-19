package domain_test

import (
	"testing"

	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
)

func TestNewEmailAccount_defaults(t *testing.T) {
	a, err := domain.NewEmailAccount("id1", "Work", "mail.example.com", 993, "user@example.com", []byte("enc"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Folder != "INBOX" {
		t.Errorf("want INBOX, got %q", a.Folder)
	}
	if a.TLS != domain.TLSTLS {
		t.Errorf("want tls, got %q", a.TLS)
	}
	if a.Status != domain.AccountStatusActive {
		t.Errorf("want active, got %q", a.Status)
	}
}

func TestNewEmailAccount_validation(t *testing.T) {
	_, err := domain.NewEmailAccount("id", "", "host", 993, "user", nil, "", "")
	if err != domain.ErrAccountNameEmpty {
		t.Errorf("want ErrAccountNameEmpty, got %v", err)
	}
	_, err = domain.NewEmailAccount("id", "name", "", 993, "user", nil, "", "")
	if err != domain.ErrAccountHostEmpty {
		t.Errorf("want ErrAccountHostEmpty, got %v", err)
	}
	_, err = domain.NewEmailAccount("id", "name", "host", 993, "", nil, "", "")
	if err != domain.ErrAccountUserEmpty {
		t.Errorf("want ErrAccountUserEmpty, got %v", err)
	}
}

func TestEmailAccount_lifecycle(t *testing.T) {
	a, _ := domain.NewEmailAccount("id1", "Work", "host", 993, "user", nil, "", "")

	a.SetError("timeout")
	if a.Status != domain.AccountStatusError {
		t.Errorf("want error, got %q", a.Status)
	}
	if a.LastError != "timeout" {
		t.Errorf("want timeout, got %q", a.LastError)
	}

	a.Resume()
	if a.Status != domain.AccountStatusActive {
		t.Errorf("want active after resume, got %q", a.Status)
	}
	if a.LastError != "" {
		t.Errorf("want empty LastError after resume, got %q", a.LastError)
	}

	a.Pause()
	if a.Status != domain.AccountStatusPaused {
		t.Errorf("want paused, got %q", a.Status)
	}

	a.MarkSynced(42)
	if a.LastUID != 42 {
		t.Errorf("want LastUID 42, got %d", a.LastUID)
	}
	if a.Status != domain.AccountStatusActive {
		t.Errorf("want active after sync, got %q", a.Status)
	}
}

func TestNormalizeSubject(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Re: Hello", "Hello"},
		{"re: Re: Hello", "Hello"},
		{"Fwd: Hello", "Hello"},
		{"FWD: Hello", "Hello"},
		{"fw: Hello", "Hello"},
		{"Hello", "Hello"},
		{"Re:Hello", "Re:Hello"}, // no space after colon — not stripped
	}
	for _, c := range cases {
		got := domain.NormalizeSubject(c.in)
		if got != c.want {
			t.Errorf("NormalizeSubject(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEmailMessage_ReferenceIDs(t *testing.T) {
	m := domain.EmailMessage{
		References: "<a@x> <b@x> <c@x>",
	}
	ids := m.ReferenceIDs()
	if len(ids) != 3 {
		t.Fatalf("want 3 ids, got %d", len(ids))
	}
	if ids[0] != "<a@x>" || ids[1] != "<b@x>" || ids[2] != "<c@x>" {
		t.Errorf("unexpected ids: %v", ids)
	}
}
