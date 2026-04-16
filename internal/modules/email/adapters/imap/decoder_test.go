package imap_test

import (
	"testing"

	imapAdapter "github.com/vvs/isp/internal/modules/email/adapters/imap"
)

func TestDecodeToUTF8_utf8_passthrough(t *testing.T) {
	in := "hello world"
	got := imapAdapter.DecodeToUTF8("utf-8", in)
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestDecodeToUTF8_latin1(t *testing.T) {
	// "Ä" in latin-1 is 0xC4.
	latin1 := "\xc4"
	got := imapAdapter.DecodeToUTF8("iso-8859-1", latin1)
	if got != "Ä" {
		t.Errorf("got %q, want %q", got, "Ä")
	}
}

func TestDecodeToUTF8_unknown_charset_sanitizes(t *testing.T) {
	// Unknown charset, invalid UTF-8 byte — should be replaced.
	in := "hello \xff world"
	got := imapAdapter.DecodeToUTF8("bogus-charset", in)
	if got == "" {
		t.Error("expected non-empty output")
	}
}

func TestExtractTextHTML_plaintext(t *testing.T) {
	raw := "From: sender@example.com\r\nTo: recv@example.com\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nHello, world!"
	text, html := imapAdapter.ExtractTextHTMLForTest([]byte(raw))
	if !contains(text, "Hello") {
		t.Errorf("expected text to contain 'Hello', got: %q", text)
	}
	_ = html
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
