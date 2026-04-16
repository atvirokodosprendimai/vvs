package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	emailhttp "github.com/vvs/isp/internal/modules/email/adapters/http"
	emailqueries "github.com/vvs/isp/internal/modules/email/app/queries"
	"github.com/vvs/isp/internal/modules/email/domain"
)

// ── stub attachment repository ────────────────────────────────────────────────

type stubAttachmentRepo struct {
	rows []*domain.AttachmentSearchRow
}

func (r *stubAttachmentRepo) Save(_ context.Context, _ *domain.EmailAttachment) error { return nil }
func (r *stubAttachmentRepo) FindByID(_ context.Context, _ string) (*domain.EmailAttachment, error) {
	return nil, domain.ErrMessageNotFound
}
func (r *stubAttachmentRepo) ListForMessage(_ context.Context, _ string) ([]*domain.EmailAttachment, error) {
	return nil, nil
}
func (r *stubAttachmentRepo) SearchByFilename(_ context.Context, accountID, query string) ([]*domain.AttachmentSearchRow, error) {
	var out []*domain.AttachmentSearchRow
	for _, row := range r.rows {
		if strings.Contains(row.Filename, query) {
			out = append(out, row)
		}
	}
	return out, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newAttachmentHandlers(repo *stubAttachmentRepo) *emailhttp.Handlers {
	searchHandler := emailqueries.NewSearchAttachmentsHandler(repo)
	return emailhttp.NewHandlers(
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
	).WithSearchAttachments(searchHandler)
}

func routerWith(h *emailhttp.Handlers) http.Handler {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// datastarGet builds a GET request that sends Datastar signals via ?datastar=<json>.
func datastarGet(t *testing.T, target string, signalsJSON string) *http.Request {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	q := u.Query()
	q.Set("datastar", signalsJSON)
	u.RawQuery = q.Encode()
	return httptest.NewRequest(http.MethodGet, u.String(), nil)
}

// ── attachment search SSE tests ───────────────────────────────────────────────

func TestAttachmentSearchSSE_ContentType(t *testing.T) {
	repo := &stubAttachmentRepo{}
	rr := httptest.NewRecorder()
	req := datastarGet(t, "/sse/attachments?account=acc1", `{"q":"pdf"}`)
	routerWith(newAttachmentHandlers(repo)).ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestAttachmentSearchSSE_ReturnsMatchingResults(t *testing.T) {
	repo := &stubAttachmentRepo{
		rows: []*domain.AttachmentSearchRow{
			{ID: "a1", Filename: "invoice.pdf", MIMEType: "application/pdf", Size: 1024,
				ThreadID: "t1", ThreadSubject: "Invoice #1", FromAddr: "bill@example.com",
				ReceivedAt: time.Now()},
			{ID: "a2", Filename: "photo.jpg", MIMEType: "image/jpeg", Size: 512,
				ThreadID: "t2", ThreadSubject: "Pics", FromAddr: "alice@example.com",
				ReceivedAt: time.Now()},
		},
	}

	rr := httptest.NewRecorder()
	req := datastarGet(t, "/sse/attachments?account=acc1", `{"q":"invoice"}`)
	routerWith(newAttachmentHandlers(repo)).ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "invoice.pdf") {
		t.Fatalf("want invoice.pdf in response, got:\n%s", body)
	}
	if strings.Contains(body, "photo.jpg") {
		t.Fatalf("photo.jpg should not appear for query 'invoice', got:\n%s", body)
	}
}

func TestAttachmentSearchSSE_EmptyQuery_ReturnsPlaceholder(t *testing.T) {
	repo := &stubAttachmentRepo{
		rows: []*domain.AttachmentSearchRow{
			{ID: "a1", Filename: "invoice.pdf"},
		},
	}

	rr := httptest.NewRecorder()
	req := datastarGet(t, "/sse/attachments?account=acc1", `{"q":""}`)
	routerWith(newAttachmentHandlers(repo)).ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "invoice.pdf") {
		t.Fatalf("empty query should not return results, got:\n%s", body)
	}
	if !strings.Contains(body, "Enter a") {
		t.Fatalf("want placeholder text for empty query, got:\n%s", body)
	}
}

func TestAttachmentSearchSSE_SignalsInURL_NotQueryParam(t *testing.T) {
	// Regression: signals must come via ?datastar=<json>, NOT ?q=...
	// If the handler reads r.URL.Query().Get("q") it gets "" and returns placeholder.
	repo := &stubAttachmentRepo{
		rows: []*domain.AttachmentSearchRow{
			{ID: "a1", Filename: "report.pdf"},
		},
	}

	rr := httptest.NewRecorder()
	// Deliberately pass q as a plain URL param (wrong way — Datastar doesn't do this)
	req := httptest.NewRequest(http.MethodGet, "/sse/attachments?account=acc1&q=report", nil)
	routerWith(newAttachmentHandlers(repo)).ServeHTTP(rr, req)

	body := rr.Body.String()
	// Should NOT find report.pdf because q was not in the datastar signal param
	if strings.Contains(body, "report.pdf") {
		t.Fatalf("plain URL ?q= should NOT be read as a signal — use ?datastar= instead")
	}
}
