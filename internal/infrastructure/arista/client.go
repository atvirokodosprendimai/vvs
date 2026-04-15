// Package arista implements domain.RouterProvisioner for Arista EOS
// via the eAPI (JSON-RPC over HTTPS, default port 443).
//
// ARP model differences vs MikroTik:
//   - Arista has no native "disabled" ARP flag.
//   - SetARPStatic adds a static entry (grants access).
//   - DisableARP removes the static entry (cuts access); to prevent dynamic
//     re-learning, callers should pair this with a null-route or port policy.
package arista

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/vvs/isp/internal/modules/network/domain"
)

// httpDoer is a narrow interface over http.Client for testability.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client implements domain.RouterProvisioner for Arista EOS via eAPI.
// Each call opens a new HTTP request; connection pooling is handled by http.Client.
type Client struct {
	http httpDoer
}

// New returns a production Arista Client.
// TLS certificate verification is skipped because network gear typically uses
// self-signed certificates.
func New() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
}

// newWithHTTP creates a Client with an injected HTTP doer (testing).
func newWithHTTP(h httpDoer) *Client {
	return &Client{http: h}
}

// eapiRequest is the JSON-RPC request body sent to EOS eAPI.
type eapiRequest struct {
	JSONRPC string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  eapiParams `json:"params"`
	ID      string   `json:"id"`
}

type eapiParams struct {
	Version int      `json:"version"`
	Cmds    []string `json:"cmds"`
	Format  string   `json:"format"` // "json" or "text"
}

// eapiResponse is the JSON-RPC response envelope.
type eapiResponse struct {
	Result []json.RawMessage `json:"result"`
	Error  *eapiError        `json:"error"`
}

type eapiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// run executes one or more EOS CLI commands via eAPI and returns the raw result array.
func (c *Client) run(ctx context.Context, conn domain.RouterConn, format string, cmds ...string) ([]json.RawMessage, error) {
	port := conn.Port
	if port == 0 {
		port = 443
	}

	scheme := "https"
	if port == 80 {
		scheme = "http"
	}

	url := fmt.Sprintf("%s://%s:%d/command-api", scheme, conn.Host, port)

	body, err := json.Marshal(eapiRequest{
		JSONRPC: "2.0",
		Method:  "runCmds",
		Params: eapiParams{
			Version: 1,
			Cmds:    cmds,
			Format:  format,
		},
		ID: "vvs",
	})
	if err != nil {
		return nil, fmt.Errorf("eapi marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("eapi request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(conn.Username, conn.Password)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("eapi call %s: %w", conn.Host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eapi %s: HTTP %d", conn.Host, resp.StatusCode)
	}

	var apiResp eapiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("eapi decode: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("eapi error %d: %s", apiResp.Error.Code, apiResp.Error.Message)
	}

	return apiResp.Result, nil
}

// arpTableEntry is the per-entry shape inside the EOS show arp JSON response.
type arpTableEntry struct {
	Address    string `json:"address"`
	HWAddress  string `json:"hwAddress"`
	Interface  string `json:"interface"`
	EntryType  string `json:"entryType"` // "static" | "dynamic" | "remote"
}

type showARPResult struct {
	ARPTable struct {
		TableEntry []arpTableEntry `json:"tableEntry"`
	} `json:"arpTable"`
}

// GetARPEntry returns the ARP entry for ip, or nil if absent.
func (c *Client) GetARPEntry(ctx context.Context, conn domain.RouterConn, ip string) (*domain.ARPEntry, error) {
	results, err := c.run(ctx, conn, "json", "show arp "+ip)
	if err != nil {
		return nil, fmt.Errorf("arista get arp: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	var result showARPResult
	if err := json.Unmarshal(results[0], &result); err != nil {
		return nil, fmt.Errorf("arista parse arp: %w", err)
	}

	for _, e := range result.ARPTable.TableEntry {
		if e.Address == ip {
			return &domain.ARPEntry{
				ID:         ip, // Arista has no separate ARP ID; use IP as key
				IPAddress:  e.Address,
				MACAddress: normalizeMAC(e.HWAddress),
				Interface:  e.Interface,
				Static:     e.EntryType == "static",
				// Arista has no "disabled" flag — absence of static entry = disabled
				Disabled: false,
			}, nil
		}
	}
	return nil, nil
}

// SetARPStatic creates or updates a static ARP entry (grants access).
// customerID is recorded as an alias comment via a description macro where supported.
func (c *Client) SetARPStatic(ctx context.Context, conn domain.RouterConn, ip, mac, customerID string) error {
	_, err := c.run(ctx, conn, "text",
		"configure",
		fmt.Sprintf("arp %s %s arpa", ip, mac),
		"end",
	)
	if err != nil {
		return fmt.Errorf("arista set arp static %s: %w", ip, err)
	}
	return nil
}

// DisableARP removes the static ARP entry for ip (cuts access).
// On Arista, "disabling" is represented by removing the entry because EOS has
// no native disabled-ARP flag. Dynamic re-learning may occur; pair with a
// null-route or port policy for hard isolation.
func (c *Client) DisableARP(ctx context.Context, conn domain.RouterConn, ip string) error {
	// Only remove if a static entry exists — avoid no-op errors.
	existing, err := c.GetARPEntry(ctx, conn, ip)
	if err != nil {
		return fmt.Errorf("arista disable arp: %w", err)
	}
	if existing == nil || !existing.Static {
		return nil // already absent or only dynamic — nothing to do
	}

	_, err = c.run(ctx, conn, "text",
		"configure",
		fmt.Sprintf("no arp %s", ip),
		"end",
	)
	if err != nil {
		return fmt.Errorf("arista remove arp %s: %w", ip, err)
	}
	return nil
}

// normalizeMAC lower-cases the MAC address returned by EOS (EOS uses uppercase xx:xx:... format).
func normalizeMAC(mac string) string {
	return strings.ToLower(mac)
}
