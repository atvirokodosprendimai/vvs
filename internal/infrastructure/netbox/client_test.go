package netbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIPByCustomerCode_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/ipam/ip-addresses/", r.URL.Path)
		assert.Equal(t, "CLI-00001", r.URL.Query().Get("description"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"count": 1,
			"results": [{
				"id": 42,
				"address": "10.0.1.55/24",
				"assigned_object": {
					"id": 7,
					"object_type": "dcim.interface",
					"mac_address": "AA:BB:CC:DD:EE:FF"
				}
			}]
		}`))
	}))
	defer srv.Close()

	c := newWithHTTP(srv.URL, "test-token", srv.Client())
	ip, mac, id, err := c.GetIPByCustomerCode(context.Background(), "CLI-00001")
	require.NoError(t, err)
	assert.Equal(t, "10.0.1.55", ip)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", mac)
	assert.Equal(t, 42, id)
}

func TestGetIPByCustomerCode_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"results":[]}`))
	}))
	defer srv.Close()

	c := newWithHTTP(srv.URL, "tok", srv.Client())
	_, _, _, err := c.GetIPByCustomerCode(context.Background(), "CLI-99999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no IP found")
}

func TestGetIPByCustomerCode_NoMACOnInterface(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"count": 1,
			"results": [{
				"id": 5,
				"address": "10.0.2.10/24",
				"assigned_object": null
			}]
		}`))
	}))
	defer srv.Close()

	c := newWithHTTP(srv.URL, "tok", srv.Client())
	ip, mac, id, err := c.GetIPByCustomerCode(context.Background(), "CLI-00002")
	require.NoError(t, err)
	assert.Equal(t, "10.0.2.10", ip)
	assert.Equal(t, "", mac) // no interface → empty MAC
	assert.Equal(t, 5, id)
}

func TestUpdateARPStatus_OK(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/ipam/ip-addresses/42/", r.URL.Path)
		assert.Equal(t, http.MethodPatch, r.Method)
		b := make([]byte, 256)
		n, _ := r.Body.Read(b)
		capturedBody = string(b[:n])
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newWithHTTP(srv.URL, "tok", srv.Client())
	err := c.UpdateARPStatus(context.Background(), 42, "active")
	require.NoError(t, err)
	assert.Contains(t, capturedBody, `"arp_status"`)
	assert.Contains(t, capturedBody, `"active"`)
}

func TestUpdateARPStatus_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"authentication credentials not provided"}`))
	}))
	defer srv.Close()

	c := newWithHTTP(srv.URL, "bad-token", srv.Client())
	err := c.UpdateARPStatus(context.Background(), 1, "active")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestGetIPByCustomerCode_URLEncodesCustomerCode(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"results":[]}`))
	}))
	defer srv.Close()

	c := newWithHTTP(srv.URL, "tok", srv.Client())
	// customer code with a space — must be percent-encoded, not raw
	_, _, _, _ = c.GetIPByCustomerCode(context.Background(), "CLI 00001")
	if gotQuery != "description=CLI+00001&limit=1" && gotQuery != "description=CLI%2000001&limit=1" {
		t.Fatalf("want URL-encoded description, got %q", gotQuery)
	}
}

func TestGetIPByCustomerCode_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	c := newWithHTTP(srv.URL, "tok", srv.Client())
	_, _, _, err := c.GetIPByCustomerCode(context.Background(), "CLI-00001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
