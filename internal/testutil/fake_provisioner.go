package testutil

import (
	"context"
	"sync"

	"github.com/atvirokodosprendimai/vvs/internal/modules/network/domain"
)

// ARPCall records a single SetARPStatic call.
type ARPCall struct {
	IP         string
	MAC        string
	CustomerID string
	Conn       domain.RouterConn
}

// DisableARPCall records a single DisableARP call.
type DisableARPCall struct {
	IP   string
	Conn domain.RouterConn
}

// FakeRouterProvisioner is a test double for domain.RouterProvisioner.
// It records all calls and supports configurable per-method errors.
type FakeRouterProvisioner struct {
	mu sync.Mutex

	SetARPStaticCalls []ARPCall
	DisableARPCalls   []DisableARPCall
	GetARPEntryCalls  []string

	setARPErr    error
	disableARPErr error
}

// SetError configures FakeRouterProvisioner to return err for the named method.
// Valid methods: "SetARPStatic", "DisableARP", "GetARPEntry".
func (f *FakeRouterProvisioner) SetError(method string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch method {
	case "SetARPStatic":
		f.setARPErr = err
	case "DisableARP":
		f.disableARPErr = err
	}
}

// Reset clears all recorded calls and configured errors.
func (f *FakeRouterProvisioner) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SetARPStaticCalls = nil
	f.DisableARPCalls = nil
	f.GetARPEntryCalls = nil
	f.setARPErr = nil
	f.disableARPErr = nil
}

func (f *FakeRouterProvisioner) SetARPStatic(_ context.Context, conn domain.RouterConn, ip, mac, customerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SetARPStaticCalls = append(f.SetARPStaticCalls, ARPCall{IP: ip, MAC: mac, CustomerID: customerID, Conn: conn})
	return f.setARPErr
}

func (f *FakeRouterProvisioner) DisableARP(_ context.Context, conn domain.RouterConn, ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DisableARPCalls = append(f.DisableARPCalls, DisableARPCall{IP: ip, Conn: conn})
	return f.disableARPErr
}

func (f *FakeRouterProvisioner) GetARPEntry(_ context.Context, _ domain.RouterConn, ip string) (*domain.ARPEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.GetARPEntryCalls = append(f.GetARPEntryCalls, ip)
	return nil, nil
}
