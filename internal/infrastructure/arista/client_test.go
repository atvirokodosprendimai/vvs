package arista

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vvs/isp/internal/modules/network/domain"
)

// rewriteDoer is an httpDoer that rewrites every request's URL to a fixed target,
// then forwards to the inner client. Used so tests can hit httptest.Server
// regardless of what host/port was built into the URL.
type rewriteDoer struct {
	target string // e.g. "http://127.0.0.1:PORT"
	inner  *http.Client
}

func (d *rewriteDoer) Do(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = strings.TrimPrefix(d.target, "http://")
	return d.inner.Do(r2)
}

// fakeClient returns a Client whose every request is routed to srv.
func fakeClient(srv *httptest.Server) *Client {
	return newWithHTTP(&rewriteDoer{target: srv.URL, inner: srv.Client()})
}

// dummyConn is a RouterConn with arbitrary fields (host/port are rewritten by rewriteDoer).
func dummyConn() domain.RouterConn {
	return domain.RouterConn{
		RouterID: "r1",
		Host:     "arista.example.local",
		Port:     443,
		Username: "admin",
		Password: "secret",
	}
}

// buildEAPIResponse wraps result into an eAPI JSON-RPC response with one entry.
func buildEAPIResponse(t *testing.T, result interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	resp := eapiResponse{Result: []json.RawMessage{raw}}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return b
}

func buildErrorResponse(code int, message string) []byte {
	resp := map[string]interface{}{
		"result": []interface{}{},
		"error":  map[string]interface{}{"code": code, "message": message},
	}
	b, _ := json.Marshal(resp)
	return b
}

func buildTextResponse() []byte {
	// "text" format results are plain strings, not objects.
	resp := map[string]interface{}{"result": []string{"", "", ""}}
	b, _ := json.Marshal(resp)
	return b
}

func staticEntry(ip, mac, iface string) arpTableEntry {
	return arpTableEntry{Address: ip, HWAddress: mac, Interface: iface, EntryType: "static"}
}

func dynamicEntry(ip, mac, iface string) arpTableEntry {
	return arpTableEntry{Address: ip, HWAddress: mac, Interface: iface, EntryType: "dynamic"}
}

func arpResult(entries ...arpTableEntry) showARPResult {
	r := showARPResult{}
	r.ARPTable.TableEntry = entries
	return r
}

// -- GetARPEntry --

func TestGetARPEntry_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEAPIResponse(t, arpResult(staticEntry("10.0.0.55", "AA:BB:CC:DD:EE:FF", "Ethernet1"))))
	}))
	defer srv.Close()

	got, err := fakeClient(srv).GetARPEntry(context.Background(), dummyConn(), "10.0.0.55")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "10.0.0.55", got.IPAddress)
	assert.Equal(t, "aa:bb:cc:dd:ee:ff", got.MACAddress) // normalised lower-case
	assert.Equal(t, "Ethernet1", got.Interface)
	assert.True(t, got.Static)
	assert.False(t, got.Disabled)
}

func TestGetARPEntry_NotFound_EmptyTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEAPIResponse(t, arpResult()))
	}))
	defer srv.Close()

	got, err := fakeClient(srv).GetARPEntry(context.Background(), dummyConn(), "10.0.0.1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestGetARPEntry_IPMismatch_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// server returns 10.0.0.99 but we query for 10.0.0.55
		w.Write(buildEAPIResponse(t, arpResult(staticEntry("10.0.0.99", "AA:BB:CC:DD:EE:FF", "Ethernet1"))))
	}))
	defer srv.Close()

	got, err := fakeClient(srv).GetARPEntry(context.Background(), dummyConn(), "10.0.0.55")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestGetARPEntry_DynamicEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEAPIResponse(t, arpResult(dynamicEntry("10.0.0.55", "AA:BB:CC:DD:EE:FF", "Ethernet1"))))
	}))
	defer srv.Close()

	got, err := fakeClient(srv).GetARPEntry(context.Background(), dummyConn(), "10.0.0.55")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.Static)
}

func TestGetARPEntry_CommandSent(t *testing.T) {
	var capturedCmd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eapiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if len(req.Params.Cmds) > 0 {
			capturedCmd = req.Params.Cmds[0]
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEAPIResponse(t, arpResult()))
	}))
	defer srv.Close()

	_, _ = fakeClient(srv).GetARPEntry(context.Background(), dummyConn(), "10.0.0.55")
	assert.Equal(t, "show arp 10.0.0.55", capturedCmd)
}

// -- SetARPStatic --

func TestSetARPStatic_SendsCorrectCommands(t *testing.T) {
	var capturedCmds []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req eapiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		capturedCmds = req.Params.Cmds
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildTextResponse())
	}))
	defer srv.Close()

	err := fakeClient(srv).SetARPStatic(context.Background(), dummyConn(), "10.0.0.55", "aa:bb:cc:dd:ee:ff", "cust-123")
	require.NoError(t, err)

	require.Len(t, capturedCmds, 3)
	assert.Equal(t, "configure", capturedCmds[0])
	assert.Equal(t, "arp 10.0.0.55 aa:bb:cc:dd:ee:ff arpa", capturedCmds[1])
	assert.Equal(t, "end", capturedCmds[2])
}

func TestSetARPStatic_BasicAuthSent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "BasicAuth header must be present")
		assert.Equal(t, "admin", user)
		assert.Equal(t, "secret", pass)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildTextResponse())
	}))
	defer srv.Close()

	err := fakeClient(srv).SetARPStatic(context.Background(), dummyConn(), "10.0.0.55", "aa:bb:cc:dd:ee:ff", "cust-123")
	require.NoError(t, err)
}

// -- DisableARP --

func TestDisableARP_StaticEntry_Removed(t *testing.T) {
	callN := 0
	var capturedCmds []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callN++
		w.Header().Set("Content-Type", "application/json")
		if callN == 1 {
			// First call: GetARPEntry — return a static entry
			w.Write(buildEAPIResponse(t, arpResult(staticEntry("10.0.0.55", "AA:BB:CC:DD:EE:FF", "Ethernet1"))))
			return
		}
		// Second call: configure commands
		var req eapiRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		capturedCmds = req.Params.Cmds
		w.Write(buildTextResponse())
	}))
	defer srv.Close()

	err := fakeClient(srv).DisableARP(context.Background(), dummyConn(), "10.0.0.55")
	require.NoError(t, err)
	assert.Equal(t, 2, callN)
	require.Len(t, capturedCmds, 3)
	assert.Equal(t, "configure", capturedCmds[0])
	assert.Equal(t, "no arp 10.0.0.55", capturedCmds[1])
	assert.Equal(t, "end", capturedCmds[2])
}

func TestDisableARP_EntryAbsent_NoOp(t *testing.T) {
	callN := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callN++
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEAPIResponse(t, arpResult())) // empty table
	}))
	defer srv.Close()

	err := fakeClient(srv).DisableARP(context.Background(), dummyConn(), "10.0.0.55")
	require.NoError(t, err)
	assert.Equal(t, 1, callN, "only GetARPEntry; no configure commands")
}

func TestDisableARP_DynamicEntry_NoOp(t *testing.T) {
	callN := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callN++
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEAPIResponse(t, arpResult(dynamicEntry("10.0.0.55", "AA:BB:CC:DD:EE:FF", "Ethernet1"))))
	}))
	defer srv.Close()

	err := fakeClient(srv).DisableARP(context.Background(), dummyConn(), "10.0.0.55")
	require.NoError(t, err)
	assert.Equal(t, 1, callN, "dynamic entry must not trigger remove")
}

// -- Error paths --

func TestEAPIError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildErrorResponse(1002, "invalid command"))
	}))
	defer srv.Close()

	_, err := fakeClient(srv).GetARPEntry(context.Background(), dummyConn(), "10.0.0.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1002")
	assert.Contains(t, err.Error(), "invalid command")
}

func TestHTTPNon200_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := fakeClient(srv).GetARPEntry(context.Background(), dummyConn(), "10.0.0.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestTransportError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close immediately — all dials will fail

	_, err := fakeClient(srv).GetARPEntry(context.Background(), dummyConn(), "10.0.0.1")
	require.Error(t, err)
}
