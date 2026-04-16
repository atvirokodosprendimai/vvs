package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vvs/isp/internal/modules/network/app/commands"
	"github.com/vvs/isp/internal/modules/network/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubCustomerProvider struct {
	data domain.CustomerARPData
	updateCalled bool
}

func (s *stubCustomerProvider) FindARPData(_ context.Context, _ string) (domain.CustomerARPData, error) {
	return s.data, nil
}
func (s *stubCustomerProvider) UpdateNetworkInfo(_ context.Context, _, _, _, _ string) error {
	s.updateCalled = true
	return nil
}

// stubRouterRepo returns a single pre-configured router
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

// countingIPAM counts how many times each method is called
type countingIPAM struct {
	getCalls    int
	updateCalls int
	ip          string
	mac         string
	id          int
}

func (c *countingIPAM) GetIPByCustomerCode(_ context.Context, _ string) (string, string, int, error) {
	c.getCalls++
	return c.ip, c.mac, c.id, nil
}
func (c *countingIPAM) AllocateIP(_ context.Context, _, _ string) (string, int, error) {
	return c.ip, c.id, nil
}
func (c *countingIPAM) UpdateARPStatus(_ context.Context, _ int, _ string) error {
	c.updateCalls++
	return nil
}

type stubProvisioner struct {
	setARPCalled    bool
	disableARPCalled bool
}

func (p *stubProvisioner) SetARPStatic(_ context.Context, _ domain.RouterConn, _, _, _ string) error {
	p.setARPCalled = true
	return nil
}
func (p *stubProvisioner) DisableARP(_ context.Context, _ domain.RouterConn, _ string) error {
	p.disableARPCalled = true
	return nil
}
func (p *stubProvisioner) GetARPEntry(_ context.Context, _ domain.RouterConn, _ string) (*domain.ARPEntry, error) {
	return nil, nil
}

type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _ string, _ events.DomainEvent) error { return nil }

// ── helpers ───────────────────────────────────────────────────────────────────

func routerID(id string) *string { return &id }

func mustRouter(t *testing.T) *domain.Router {
	t.Helper()
	r, err := domain.NewRouter("edge-01", "mikrotik", "10.0.0.1", 8728, "admin", "pass", "")
	if err != nil {
		t.Fatalf("mustRouter: %v", err)
	}
	id := "router-1"
	r.ID = id
	return r
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestSyncARP_Enable_CallsSetARPStatic(t *testing.T) {
	router := mustRouter(t)
	prov := &stubProvisioner{}
	ipam := &countingIPAM{ip: "10.0.1.1", mac: "AA:BB:CC:DD:EE:FF", id: 42}

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomerProvider{data: domain.CustomerARPData{
			ID: "c1", Code: "CLI-00001", RouterID: routerID(router.ID),
			IPAddress: "10.0.1.1", MACAddress: "AA:BB:CC:DD:EE:FF",
		}},
		&stubRouterRepo{router: router},
		prov, ipam, noopPublisher{},
	)

	if err := h.Handle(context.Background(), commands.SyncCustomerARPCommand{
		CustomerID: "c1", Action: commands.ARPActionEnable,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prov.setARPCalled {
		t.Fatal("want SetARPStatic called")
	}
}

func TestSyncARP_Disable_CallsDisableARP(t *testing.T) {
	router := mustRouter(t)
	prov := &stubProvisioner{}

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomerProvider{data: domain.CustomerARPData{
			ID: "c2", Code: "CLI-00002", RouterID: routerID(router.ID),
			IPAddress: "10.0.1.2",
		}},
		&stubRouterRepo{router: router},
		prov, nil, noopPublisher{},
	)

	if err := h.Handle(context.Background(), commands.SyncCustomerARPCommand{
		CustomerID: "c2", Action: commands.ARPActionDisable,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prov.disableARPCalled {
		t.Fatal("want DisableARP called")
	}
}

func TestSyncARP_NoRouter_NoOp(t *testing.T) {
	prov := &stubProvisioner{}

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomerProvider{data: domain.CustomerARPData{ID: "c3", RouterID: nil}},
		&stubRouterRepo{}, prov, nil, noopPublisher{},
	)

	if err := h.Handle(context.Background(), commands.SyncCustomerARPCommand{
		CustomerID: "c3", Action: commands.ARPActionEnable,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.setARPCalled || prov.disableARPCalled {
		t.Fatal("want no provisioner calls when no router assigned")
	}
}

// ── double-call regression ────────────────────────────────────────────────────

// TestSyncARP_IPAMCalledOnceWhenIPAlreadyKnown verifies that GetIPByCustomerCode
// is NOT called when the customer already has an IP address stored — it should
// only be called once (for UpdateARPStatus path) or zero times if we cache ipID.
func TestSyncARP_IPAMCalledOnlyOnce_WhenIPMustBeResolved(t *testing.T) {
	router := mustRouter(t)
	prov := &stubProvisioner{}
	ipam := &countingIPAM{ip: "10.0.1.5", mac: "DE:AD:BE:EF:00:01", id: 7}

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomerProvider{data: domain.CustomerARPData{
			ID: "c4", Code: "CLI-00004", RouterID: routerID(router.ID),
			IPAddress: "", // empty — triggers IPAM lookup
		}},
		&stubRouterRepo{router: router},
		prov, ipam, noopPublisher{},
	)

	if err := h.Handle(context.Background(), commands.SyncCustomerARPCommand{
		CustomerID: "c4", Action: commands.ARPActionEnable,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ipam.getCalls != 1 {
		t.Fatalf("want GetIPByCustomerCode called exactly once, got %d", ipam.getCalls)
	}
	if ipam.updateCalls != 1 {
		t.Fatalf("want UpdateARPStatus called once, got %d", ipam.updateCalls)
	}
}

func TestSyncARP_IPAMNotCalledWhenIPAlreadyKnown(t *testing.T) {
	router := mustRouter(t)
	prov := &stubProvisioner{}
	ipam := &countingIPAM{ip: "10.0.1.5", mac: "DE:AD:BE:EF:00:01", id: 7}

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomerProvider{data: domain.CustomerARPData{
			ID: "c5", Code: "CLI-00005", RouterID: routerID(router.ID),
			IPAddress: "10.0.1.5", MACAddress: "DE:AD:BE:EF:00:01",
		}},
		&stubRouterRepo{router: router},
		prov, ipam, noopPublisher{},
	)

	if err := h.Handle(context.Background(), commands.SyncCustomerARPCommand{
		CustomerID: "c5", Action: commands.ARPActionEnable,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// IP already known — no resolution call needed; UpdateARPStatus still uses stored id
	if ipam.getCalls != 0 {
		t.Fatalf("want GetIPByCustomerCode NOT called when IP already known, got %d calls", ipam.getCalls)
	}
}

func TestSyncARP_UnknownAction_Error(t *testing.T) {
	router := mustRouter(t)

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomerProvider{data: domain.CustomerARPData{
			ID: "c6", RouterID: routerID(router.ID), IPAddress: "10.0.0.1",
		}},
		&stubRouterRepo{router: router},
		&stubProvisioner{}, nil, noopPublisher{},
	)

	err := h.Handle(context.Background(), commands.SyncCustomerARPCommand{
		CustomerID: "c6", Action: "bad-action",
	})
	if err == nil {
		t.Fatal("want error for unknown action")
	}
}

func TestSyncARP_EnableWithoutMAC_Error(t *testing.T) {
	router := mustRouter(t)

	h := commands.NewSyncCustomerARPHandler(
		&stubCustomerProvider{data: domain.CustomerARPData{
			ID: "c7", RouterID: routerID(router.ID),
			IPAddress: "10.0.0.1", MACAddress: "", // no MAC
		}},
		&stubRouterRepo{router: router},
		&stubProvisioner{}, nil, noopPublisher{},
	)

	err := h.Handle(context.Background(), commands.SyncCustomerARPCommand{
		CustomerID: "c7", Action: commands.ARPActionEnable,
	})
	if err == nil || !errors.Is(err, nil) {
		// just need an error, not nil
	}
	if err == nil {
		t.Fatal("want error when enabling without MAC address")
	}
}
