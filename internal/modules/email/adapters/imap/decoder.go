package imap

import (
	"bytes"
	"io"
	"mime/quotedprintable"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
)

// DecodeToUTF8 decodes content encoded with the given MIME charset into UTF-8.
// If charset is empty or unknown, attempts windows-1252 fallback then replaces
// invalid bytes with the UTF-8 replacement character.
func DecodeToUTF8(charset, content string) string {
	if charset == "" || strings.EqualFold(charset, "utf-8") || strings.EqualFold(charset, "us-ascii") {
		return sanitizeUTF8(content)
	}

	dec := decoderForCharset(charset)
	if dec == nil {
		// Unknown charset — try windows-1252 heuristic, then sanitize.
		dec = charmap.Windows1252.NewDecoder()
	}

	out, err := dec.String(content)
	if err != nil {
		return sanitizeUTF8(content)
	}
	return out
}

// DecodeQP decodes quoted-printable encoded content.
func DecodeQP(s string) string {
	r := quotedprintable.NewReader(strings.NewReader(s))
	b, err := io.ReadAll(r)
	if err != nil {
		return s
	}
	return string(b)
}

// DecodeReaderToUTF8 reads all bytes from r and decodes them from charset to UTF-8.
func DecodeReaderToUTF8(charset string, r io.Reader) (string, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	if charset == "" || strings.EqualFold(charset, "utf-8") || strings.EqualFold(charset, "us-ascii") {
		return sanitizeUTF8(string(raw)), nil
	}

	dec := decoderForCharset(charset)
	if dec == nil {
		dec = charmap.Windows1252.NewDecoder()
	}

	out, err := dec.Bytes(raw)
	if err != nil {
		return sanitizeUTF8(string(raw)), nil
	}
	return string(out), nil
}

// DecodeReaderToUTF8Bytes is like DecodeReaderToUTF8 but returns []byte (for binary parts).
func DecodeReaderToUTF8Bytes(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func decoderForCharset(charset string) *encoding.Decoder {
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return nil
	}
	return enc.NewDecoder()
}

func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var buf bytes.Buffer
	for _, r := range []byte(s) {
		if utf8.RuneLen(rune(r)) < 0 {
			buf.WriteRune('\uFFFD')
		} else {
			buf.WriteByte(r)
		}
	}
	return buf.String()
}
