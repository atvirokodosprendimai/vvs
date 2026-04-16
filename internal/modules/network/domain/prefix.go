package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrPrefixNotFound = errors.New("prefix not found")

// NetBoxPrefix represents a NetBox IP prefix managed by VVS for IP allocation.
// Multiple prefixes can share a location — when the first is full, the next is tried.
type NetBoxPrefix struct {
	ID        string
	NetBoxID  int    // NetBox prefix primary key (integer)
	CIDR      string // display only, e.g. "10.0.1.0/24"
	Location  string // e.g. "Kaunas" — used to match customer NetworkZone
	Priority  int    // lower = tried first within a location
	CreatedAt time.Time
}

func NewNetBoxPrefix(netboxID int, cidr, location string, priority int) (*NetBoxPrefix, error) {
	if netboxID <= 0 {
		return nil, errors.New("netbox prefix ID must be positive")
	}
	cidr = strings.TrimSpace(cidr)
	location = strings.TrimSpace(location)
	if location == "" {
		return nil, errors.New("location is required")
	}
	return &NetBoxPrefix{
		ID:        uuid.Must(uuid.NewV7()).String(),
		NetBoxID:  netboxID,
		CIDR:      cidr,
		Location:  location,
		Priority:  priority,
		CreatedAt: time.Now().UTC(),
	}, nil
}
