package imap

import (
	"bytes"
	"io"
	"log"
	"strings"

	"github.com/emersion/go-message/mail"
)

const maxAttachmentSize = 20 * 1024 * 1024 // 20 MB

// ParsedMessage holds all parts extracted from a raw RFC 2822 message.
type ParsedMessage struct {
	Text        string
	HTML        string
	References  string
	Attachments []ParsedAttachment
}

// ParsedAttachment is a single MIME attachment part.
type ParsedAttachment struct {
	Filename string
	MIMEType string
	Data     []byte // nil if size > maxAttachmentSize
}

// ParseMessage parses raw RFC 2822 bytes into text/html bodies, References
// header, and attachment parts (up to maxAttachmentSize each).
func ParseMessage(raw []byte) ParsedMessage {
	r, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return ParsedMessage{Text: DecodeToUTF8("utf-8", string(raw))}
	}
	defer r.Close()

	result := ParsedMessage{
		References: r.Header.Get("References"),
	}

	for {
		part, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("email: parse part: %v", err)
			break
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			ct, params, _ := h.ContentType()
			body, err := DecodeReaderToUTF8(params["charset"], part.Body)
			if err != nil {
				continue
			}
			switch {
			case strings.HasPrefix(ct, "text/plain"):
				if result.Text == "" {
					result.Text = body
				}
			case strings.HasPrefix(ct, "text/html"):
				if result.HTML == "" {
					result.HTML = body
				}
			}

		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			ct, _, _ := h.ContentType()
			data, err := io.ReadAll(io.LimitReader(part.Body, maxAttachmentSize+1))
			if err != nil || len(data) == 0 {
				continue
			}
			if int64(len(data)) > maxAttachmentSize {
				data = nil // too large — store metadata only
			}
			result.Attachments = append(result.Attachments, ParsedAttachment{
				Filename: filename,
				MIMEType: ct,
				Data:     data,
			})
		}
	}
	return result
}

// ExtractTextHTMLForTest is exported for testing.
var ExtractTextHTMLForTest = extractTextHTML

// extractTextHTML is kept for tests; delegates to ParseMessage.
func extractTextHTML(raw []byte) (text, html string) {
	p := ParseMessage(raw)
	return p.Text, p.HTML
}
