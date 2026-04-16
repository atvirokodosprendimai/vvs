package domain

import "context"

// IPAMProvider is a port for IP address management lookups.
// NetBox is the primary implementation; swap by changing one line in app.go.
type IPAMProvider interface {
	// GetIPByCustomerCode returns the IP address (without prefix), MAC address,
	// and IP record ID for the customer with the given code.
	// Returns an error if no IP is found.
	GetIPByCustomerCode(ctx context.Context, customerCode string) (ip, mac string, id int, err error)

	// AllocateIP claims the next available IP for the given zone (location) and
	// tags it with customerCode as description. Tries prefixes for the zone in
	// priority order; moves to the next prefix if one is full.
	// Returns the allocated IP (without prefix length) and the NetBox record ID.
	AllocateIP(ctx context.Context, customerCode, zone string) (ip string, id int, err error)

	// UpdateARPStatus writes the arp_status custom field to the IP record.
	// status should be "active" or "disabled".
	UpdateARPStatus(ctx context.Context, ipID int, status string) error
}
