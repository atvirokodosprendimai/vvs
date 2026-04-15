package domain

import "context"

// ARPEntry represents a single ARP table entry on a router.
type ARPEntry struct {
	ID         string
	IPAddress  string
	MACAddress string
	Interface  string
	Disabled   bool
	Static     bool
}

// RouterConn holds the connection parameters for a specific router.
// Passed per-call so the provisioner can be a single shared instance.
type RouterConn struct {
	RouterID string // pool key — used to reuse existing TCP connections
	Host     string
	Port     int // default 8728
	Username string
	Password string
}

// RouterProvisioner is a vendor-agnostic port for L2 access control.
// Swap MikroTik → Arista by changing one line in app.go.
type RouterProvisioner interface {
	// SetARPStatic creates or enables a static ARP entry (grants access).
	// customerID is used as a comment on the router for traceability.
	SetARPStatic(ctx context.Context, conn RouterConn, ip, mac, customerID string) error

	// DisableARP disables the ARP entry for the given IP (cuts access).
	DisableARP(ctx context.Context, conn RouterConn, ip string) error

	// GetARPEntry returns the current ARP entry for the IP, or nil if absent.
	GetARPEntry(ctx context.Context, conn RouterConn, ip string) (*ARPEntry, error)
}
