package mikrotik

import (
	"context"
	"fmt"
	"sync"

	routeros "github.com/go-routeros/routeros"

	"github.com/vvs/isp/internal/modules/network/domain"
)

// routerosConn is a narrow interface over the RouterOS client for testability.
// Returns rows as plain maps so tests never need to import the proto sub-package.
type routerosConn interface {
	RunArgs(sentence []string) ([]map[string]string, error)
	Close()
}

// realConn wraps *routeros.Client and adapts it to routerosConn.
type realConn struct{ c *routeros.Client }

func (r *realConn) RunArgs(sentence []string) ([]map[string]string, error) {
	reply, err := r.c.RunArgs(sentence)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]string, len(reply.Re))
	for i, s := range reply.Re {
		rows[i] = s.Map
	}
	return rows, nil
}

func (r *realConn) Close() { r.c.Close() }

// dialFunc signature — inject a fake in tests.
type dialFunc func(addr, username, password string) (routerosConn, error)

// defaultDial dials a real RouterOS device.
func defaultDial(addr, username, password string) (routerosConn, error) {
	c, err := routeros.Dial(addr, username, password)
	if err != nil {
		return nil, err
	}
	return &realConn{c}, nil
}

// Client implements domain.RouterProvisioner for MikroTik RouterOS.
// It maintains one persistent TCP connection per router (pooled by RouterID).
type Client struct {
	mu    sync.Mutex
	conns map[string]routerosConn
	dial  dialFunc
}

// New returns a production MikroTik Client.
func New() *Client {
	return &Client{
		conns: make(map[string]routerosConn),
		dial:  defaultDial,
	}
}

// newWithDial creates a Client with an injected dial function (testing).
func newWithDial(fn dialFunc) *Client {
	return &Client{
		conns: make(map[string]routerosConn),
		dial:  fn,
	}
}

func (c *Client) conn(r domain.RouterConn) (routerosConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if rc, ok := c.conns[r.RouterID]; ok {
		return rc, nil
	}
	port := r.Port
	if port == 0 {
		port = 8728
	}
	rc, err := c.dial(fmt.Sprintf("%s:%d", r.Host, port), r.Username, r.Password)
	if err != nil {
		return nil, fmt.Errorf("mikrotik dial %s: %w", r.Host, err)
	}
	c.conns[r.RouterID] = rc
	return rc, nil
}

func (c *Client) evict(routerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rc, ok := c.conns[routerID]; ok {
		rc.Close()
		delete(c.conns, routerID)
	}
}

// GetARPEntry returns the ARP entry for ip, or nil if not found.
func (c *Client) GetARPEntry(_ context.Context, r domain.RouterConn, ip string) (*domain.ARPEntry, error) {
	rc, err := c.conn(r)
	if err != nil {
		return nil, err
	}

	rows, err := rc.RunArgs([]string{"/ip/arp/print", "?address=" + ip})
	if err != nil {
		c.evict(r.RouterID)
		return nil, fmt.Errorf("arp print: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	m := rows[0]
	return &domain.ARPEntry{
		ID:         m[".id"],
		IPAddress:  m["address"],
		MACAddress: m["mac-address"],
		Interface:  m["interface"],
		Disabled:   m["disabled"] == "true",
		Static:     m["dynamic"] != "true",
	}, nil
}

// SetARPStatic creates or re-enables a static ARP entry for ip+mac.
func (c *Client) SetARPStatic(ctx context.Context, r domain.RouterConn, ip, mac, customerID string) error {
	rc, err := c.conn(r)
	if err != nil {
		return err
	}

	existing, err := c.GetARPEntry(ctx, r, ip)
	if err != nil {
		return err
	}

	comment := "vvs-" + customerID

	if existing != nil {
		_, err = rc.RunArgs([]string{
			"/ip/arp/set",
			"=.id=" + existing.ID,
			"=disabled=no",
			"=mac-address=" + mac,
			"=comment=" + comment,
		})
		if err != nil {
			c.evict(r.RouterID)
			return fmt.Errorf("arp set (enable): %w", err)
		}
		return nil
	}

	_, err = rc.RunArgs([]string{
		"/ip/arp/add",
		"=address=" + ip,
		"=mac-address=" + mac,
		"=interface=bridge",
		"=comment=" + comment,
	})
	if err != nil {
		c.evict(r.RouterID)
		return fmt.Errorf("arp add: %w", err)
	}
	return nil
}

// DisableARP disables the ARP entry for ip (cuts L2 access).
func (c *Client) DisableARP(ctx context.Context, r domain.RouterConn, ip string) error {
	rc, err := c.conn(r)
	if err != nil {
		return err
	}

	existing, err := c.GetARPEntry(ctx, r, ip)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}

	_, err = rc.RunArgs([]string{
		"/ip/arp/set",
		"=.id=" + existing.ID,
		"=disabled=yes",
	})
	if err != nil {
		c.evict(r.RouterID)
		return fmt.Errorf("arp set (disable): %w", err)
	}
	return nil
}
