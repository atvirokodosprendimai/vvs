package mikrotik

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vvs/isp/internal/modules/network/domain"
)

// fakeConn is a controllable routerosConn for tests.
type fakeConn struct {
	runFunc func(sentence []string) ([]map[string]string, error)
	calls   [][]string
	closed  bool
}

func (f *fakeConn) RunArgs(sentence []string) ([]map[string]string, error) {
	f.calls = append(f.calls, sentence)
	if f.runFunc != nil {
		return f.runFunc(sentence)
	}
	return nil, nil
}

func (f *fakeConn) Close() { f.closed = true }

func testConn() domain.RouterConn {
	return domain.RouterConn{
		RouterID: "r1",
		Host:     "192.168.1.1",
		Port:     8728,
		Username: "admin",
		Password: "secret",
	}
}

func fakeClient(conn *fakeConn) *Client {
	return newWithDial(func(addr, user, pass string) (routerosConn, error) {
		return conn, nil
	})
}

// arpRow returns a single ARP row map with sensible defaults.
func arpRow(overrides map[string]string) map[string]string {
	m := map[string]string{
		".id":         "*1",
		"address":     "10.0.0.55",
		"mac-address": "AA:BB:CC:DD:EE:FF",
		"interface":   "bridge",
		"disabled":    "false",
		"dynamic":     "false",
	}
	for k, v := range overrides {
		m[k] = v
	}
	return m
}

// -- GetARPEntry --

func TestGetARPEntry_NotFound(t *testing.T) {
	fake := &fakeConn{}
	c := fakeClient(fake)

	entry, err := c.GetARPEntry(context.Background(), testConn(), "10.0.0.1")
	require.NoError(t, err)
	assert.Nil(t, entry)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, []string{"/ip/arp/print", "?address=10.0.0.1"}, fake.calls[0])
}

func TestGetARPEntry_Found(t *testing.T) {
	fake := &fakeConn{
		runFunc: func(s []string) ([]map[string]string, error) {
			return []map[string]string{arpRow(nil)}, nil
		},
	}
	c := fakeClient(fake)

	entry, err := c.GetARPEntry(context.Background(), testConn(), "10.0.0.55")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "*1", entry.ID)
	assert.Equal(t, "10.0.0.55", entry.IPAddress)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", entry.MACAddress)
	assert.False(t, entry.Disabled)
	assert.True(t, entry.Static)
}

// -- SetARPStatic --

func TestSetARPStatic_NewEntry(t *testing.T) {
	callN := 0
	fake := &fakeConn{
		runFunc: func(s []string) ([]map[string]string, error) {
			callN++
			return nil, nil // first call = GetARPEntry (empty = not found); second = add
		},
	}
	c := fakeClient(fake)

	err := c.SetARPStatic(context.Background(), testConn(), "10.0.0.55", "AA:BB:CC:DD:EE:FF", "cust-123")
	require.NoError(t, err)
	require.Len(t, fake.calls, 2)
	assert.Contains(t, fake.calls[1], "/ip/arp/add")
	assert.Contains(t, fake.calls[1], "=address=10.0.0.55")
	assert.Contains(t, fake.calls[1], "=mac-address=AA:BB:CC:DD:EE:FF")
	assert.Contains(t, fake.calls[1], "=comment=vvs-cust-123")
}

func TestSetARPStatic_ExistingDisabledEntry(t *testing.T) {
	callN := 0
	fake := &fakeConn{
		runFunc: func(s []string) ([]map[string]string, error) {
			callN++
			if callN == 1 {
				return []map[string]string{arpRow(map[string]string{"disabled": "true"})}, nil
			}
			return nil, nil
		},
	}
	c := fakeClient(fake)

	err := c.SetARPStatic(context.Background(), testConn(), "10.0.0.55", "AA:BB:CC:DD:EE:FF", "cust-123")
	require.NoError(t, err)
	require.Len(t, fake.calls, 2)
	// second call must be /ip/arp/set with disabled=no
	assert.Contains(t, fake.calls[1], "/ip/arp/set")
	assert.Contains(t, fake.calls[1], "=disabled=no")
	assert.Contains(t, fake.calls[1], "=.id=*1")
}

// -- DisableARP --

func TestDisableARP_Found(t *testing.T) {
	callN := 0
	fake := &fakeConn{
		runFunc: func(s []string) ([]map[string]string, error) {
			callN++
			if callN == 1 {
				return []map[string]string{arpRow(map[string]string{".id": "*2"})}, nil
			}
			return nil, nil
		},
	}
	c := fakeClient(fake)

	err := c.DisableARP(context.Background(), testConn(), "10.0.0.55")
	require.NoError(t, err)
	require.Len(t, fake.calls, 2)
	assert.Contains(t, fake.calls[1], "=disabled=yes")
	assert.Contains(t, fake.calls[1], "=.id=*2")
}

func TestDisableARP_NotFound(t *testing.T) {
	fake := &fakeConn{}
	c := fakeClient(fake)

	err := c.DisableARP(context.Background(), testConn(), "10.0.0.99")
	require.NoError(t, err)
	assert.Len(t, fake.calls, 1) // only GetARPEntry, no set
}

// -- Connection pool --

func TestConnectionPool_ReuseConn(t *testing.T) {
	dialCount := 0
	fake := &fakeConn{}
	c := newWithDial(func(addr, user, pass string) (routerosConn, error) {
		dialCount++
		return fake, nil
	})

	conn := testConn()
	_, _ = c.GetARPEntry(context.Background(), conn, "10.0.0.1")
	_, _ = c.GetARPEntry(context.Background(), conn, "10.0.0.2")
	assert.Equal(t, 1, dialCount, "should dial once and reuse connection")
}

func TestDialError(t *testing.T) {
	c := newWithDial(func(addr, user, pass string) (routerosConn, error) {
		return nil, errors.New("connection refused")
	})

	_, err := c.GetARPEntry(context.Background(), testConn(), "10.0.0.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mikrotik dial")
}

func TestEvict_OnRunError(t *testing.T) {
	fake := &fakeConn{
		runFunc: func(s []string) ([]map[string]string, error) {
			return nil, errors.New("broken pipe")
		},
	}
	c := fakeClient(fake)

	_, err := c.GetARPEntry(context.Background(), testConn(), "10.0.0.1")
	require.Error(t, err)
	assert.True(t, fake.closed, "evict should close the bad connection")
}
