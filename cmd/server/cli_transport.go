package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// transport sends an RPC request and decodes the JSON response into resp.
// subject: e.g. "customer.list" (without the "isp.rpc." prefix)
type transport interface {
	do(ctx context.Context, subject string, req, resp any) error
}

// envelope matches the {"data": ..., "error": "..."} shape used by both NATS RPC and HTTP RPC.
type rpcEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

// ── NATS transport ─────────────────────────────────────────────────────────

type natsTransport struct {
	nc *nats.Conn
}

func newNATSTransport(natsURL string) (*natsTransport, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("connect NATS: %w", err)
	}
	return &natsTransport{nc: nc}, nil
}

func (t *natsTransport) do(ctx context.Context, subject string, req, resp any) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	msg, err := t.nc.RequestWithContext(ctx, "isp.rpc."+subject, payload)
	if err != nil {
		return fmt.Errorf("NATS request: %w", err)
	}
	return decodeEnvelope(msg.Data, resp)
}

// ── HTTP transport ─────────────────────────────────────────────────────────

type httpTransport struct {
	baseURL string
	token   string
	client  *http.Client
}

func newHTTPTransport(baseURL, token string) *httpTransport {
	return &httpTransport{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *httpTransport) do(ctx context.Context, subject string, req, resp any) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	url := t.baseURL + "/api/v1/rpc/" + subject
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+t.token)

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}
	return decodeEnvelope(body, resp)
}

// ── shared ─────────────────────────────────────────────────────────────────

func decodeEnvelope(data []byte, resp any) error {
	var env rpcEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if env.Error != "" {
		return fmt.Errorf("%s", env.Error)
	}
	if resp != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, resp)
	}
	return nil
}

// newTransport creates the appropriate transport from CLI flags.
// Prefers NATS when --nats-url is set; falls back to HTTP when --api-url is set.
func newTransport(natsURL, apiURL, apiToken string) (transport, error) {
	if natsURL != "" {
		return newNATSTransport(natsURL)
	}
	if apiURL != "" {
		if apiToken == "" {
			return nil, fmt.Errorf("--api-token is required when using --api-url")
		}
		return newHTTPTransport(apiURL, apiToken), nil
	}
	return nil, fmt.Errorf("provide --nats-url or --api-url to connect to the server")
}

// printJSON writes v as pretty-printed JSON to stdout.
func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
