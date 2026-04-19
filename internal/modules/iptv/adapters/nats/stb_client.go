package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// STBDeviceAPI is the canonical 3-method STB device interface.
// Every STB app (Tivimate, Smarters, SIPTV, MAG, Formuler) uses this contract.
type STBDeviceAPI interface {
	// GetConfig authenticates the device and returns server config.
	// Pass token OR mac; one must be non-empty.
	GetConfig(ctx context.Context, token, mac string) (*ConfigResult, error)

	// GetChannel returns the live stream URL for the requested channel.
	GetChannel(ctx context.Context, token, channelID string) (string, error)

	// GetDVR returns the DVR playback URL for a channel starting at startAt UTC.
	GetDVR(ctx context.Context, token, channelID string, startAt time.Time) (string, error)
}

// Compile-time check: STBNATSClient implements STBDeviceAPI.
var _ STBDeviceAPI = (*STBNATSClient)(nil)

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

// EPGShortSlot is a single programme slot in the short EPG response.
type EPGShortSlot struct {
	Title     string `json:"title"`
	StartTime string `json:"start"`
	StopTime  string `json:"stop"`
}

// EPGShortEntry is current+next programme for one channel.
type EPGShortEntry struct {
	ChannelEPGID string        `json:"channelEPGID"`
	Current      *EPGShortSlot `json:"current,omitempty"`
	Next         *EPGShortSlot `json:"next,omitempty"`
}

// EPGDeviceAPI is the 2-method EPG interface for STB devices.
// token may be empty for public/unauthenticated EPG access.
type EPGDeviceAPI interface {
	// GetEPGShort returns the current+next programme for a channel.
	GetEPGShort(ctx context.Context, token, channelID string) (*EPGShortEntry, error)

	// GetEPG returns a XMLTV document for the given channel and date (YYYY-MM-DD).
	// date may be empty — defaults to today (UTC).
	GetEPG(ctx context.Context, token, channelID, date string) (string, error)
}

// Compile-time check: STBNATSClient implements EPGDeviceAPI.
var _ EPGDeviceAPI = (*STBNATSClient)(nil)

// GetEPGShort returns current+next programme for a single channel.
// token may be empty for unauthenticated access.
func (c *STBNATSClient) GetEPGShort(ctx context.Context, token, channelID string) (*EPGShortEntry, error) {
	var resp EPGShortEntry
	if err := c.rpc(ctx, SubjectEPGShort, map[string]string{"token": token, "channelID": channelID}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetEPG returns a XMLTV document for a single channel on the given date (YYYY-MM-DD).
// token may be empty for unauthenticated access. date may be empty for today.
func (c *STBNATSClient) GetEPG(ctx context.Context, token, channelID, date string) (string, error) {
	var resp struct {
		XMLTV string `json:"xmltv"`
	}
	if err := c.rpc(ctx, SubjectEPGGet, map[string]string{"token": token, "channelID": channelID, "date": date}, &resp); err != nil {
		return "", err
	}
	return resp.XMLTV, nil
}

// ── Config ────────────────────────────────────────────────────────────────────

// ConfigResult holds the device configuration returned by the bridge.
type ConfigResult struct {
	Token  string `json:"token"`
	Active bool   `json:"active"`
}

// GetConfig looks up config by token or MAC address (one must be non-empty).
func (c *STBNATSClient) GetConfig(ctx context.Context, token, mac string) (*ConfigResult, error) {
	var resp ConfigResult
	req := map[string]string{"token": token, "mac": mac}
	if err := c.rpc(ctx, SubjectConfigGet, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ── Channel + DVR ─────────────────────────────────────────────────────────────

// GetChannel returns the live stream URL for the requested channel.
func (c *STBNATSClient) GetChannel(ctx context.Context, token, channelID string) (string, error) {
	var resp struct {
		StreamURL string `json:"streamURL"`
	}
	if err := c.rpc(ctx, SubjectChannelResolve, map[string]string{"token": token, "channelID": channelID}, &resp); err != nil {
		return "", err
	}
	return resp.StreamURL, nil
}

// GetDVR returns the DVR playback URL for a channel recording starting at startAt UTC.
// Returns an error if DVR is not enabled.
func (c *STBNATSClient) GetDVR(ctx context.Context, token, channelID string, startAt time.Time) (string, error) {
	var resp struct {
		StreamURL string `json:"streamURL"`
	}
	req := map[string]any{"token": token, "channelID": channelID, "startAt": startAt.UTC().Unix()}
	if err := c.rpc(ctx, SubjectDVRGet, req, &resp); err != nil {
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
