package subscribers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/network/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/network/app/subscribers"
	"github.com/atvirokodosprendimai/vvs/internal/modules/network/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/atvirokodosprendimai/vvs/internal/testutil"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubCustomer struct {
	data domain.CustomerARPData
}

func (s *stubCustomer) FindARPData(_ context.Context, _ string) (domain.CustomerARPData, error) {
	return s.data, nil
}
func (s *stubCustomer) UpdateNetworkInfo(_ context.Context, _, _, _, _ string) error { return nil }

type stubRouterRepo struct {
	router *domain.Router
}

func (r *stubRouterRepo) FindByID(_ context.Context, _ string) (*domain.Router, error) {
	if r.router == nil {
		return nil, domain.ErrRouterNotFound
	}
	return r.router, nil
}
func (r *stubRouterRepo) Save(_ context.Context, _ *domain.Router) error { return nil }
func (r *stubRouterRepo) Delete(_ context.Context, _ string) error       { return nil }
func (r *stubRouterRepo) FindAll(_ context.Context) ([]*domain.Router, error) {
	if r.router != nil {
		return []*domain.Router{r.router}, nil
	}
	return nil, nil
}

type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _ string, _ events.DomainEvent) error { return nil }

// ── helper ────────────────────────────────────────────────────────────────────

func testRouter(t *testing.T) *domain.Router {
	t.Helper()
	r, err := domain.NewRouter("edge-01", "mikrotik", "10.0.0.1", 8728, "admin", "pass", "")
	if err != nil {
		t.Fatalf("testRouter: %v", err)
	}
	r.ID = "router-1"
	return r
}

func routerIDPtr(id string) *string { return &id }

func publishEvent(t *testing.T, pub events.EventPublisher, subject string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	pub.Publish(context.Background(), subject, events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        subject,
		AggregateID: "test",
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
}

// waitForCall polls fn until it returns true or timeout expires.
func waitForCall(fn func() bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestARPWorker_ServiceSuspended_DisablesARP(t *testing.T) {
	pub, sub := testutil.NewTestNATS(t)
	fake := &testutil.FakeRouterProvisioner{}
	router := testRouter(t)

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomer{data: domain.CustomerARPData{
			ID: "cust-1", Code: "CLI-001",
			RouterID:  routerIDPtr(router.ID),
			IPAddress: "10.0.1.1",
		}},
		&stubRouterRepo{router: router},
		fake, nil, noopPublisher{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := subscribers.NewARPWorker(h)
	go worker.Run(ctx, sub)
	time.Sleep(50 * time.Millisecond) // let subscriptions register

	publishEvent(t, pub, events.ServiceSuspended.String(), map[string]string{
		"id":          "svc-1",
		"customer_id": "cust-1",
		"status":      "suspended",
	})

	if !waitForCall(func() bool {
		return len(fake.DisableARPCalls) > 0
	}) {
		t.Fatal("DisableARP not called within timeout")
	}

	if fake.DisableARPCalls[0].IP != "10.0.1.1" {
		t.Errorf("want DisableARP for IP 10.0.1.1; got %s", fake.DisableARPCalls[0].IP)
	}
}

func TestARPWorker_ServiceReactivated_EnablesARP(t *testing.T) {
	pub, sub := testutil.NewTestNATS(t)
	fake := &testutil.FakeRouterProvisioner{}
	router := testRouter(t)

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomer{data: domain.CustomerARPData{
			ID: "cust-2", Code: "CLI-002",
			RouterID:   routerIDPtr(router.ID),
			IPAddress:  "10.0.1.2",
			MACAddress: "AA:BB:CC:DD:EE:FF",
		}},
		&stubRouterRepo{router: router},
		fake, nil, noopPublisher{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := subscribers.NewARPWorker(h)
	go worker.Run(ctx, sub)
	time.Sleep(50 * time.Millisecond)

	publishEvent(t, pub, events.ServiceReactivated.String(), map[string]string{
		"id":          "svc-2",
		"customer_id": "cust-2",
		"status":      "active",
	})

	if !waitForCall(func() bool {
		return len(fake.SetARPStaticCalls) > 0
	}) {
		t.Fatal("SetARPStatic not called within timeout")
	}

	call := fake.SetARPStaticCalls[0]
	if call.IP != "10.0.1.2" {
		t.Errorf("want IP 10.0.1.2; got %s", call.IP)
	}
	if call.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("want MAC AA:BB:CC:DD:EE:FF; got %s", call.MAC)
	}
}

func TestARPWorker_ServiceCancelled_DisablesARP(t *testing.T) {
	pub, sub := testutil.NewTestNATS(t)
	fake := &testutil.FakeRouterProvisioner{}
	router := testRouter(t)

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomer{data: domain.CustomerARPData{
			ID: "cust-3", Code: "CLI-003",
			RouterID:  routerIDPtr(router.ID),
			IPAddress: "10.0.1.3",
		}},
		&stubRouterRepo{router: router},
		fake, nil, noopPublisher{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := subscribers.NewARPWorker(h)
	go worker.Run(ctx, sub)
	time.Sleep(50 * time.Millisecond)

	publishEvent(t, pub, events.ServiceCancelled.String(), map[string]string{
		"id":          "svc-3",
		"customer_id": "cust-3",
		"status":      "cancelled",
	})

	if !waitForCall(func() bool {
		return len(fake.DisableARPCalls) > 0
	}) {
		t.Fatal("DisableARP not called for cancelled service")
	}
}
