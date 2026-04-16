package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/vvs/isp/internal/infrastructure/netbox"
	"github.com/vvs/isp/internal/modules/network/domain"
)

// IPAllocatorService tries each prefix for the given zone in priority order.
// Satisfies both domain.IPAMProvider (for SyncCustomerARPHandler)
// and the IPAllocator port used by CreateCustomerHandler.
type IPAllocatorService struct {
	prefixes domain.PrefixRepository
	client   *netbox.Client
}

func NewIPAllocatorService(prefixes domain.PrefixRepository, client *netbox.Client) *IPAllocatorService {
	return &IPAllocatorService{prefixes: prefixes, client: client}
}

// GetIPByCustomerCode searches NetBox for an existing IP by description.
func (s *IPAllocatorService) GetIPByCustomerCode(ctx context.Context, customerCode string) (ip, mac string, id int, err error) {
	return s.client.GetIPByCustomerCode(ctx, customerCode)
}

// AllocateIP claims the next available IP for the given zone.
// Tries prefixes in priority order; moves on if a prefix is full (non-2xx response).
// Falls back to any prefix if zone is empty.
func (s *IPAllocatorService) AllocateIP(ctx context.Context, customerCode, zone string) (ip string, id int, err error) {
	var prefixes []*domain.NetBoxPrefix
	if zone != "" {
		prefixes, err = s.prefixes.ListByLocation(ctx, zone)
	} else {
		prefixes, err = s.prefixes.ListAll(ctx)
	}
	if err != nil {
		return "", 0, fmt.Errorf("allocate ip: load prefixes: %w", err)
	}
	if len(prefixes) == 0 {
		if zone != "" {
			return "", 0, fmt.Errorf("allocate ip: no prefixes configured for zone %q", zone)
		}
		return "", 0, fmt.Errorf("allocate ip: no prefixes configured")
	}

	var errs []string
	for _, p := range prefixes {
		ip, id, err = s.client.AllocateFromPrefix(ctx, p.NetBoxID, customerCode)
		if err == nil {
			return ip, id, nil
		}
		errs = append(errs, fmt.Sprintf("prefix %d (%s): %v", p.NetBoxID, p.CIDR, err))
	}
	return "", 0, fmt.Errorf("allocate ip: all prefixes exhausted: %s", strings.Join(errs, "; "))
}

// UpdateARPStatus writes the arp_status custom field to the NetBox IP record.
func (s *IPAllocatorService) UpdateARPStatus(ctx context.Context, ipID int, status string) error {
	return s.client.UpdateARPStatus(ctx, ipID, status)
}
