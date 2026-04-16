package domain

import "context"

// CustomerARPData is the minimal customer information the network module
// needs to provision ARP entries. Defined here so the network module does
// not import the customer domain package.
type CustomerARPData struct {
	ID          string
	Code        string
	RouterID    *string
	IPAddress   string
	MACAddress  string
	NetworkZone string
}

// HasNetworkProvisioning reports whether the customer has a router assigned.
func (c CustomerARPData) HasNetworkProvisioning() bool {
	return c.RouterID != nil && *c.RouterID != ""
}

// CustomerARPProvider is the port the network module uses to read and update
// customer network information. Implemented by a bridge in the composition root.
type CustomerARPProvider interface {
	// FindARPData returns the ARP-relevant data for a customer.
	FindARPData(ctx context.Context, id string) (CustomerARPData, error)
	// UpdateNetworkInfo writes back resolved IP and MAC after IPAM lookup.
	UpdateNetworkInfo(ctx context.Context, id, routerID, ip, mac string) error
}
