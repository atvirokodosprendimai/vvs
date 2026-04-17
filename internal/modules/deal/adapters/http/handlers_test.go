package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	dealhttp "github.com/vvs/isp/internal/modules/deal/adapters/http"
	dealcommands "github.com/vvs/isp/internal/modules/deal/app/commands"
	dealqueries "github.com/vvs/isp/internal/modules/deal/app/queries"
	"github.com/vvs/isp/internal/modules/deal/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// ── stubs ────────────────────────────────────────────────────────────────────

type dealNoopPub struct{}

func (n *dealNoopPub) Publish(_ context.Context, _ string, _ events.DomainEvent) error { return nil }

type dealNoopSub struct{}

func (n *dealNoopSub) Subscribe(_ string, _ events.EventHandler) error { return nil }
func (n *dealNoopSub) ChanSubscription(_ string) (<-chan events.DomainEvent, func()) {
	ch := make(chan events.DomainEvent)
	close(ch)
	return ch, func() {}
}
func (n *dealNoopSub) Close() error { return nil }

type dealStubRepo struct {
	deals []*domain.Deal
}

func (r *dealStubRepo) Save(_ context.Context, d *domain.Deal) error {
	for i, existing := range r.deals {
		if existing.ID == d.ID {
			r.deals[i] = d
			return nil
		}
	}
	r.deals = append(r.deals, d)
	return nil
}

func (r *dealStubRepo) FindByID(_ context.Context, id string) (*domain.Deal, error) {
	for _, d := range r.deals {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *dealStubRepo) ListForCustomer(_ context.Context, customerID string) ([]*domain.Deal, error) {
	var out []*domain.Deal
	for _, d := range r.deals {
		if d.CustomerID == customerID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *dealStubRepo) ListAll(_ context.Context) ([]*domain.Deal, error) {
	return r.deals, nil
}

func (r *dealStubRepo) Delete(_ context.Context, id string) error {
	for i, d := range r.deals {
		if d.ID == id {
			r.deals = append(r.deals[:i], r.deals[i+1:]...)
			return nil
		}
	}
	return nil
}

type stubCustNames struct{}

func (s *stubCustNames) ResolveCustomerName(_ context.Context, _ string) string { return "Test Corp" }

// ── helpers ──────────────────────────────────────────────────────────────────

const dealTestCustomerID = "cust-deal-http-001"

func dealRouter(t *testing.T) (http.Handler, *dealStubRepo) {
	t.Helper()
	repo := &dealStubRepo{}
	pub := &dealNoopPub{}
	sub := &dealNoopSub{}

	h := dealhttp.NewHandlers(
		dealcommands.NewAddDealHandler(repo, pub),
		dealcommands.NewUpdateDealHandler(repo, pub),
		dealcommands.NewDeleteDealHandler(repo, pub),
		dealcommands.NewAdvanceDealHandler(repo, pub),
		dealqueries.NewListDealsForCustomerHandler(repo),
		sub,
	)
	h.WithListAll(dealqueries.NewListAllDealsHandler(repo))
	h.WithCustomerNames(&stubCustNames{})

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, repo
}

func addDealViaHandler(t *testing.T, router http.Handler, title string) {
	t.Helper()
	body := `{"dealTitle":"` + title + `","dealValue":"500.00","dealCurrency":"EUR","dealNotes":""}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/customers/"+dealTestCustomerID+"/deals", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestDealsPage_OK(t *testing.T) {
	router, _ := dealRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/deals", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestDealListAllSSE_ContentType(t *testing.T) {
	router, _ := dealRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/deals", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestDealListForCustomerSSE_ContentType(t *testing.T) {
	router, _ := dealRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/customers/"+dealTestCustomerID+"/deals", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestDealAddSSE_CreatesDeal(t *testing.T) {
	router, repo := dealRouter(t)
	addDealViaHandler(t, router, "Fiber Upgrade")

	if len(repo.deals) != 1 {
		t.Fatalf("want 1 deal, got %d", len(repo.deals))
	}
	if repo.deals[0].Title != "Fiber Upgrade" {
		t.Fatalf("want 'Fiber Upgrade', got %q", repo.deals[0].Title)
	}
}

func TestDealUpdateSSE_UpdatesDeal(t *testing.T) {
	router, repo := dealRouter(t)
	addDealViaHandler(t, router, "Initial Deal")

	id := repo.deals[0].ID
	updateBody := `{"dealTitle":"Revised Deal","dealValue":"750.00","dealCurrency":"EUR","dealNotes":"updated"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/deals/"+id, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	deal, _ := repo.FindByID(context.Background(), id)
	if deal.Title != "Revised Deal" {
		t.Fatalf("want 'Revised Deal', got %q", deal.Title)
	}
}

func TestDealAdvanceSSE_AdvancesDeal(t *testing.T) {
	router, repo := dealRouter(t)
	addDealViaHandler(t, router, "Pipeline Deal")

	id := repo.deals[0].ID
	advBody := `{"dealAdvanceAction":"qualify"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/deals/"+id+"/advance", strings.NewReader(advBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	deal, _ := repo.FindByID(context.Background(), id)
	if deal.Stage != domain.StageQualified {
		t.Fatalf("want StageQualified, got %q", deal.Stage)
	}
}

func TestDealDeleteSSE_DeletesDeal(t *testing.T) {
	router, repo := dealRouter(t)
	addDealViaHandler(t, router, "Deal to Delete")

	id := repo.deals[0].ID
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/deals/"+id, nil)
	router.ServeHTTP(rr, req)

	if len(repo.deals) != 0 {
		t.Fatalf("want 0 deals after delete, got %d", len(repo.deals))
	}
}

func TestDealUpdateSSE_NotFound_NoPanic(t *testing.T) {
	// SSE commits headers (200) before the not-found check runs — we just verify no panic.
	router, _ := dealRouter(t)
	body := `{"dealTitle":"X","dealValue":"100","dealCurrency":"EUR"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/deals/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
	// Response is 200 (SSE headers already flushed) — just confirm no panic.
	_ = rr.Code
}
