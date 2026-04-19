package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// STBNATSClient calls isp.stb.rpc.* subjects on vvs-core.
// Runs in vvs-stb — no DB, NATS client only.
type STBNATSClient struct {
	nc      *nats.Conn
	timeout time.Duration
}

// NewSTBNATSClient creates a client. timeout 0 → 5s default.
func NewSTBNATSClient(nc *nats.Conn, timeout time.Duration) *STBNATSClient {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &STBNATSClient{nc: nc, timeout: timeout}
}

// ── KeyValidateResult ─────────────────────────────────────────────────────────

type KeyValidateResult struct {
	CustomerID     string `json:"customerID"`
	SubscriptionID string `json:"subscriptionID"`
	PackageID      string `json:"packageID"`
	Active         bool   `json:"active"`
}

func (c *STBNATSClient) ValidateKey(ctx context.Context, token string) (*KeyValidateResult, error) {
	var resp KeyValidateResult
	if err := c.rpc(ctx, SubjectKeyValidate, map[string]string{"token": token}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ── Playlist ──────────────────────────────────────────────────────────────────

type ChannelItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	LogoURL   string `json:"logoURL"`
	EPGSource string `json:"epgSource"`
	Category  string `json:"category"`
}

type PlaylistResult struct {
	Channels []ChannelItem `json:"channels"`
}

func (c *STBNATSClient) GetPlaylist(ctx context.Context, token string) (*PlaylistResult, error) {
	var resp PlaylistResult
	if err := c.rpc(ctx, SubjectPlaylistGet, map[string]string{"token": token}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ── EPG ───────────────────────────────────────────────────────────────────────

func (c *STBNATSClient) GetEPG(ctx context.Context, token string, days int) (string, error) {
	var resp struct {
		XMLTV string `json:"xmltv"`
	}
	if err := c.rpc(ctx, SubjectEPGGet, map[string]any{"token": token, "days": days}, &resp); err != nil {
		return "", err
	}
	return resp.XMLTV, nil
}

// ── Channel resolve ───────────────────────────────────────────────────────────

func (c *STBNATSClient) ResolveChannel(ctx context.Context, token, channelID string) (string, error) {
	var resp struct {
		StreamURL string `json:"streamURL"`
	}
	if err := c.rpc(ctx, SubjectChannelResolve, map[string]string{"token": token, "channelID": channelID}, &resp); err != nil {
		return "", err
	}
	return resp.StreamURL, nil
}

// ── rpc helper ────────────────────────────────────────────────────────────────

func (c *STBNATSClient) rpc(ctx context.Context, subject string, req any, out any) error {
	b, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("stb rpc: marshal: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	msg, err := c.nc.RequestMsgWithContext(tctx, &nats.Msg{Subject: subject, Data: b})
	if err != nil {
		return fmt.Errorf("stb rpc %s: %w", subject, err)
	}
	var env struct {
		Data  json.RawMessage `json:"data,omitempty"`
		Error string          `json:"error,omitempty"`
	}
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		return fmt.Errorf("stb rpc %s: unmarshal envelope: %w", subject, err)
	}
	if env.Error != "" {
		return fmt.Errorf("stb rpc %s: %s", subject, env.Error)
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("stb rpc %s: unmarshal data: %w", subject, err)
		}
	}
	return nil
}
