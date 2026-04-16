package netbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// IPRecord holds the fields VVS cares about from a NetBox IP address object.
type IPRecord struct {
	ID         int    `json:"id"`
	Address    string `json:"address"` // e.g. "10.0.1.55/24"
	MACAddress string // populated by following assigned_object
}

// Client is a minimal NetBox REST client.
type Client struct {
	baseURL  string
	token    string
	prefixID int // NetBox prefix PK to allocate IPs from; 0 = disabled
	http     httpDoer
}

// httpDoer allows injecting a fake HTTP client in tests.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// New creates a NetBox client.
// prefixID is the NetBox prefix PK used for IP allocation; 0 disables allocation.
func New(baseURL, token string, prefixID int) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		token:    token,
		prefixID: prefixID,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// newWithHTTP creates a Client with an injected HTTP doer (testing).
func newWithHTTP(baseURL, token string, h httpDoer) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    h,
	}
}

// GetIPByCustomerCode searches NetBox for an IP address whose description
// contains customerCode (e.g. "CLI-00001") and returns IP (without prefix),
// MAC address, and the NetBox IP record ID.
//
// MAC address lives on the interface that the IP is assigned to, so the client
// follows the assigned_object reference automatically.
func (c *Client) GetIPByCustomerCode(ctx context.Context, customerCode string) (ip, mac string, id int, err error) {
	type assignedObject struct {
		ID          int    `json:"id"`
		ObjectType  string `json:"object_type"` // "dcim.interface" or "virtualization.vminterface"
		MACAddress  string `json:"mac_address"`
	}
	type ipRecord struct {
		ID             int             `json:"id"`
		Address        string          `json:"address"`
		AssignedObject *assignedObject `json:"assigned_object"`
	}
	type listResult struct {
		Count   int        `json:"count"`
		Results []ipRecord `json:"results"`
	}

	endpoint := fmt.Sprintf("%s/api/ipam/ip-addresses/?description=%s&limit=1", c.baseURL, url.QueryEscape(customerCode))
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return "", "", 0, fmt.Errorf("netbox ip search %s: %w", customerCode, err)
	}

	var result listResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", 0, fmt.Errorf("netbox ip decode: %w", err)
	}
	if result.Count == 0 || len(result.Results) == 0 {
		return "", "", 0, fmt.Errorf("netbox: no IP found for customer %s", customerCode)
	}

	rec := result.Results[0]

	// Strip prefix length from address ("10.0.1.55/24" → "10.0.1.55")
	rawIP := rec.Address
	if idx := strings.Index(rawIP, "/"); idx != -1 {
		rawIP = rawIP[:idx]
	}

	// MAC lives on assigned_object (interface), not the IP record itself
	macAddr := ""
	if rec.AssignedObject != nil {
		macAddr = rec.AssignedObject.MACAddress
	}

	return rawIP, macAddr, rec.ID, nil
}

// AllocateIP claims the next available IP from the configured prefix and sets
// customerCode as the description. Returns the IP (without prefix) and record ID.
// Returns an error if no prefix is configured (prefixID == 0).
func (c *Client) AllocateIP(ctx context.Context, customerCode string) (ip string, id int, err error) {
	if c.prefixID == 0 {
		return "", 0, fmt.Errorf("netbox: no prefix configured for IP allocation (set NETBOX_PREFIX_ID)")
	}

	endpoint := fmt.Sprintf("%s/api/ipam/prefixes/%d/available-ips/", c.baseURL, c.prefixID)
	desc, _ := json.Marshal(customerCode)
	payload := fmt.Sprintf(`{"description":%s,"status":"active"}`, string(desc))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("netbox allocate ip: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("netbox allocate ip %d: %s", resp.StatusCode, string(b))
	}

	var rec struct {
		ID      int    `json:"id"`
		Address string `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		return "", 0, fmt.Errorf("netbox allocate ip decode: %w", err)
	}

	rawIP := rec.Address
	if idx := strings.Index(rawIP, "/"); idx != -1 {
		rawIP = rawIP[:idx]
	}
	return rawIP, rec.ID, nil
}

// UpdateARPStatus writes the arp_status custom field back to the NetBox IP record.
// status should be "active" or "disabled".
func (c *Client) UpdateARPStatus(ctx context.Context, ipID int, status string) error {
	url := fmt.Sprintf("%s/api/ipam/ip-addresses/%d/", c.baseURL, ipID)
	payload := fmt.Sprintf(`{"custom_fields":{"arp_status":%q}}`, status)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, strings.NewReader(payload))
	if err != nil {
		return err
	}
	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("netbox arp_status patch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("netbox arp_status patch %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// get performs a GET request and returns the response body.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
}
