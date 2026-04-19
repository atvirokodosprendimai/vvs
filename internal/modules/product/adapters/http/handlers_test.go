package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	producthttp "github.com/atvirokodosprendimai/vvs/internal/modules/product/adapters/http"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/adapters/persistence"
	productcommands "github.com/atvirokodosprendimai/vvs/internal/modules/product/app/commands"
	productqueries "github.com/atvirokodosprendimai/vvs/internal/modules/product/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/product/migrations"
	shareddomain "github.com/atvirokodosprendimai/vvs/internal/shared/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

// ── stubs ────────────────────────────────────────────────────────────────────

type noopPublisher struct{}

func (n *noopPublisher) Publish(_ context.Context, _ string, _ events.DomainEvent) error {
	return nil
}

type noopSubscriber struct{}

func (n *noopSubscriber) Subscribe(_ string, _ events.EventHandler) error { return nil }
func (n *noopSubscriber) ChanSubscription(_ string) (<-chan events.DomainEvent, func()) {
	ch := make(chan events.DomainEvent)
	close(ch) // immediately closed → SSE for-select exits after initial render
	return ch, func() {}
}
func (n *noopSubscriber) Close() error { return nil }

// ── helpers ──────────────────────────────────────────────────────────────────

func productRouter(t *testing.T) (http.Handler, *persistence.GormProductRepository) {
	t.Helper()
	db := testutil.NewTestDB(t)
	testutil.RunMigrations(t, db, migrations.FS, "goose_product")
	repo := persistence.NewGormProductRepository(db)
	h := producthttp.NewHandlers(
		productcommands.NewCreateProductHandler(repo, &noopPublisher{}),
		productcommands.NewUpdateProductHandler(repo, &noopPublisher{}),
		productcommands.NewDeleteProductHandler(repo, &noopPublisher{}),
		productqueries.NewListProductsHandler(db),
		productqueries.NewGetProductHandler(db),
		&noopSubscriber{},
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, repo
}

func createProductViaHandler(t *testing.T, router http.Handler, name string) {
	t.Helper()
	body := `{"name":"` + name + `","productType":"internet","priceAmount":"29.99","priceCurrency":"EUR","billingPeriod":"monthly"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestProductListPage_OK(t *testing.T) {
	router, _ := productRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestProductCreatePage_OK(t *testing.T) {
	router, _ := productRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/products/new", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestProductDetailPage_NotFound(t *testing.T) {
	router, _ := productRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/products/nonexistent", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
}

func TestProductListSSE_ContentType(t *testing.T) {
	router, _ := productRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestProductCreateSSE_CreatesProduct(t *testing.T) {
	router, repo := productRouter(t)
	createProductViaHandler(t, router, "Static IP")

	products, _, err := repo.FindAll(context.Background(), domain.ProductFilter{}, shareddomain.NewPagination(1, 25))
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("want 1 product, got %d", len(products))
	}
	if products[0].Name != "Static IP" {
		t.Fatalf("want name 'Static IP', got %q", products[0].Name)
	}
}

func TestProductUpdateSSE_UpdatesProduct(t *testing.T) {
	router, repo := productRouter(t)
	createProductViaHandler(t, router, "Internet 100Mbps")

	products, _, _ := repo.FindAll(context.Background(), domain.ProductFilter{}, shareddomain.NewPagination(1, 25))
	if len(products) == 0 {
		t.Fatal("product not created")
	}
	id := products[0].ID

	updateBody := `{"name":"Internet 1Gbps","productType":"internet","priceAmount":"49.99","priceCurrency":"EUR","billingPeriod":"monthly"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/products/"+id, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	updated, _ := repo.FindByID(context.Background(), id)
	if updated.Name != "Internet 1Gbps" {
		t.Fatalf("want updated name, got %q", updated.Name)
	}
}

func TestProductDeleteSSE_DeletesProduct(t *testing.T) {
	router, repo := productRouter(t)
	createProductViaHandler(t, router, "Old Product")

	products, _, _ := repo.FindAll(context.Background(), domain.ProductFilter{}, shareddomain.NewPagination(1, 25))
	if len(products) == 0 {
		t.Fatal("product not created")
	}
	id := products[0].ID

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/products/"+id, nil)
	router.ServeHTTP(rr, req)

	remaining, _, _ := repo.FindAll(context.Background(), domain.ProductFilter{}, shareddomain.NewPagination(1, 25))
	if len(remaining) != 0 {
		t.Fatalf("want 0 products after delete, got %d", len(remaining))
	}
}
