package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	tickethttp "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/adapters/http"
	ticketcommands "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/app/commands"
	ticketqueries "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/ticket/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// ── stubs ────────────────────────────────────────────────────────────────────

type ticketNoopPub struct{}

func (n *ticketNoopPub) Publish(_ context.Context, _ string, _ events.DomainEvent) error { return nil }

type ticketNoopSub struct{}

func (n *ticketNoopSub) Subscribe(_ string, _ events.EventHandler) error { return nil }
func (n *ticketNoopSub) ChanSubscription(_ string) (<-chan events.DomainEvent, func()) {
	ch := make(chan events.DomainEvent)
	close(ch)
	return ch, func() {}
}
func (n *ticketNoopSub) Close() error { return nil }

type ticketStubRepo struct {
	tickets  []*domain.Ticket
	comments []*domain.TicketComment
}

func (r *ticketStubRepo) Save(_ context.Context, t *domain.Ticket) error {
	for i, existing := range r.tickets {
		if existing.ID == t.ID {
			r.tickets[i] = t
			return nil
		}
	}
	r.tickets = append(r.tickets, t)
	return nil
}

func (r *ticketStubRepo) FindByID(_ context.Context, id string) (*domain.Ticket, error) {
	for _, t := range r.tickets {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *ticketStubRepo) ListAll(_ context.Context) ([]*domain.Ticket, error) {
	return r.tickets, nil
}

func (r *ticketStubRepo) ListForCustomer(_ context.Context, customerID string) ([]*domain.Ticket, error) {
	var out []*domain.Ticket
	for _, t := range r.tickets {
		if t.CustomerID == customerID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *ticketStubRepo) Delete(_ context.Context, id string) error {
	for i, t := range r.tickets {
		if t.ID == id {
			r.tickets = append(r.tickets[:i], r.tickets[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *ticketStubRepo) SaveComment(_ context.Context, c *domain.TicketComment) error {
	r.comments = append(r.comments, c)
	return nil
}

func (r *ticketStubRepo) ListComments(_ context.Context, ticketID string) ([]*domain.TicketComment, error) {
	var out []*domain.TicketComment
	for _, c := range r.comments {
		if c.TicketID == ticketID {
			out = append(out, c)
		}
	}
	return out, nil
}

// stubNameResolver satisfies the unexported customerNameResolver interface in ticket queries.
type stubNameResolver struct{}

func (s *stubNameResolver) CustomerName(_ context.Context, _ string) string { return "Test Corp" }

// ── helpers ──────────────────────────────────────────────────────────────────

const ticketTestCustomerID = "cust-ticket-http-001"

func ticketRouter(t *testing.T) (http.Handler, *ticketStubRepo) {
	t.Helper()
	repo := &ticketStubRepo{}
	pub := &ticketNoopPub{}
	sub := &ticketNoopSub{}
	resolver := &stubNameResolver{}

	h := tickethttp.NewHandlers(
		ticketcommands.NewOpenTicketHandler(repo, pub),
		ticketcommands.NewUpdateTicketHandler(repo, pub),
		ticketcommands.NewDeleteTicketHandler(repo, pub),
		ticketcommands.NewChangeTicketStatusHandler(repo, pub),
		ticketcommands.NewAddCommentHandler(repo, pub),
		ticketqueries.NewListTicketsForCustomerHandler(repo),
		ticketqueries.NewListCommentsHandler(repo),
		sub,
		pub,
	)
	h.WithListAll(ticketqueries.NewListAllTicketsHandler(repo, resolver))
	h.WithGetTicket(ticketqueries.NewGetTicketHandler(repo, resolver))

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, repo
}

func openTicketViaHandler(t *testing.T, router http.Handler, subject string) {
	t.Helper()
	body := `{"ticketSubject":"` + subject + `","ticketBody":"body text","ticketPriority":"normal"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers/"+ticketTestCustomerID+"/tickets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestTicketsPage_OK(t *testing.T) {
	router, _ := ticketRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tickets", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestTicketListForCustomerSSE_ContentType(t *testing.T) {
	router, _ := ticketRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/customers/"+ticketTestCustomerID+"/tickets", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestTicketListAllSSE_ContentType(t *testing.T) {
	router, _ := ticketRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/tickets", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestTicketOpenSSE_CreatesTicket(t *testing.T) {
	router, repo := ticketRouter(t)
	openTicketViaHandler(t, router, "Router down")

	if len(repo.tickets) != 1 {
		t.Fatalf("want 1 ticket, got %d", len(repo.tickets))
	}
	if repo.tickets[0].Subject != "Router down" {
		t.Fatalf("want 'Router down', got %q", repo.tickets[0].Subject)
	}
	if repo.tickets[0].Status != domain.StatusOpen {
		t.Fatalf("want status open, got %q", repo.tickets[0].Status)
	}
}

func TestTicketUpdateSSE_UpdatesTicket(t *testing.T) {
	router, repo := ticketRouter(t)
	openTicketViaHandler(t, router, "Original subject")

	id := repo.tickets[0].ID
	updateBody := `{"ticketSubject":"Updated subject","ticketBody":"new body","ticketPriority":"high"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/tickets/"+id, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	ticket, _ := repo.FindByID(context.Background(), id)
	if ticket.Subject != "Updated subject" {
		t.Fatalf("want 'Updated subject', got %q", ticket.Subject)
	}
}

func TestTicketChangeStatusSSE_StartsTicket(t *testing.T) {
	router, repo := ticketRouter(t)
	openTicketViaHandler(t, router, "Status test")

	id := repo.tickets[0].ID
	statusBody := `{"ticketAction":"start"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/tickets/"+id+"/status", strings.NewReader(statusBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	ticket, _ := repo.FindByID(context.Background(), id)
	if ticket.Status != domain.StatusInProgress {
		t.Fatalf("want in_progress, got %q", ticket.Status)
	}
}

func TestTicketDeleteSSE_DeletesTicket(t *testing.T) {
	router, repo := ticketRouter(t)
	openTicketViaHandler(t, router, "Delete me")

	id := repo.tickets[0].ID
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/tickets/"+id, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if len(repo.tickets) != 0 {
		t.Fatalf("want 0 tickets after delete, got %d", len(repo.tickets))
	}
}

func TestTicketAddCommentSSE_AddsComment(t *testing.T) {
	router, repo := ticketRouter(t)
	openTicketViaHandler(t, router, "Comment ticket")

	id := repo.tickets[0].ID
	commentBody := `{"commentBody":"This is a comment"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/"+id+"/comments", strings.NewReader(commentBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	comments, _ := repo.ListComments(context.Background(), id)
	if len(comments) != 1 {
		t.Fatalf("want 1 comment, got %d", len(comments))
	}
	if comments[0].Body != "This is a comment" {
		t.Fatalf("want comment body, got %q", comments[0].Body)
	}
}

func TestTicketListCommentsSSE_ContentType(t *testing.T) {
	router, repo := ticketRouter(t)
	openTicketViaHandler(t, router, "Comments SSE test")
	id := repo.tickets[0].ID

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/tickets/"+id+"/comments", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}
