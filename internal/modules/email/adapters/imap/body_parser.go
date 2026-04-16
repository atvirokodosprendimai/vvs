package imap

import (
	"bytes"
	"io"
	"log"
	"strings"

	"github.com/emersion/go-message/mail"
)

// ExtractTextHTMLForTest is exported for testing.
var ExtractTextHTMLForTest = extractTextHTML

// extractTextHTML parses raw RFC 2822 message bytes and returns text/plain and text/html parts
// decoded to UTF-8. Both may be empty if the message has no text parts.
func extractTextHTML(raw []byte) (text, html string) {
	r, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		// Not a full RFC 2822 message — treat raw as plain text body.
		return DecodeToUTF8("utf-8", string(raw)), ""
	}
	defer r.Close()

	for {
		part, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("email: parse part: %v", err)
			break
		}

		// ContentType is on *mail.InlineHeader (embedded message.Header).
		var ct, partCharset string
		if ih, ok := part.Header.(*mail.InlineHeader); ok {
			var params map[string]string
			ct, params, _ = ih.ContentType()
			partCharset = params["charset"]
		}

		body, err := DecodeReaderToUTF8(partCharset, part.Body)
		if err != nil {
			continue
		}

		switch {
		case strings.HasPrefix(ct, "text/plain"):
			if text == "" {
				text = body
			}
		case strings.HasPrefix(ct, "text/html"):
			if html == "" {
				html = body
			}
		}
	}
	return
}
