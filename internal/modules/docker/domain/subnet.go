package domain

import (
	"encoding/binary"
	"fmt"
	"net"
)

// SplitSubnet splits a CIDR into two equal halves.
// Lower half = DHCP pool (Docker-managed), upper half = reserved range (VVS-managed).
// Returns start/end of each half as dotted-decimal strings (host addresses, not network/broadcast).
//
// Example: "10.100.0.0/17" →
//   dhcpStart="10.100.0.1", dhcpEnd="10.100.63.254"
//   reservedStart="10.100.64.0", reservedEnd="10.100.127.254"
func SplitSubnet(cidr string) (dhcpStart, dhcpEnd, reservedStart, reservedEnd string, err error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", "", "", fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	ones, bits := network.Mask.Size()
	if bits != 32 {
		return "", "", "", "", fmt.Errorf("only IPv4 CIDR supported")
	}
	if ones >= 31 {
		return "", "", "", "", fmt.Errorf("subnet too small to split (%d prefix)", ones)
	}

	// Base address as uint32
	base := binary.BigEndian.Uint32(network.IP.To4())
	size := uint32(1) << (32 - ones)
	half := size / 2

	dhcpNet := base + 1          // skip network address
	dhcpBroadcast := base + half - 1
	reservedNet := base + half
	reservedBroadcast := base + size - 2 // skip broadcast

	dhcpStart = intToIP(dhcpNet)
	dhcpEnd = intToIP(dhcpBroadcast)
	reservedStart = intToIP(reservedNet)
	reservedEnd = intToIP(reservedBroadcast)
	return
}

func intToIP(n uint32) string {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return net.IP(b).String()
}

// DHCPRangeCIDR returns the CIDR for the lower half of cidr — the DHCP pool boundary
// passed as --ip-range to Docker NetworkCreate to constrain auto-assignment.
// Returns empty string if cidr cannot be parsed.
func DHCPRangeCIDR(cidr string) string {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}
	ones, bits := network.Mask.Size()
	if bits != 32 || ones >= 31 {
		return ""
	}
	// Lower half has one extra bit set → prefix length = ones+1
	return fmt.Sprintf("%s/%d", network.IP.String(), ones+1)
}
