// Package proxmox implements the domain.VMProvisioner port for Proxmox VE.
// All operations communicate with the Proxmox REST API at /api2/json.
// Authentication uses API tokens: PVEAPIToken=USER@REALM!TOKENID=SECRET.
package proxmox

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	proxmoxdomain "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
)

// Client is a stateless Proxmox VE REST API client.
// It satisfies the domain.VMProvisioner interface.
// TLS verification is configured per-call via NodeConn.InsecureTLS.
type Client struct {
	secure    *http.Client // TLS-verifying client
	insecure  *http.Client // InsecureSkipVerify client (self-signed certs)
}

// New returns a Client. Both TLS variants are initialised lazily on first use.
func New() *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // operator opt-in per node
	}
	return &Client{
		secure: &http.Client{Timeout: 30 * time.Second},
		insecure: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (c *Client) httpClient(conn proxmoxdomain.NodeConn) *http.Client {
	if conn.InsecureTLS {
		return c.insecure
	}
	return c.secure
}

func (c *Client) baseURL(conn proxmoxdomain.NodeConn) string {
	return fmt.Sprintf("https://%s:%d/api2/json", conn.Host, conn.Port)
}

func (c *Client) authHeader(conn proxmoxdomain.NodeConn) string {
	return fmt.Sprintf("PVEAPIToken=%s!%s=%s", conn.User, conn.TokenID, conn.TokenSecret)
}

// apiResponse is the envelope Proxmox wraps all responses in.
type apiResponse struct {
	Data json.RawMessage `json:"data"`
}

func (c *Client) do(ctx context.Context, conn proxmoxdomain.NodeConn, method, path string, body any) (json.RawMessage, error) {
	url := c.baseURL(conn) + path

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("proxmox: marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("proxmox: build request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader(conn))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient(conn).Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxmox: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("proxmox: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("proxmox: %s %s → HTTP %d: %s", method, path, resp.StatusCode, raw)
	}

	var envelope apiResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("proxmox: unmarshal envelope: %w", err)
	}
	return envelope.Data, nil
}

// NextVMID returns the next available VMID from the Proxmox cluster.
func (c *Client) NextVMID(ctx context.Context, conn proxmoxdomain.NodeConn) (int, error) {
	data, err := c.do(ctx, conn, http.MethodGet, "/cluster/nextid", nil)
	if err != nil {
		return 0, err
	}
	// Response data is a quoted integer string, e.g. "101"
	var idStr string
	if err := json.Unmarshal(data, &idStr); err != nil {
		// Try direct int
		var id int
		if err2 := json.Unmarshal(data, &id); err2 != nil {
			return 0, fmt.Errorf("proxmox: parse nextid: %w", err)
		}
		return id, nil
	}
	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		return 0, fmt.Errorf("proxmox: parse nextid %q: %w", idStr, err)
	}
	return id, nil
}
