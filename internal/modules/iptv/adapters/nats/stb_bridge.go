// Package nats provides a NATS RPC bridge for the STB device API.
// STBBridge runs on vvs-core and serves playlist/EPG/validation requests from vvs-stb.
// All subjects use the isp.stb.rpc.* namespace.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vvs/isp/internal/modules/iptv/domain"
)

// Subjects served by STBBridge.
const (
	SubjectKeyValidate     = "isp.stb.rpc.key.validate"
	SubjectPlaylistGet     = "isp.stb.rpc.playlist.get"
	SubjectEPGGet          = "isp.stb.rpc.epg.get"
	SubjectChannelResolve  = "isp.stb.rpc.channel.resolve"
)

// ── Interfaces ────────────────────────────────────────────────────────────────

type subscriptionKeyReader interface {
	FindByToken(ctx context.Context, token string) (*domain.SubscriptionKey, error)
}

type subscriptionReader interface {
	FindByID(ctx context.Context, id string) (*domain.Subscription, error)
}

type channelsByPackageReader interface {
	FindByPackage(ctx context.Context, packageID string) ([]*domain.Channel, error)
	FindByID(ctx context.Context, id string) (*domain.Channel, error)
}

// STBBridge subscribes to isp.stb.rpc.* subjects and serves STB data.
// Runs on vvs-core — has direct access to SQLite via the injected repos.
type STBBridge struct {
	nc       *nats.Conn
	keys     subscriptionKeyReader
	subs     subscriptionReader
	channels channelsByPackageReader
	nSubs    []*nats.Subscription
}

// NewSTBBridge creates a bridge. Call Register() to start serving.
func NewSTBBridge(
	nc *nats.Conn,
	keys subscriptionKeyReader,
	subs subscriptionReader,
	channels channelsByPackageReader,
) *STBBridge {
	return &STBBridge{nc: nc, keys: keys, subs: subs, channels: channels}
}

// Register subscribes to all STB RPC subjects.
func (b *STBBridge) Register() error {
	entries := []struct {
		subject string
		handler nats.MsgHandler
	}{
		{SubjectKeyValidate, b.handleKeyValidate},
		{SubjectPlaylistGet, b.handlePlaylistGet},
		{SubjectEPGGet, b.handleEPGGet},
		{SubjectChannelResolve, b.handleChannelResolve},
	}
	for _, e := range entries {
		sub, err := b.nc.Subscribe(e.subject, e.handler)
		if err != nil {
			return err
		}
		b.nSubs = append(b.nSubs, sub)
	}
	return nil
}

// Close unsubscribes all handlers.
func (b *STBBridge) Close() {
	for _, s := range b.nSubs {
		_ = s.Unsubscribe()
	}
	b.nSubs = nil
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (b *STBBridge) handleKeyValidate(msg *nats.Msg) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if req.Token == "" {
		stbBridgeReply(msg, nil, errInvalidToken)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key, sub, err := b.resolveKey(ctx, req.Token)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}

	stbBridgeReply(msg, struct {
		CustomerID     string `json:"customerID"`
		SubscriptionID string `json:"subscriptionID"`
		PackageID      string `json:"packageID"`
		Active         bool   `json:"active"`
	}{
		CustomerID:     key.CustomerID,
		SubscriptionID: key.SubscriptionID,
		PackageID:      key.PackageID,
		Active:         sub.Status == domain.SubscriptionActive,
	}, nil)
}

func (b *STBBridge) handlePlaylistGet(msg *nats.Msg) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if req.Token == "" {
		stbBridgeReply(msg, nil, errInvalidToken)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key, sub, err := b.resolveKey(ctx, req.Token)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if sub.Status != domain.SubscriptionActive {
		stbBridgeReply(msg, nil, errSuspended)
		return
	}

	chs, err := b.channels.FindByPackage(ctx, key.PackageID)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}

	type channelItem struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		LogoURL  string `json:"logoURL"`
		EPGSource string `json:"epgSource"`
		Category string `json:"category"`
	}
	items := make([]channelItem, 0, len(chs))
	for _, ch := range chs {
		if !ch.Active {
			continue
		}
		items = append(items, channelItem{
			ID:       ch.ID,
			Name:     ch.Name,
			LogoURL:  ch.LogoURL,
			EPGSource: ch.EPGSource,
			Category: ch.Category,
		})
	}

	stbBridgeReply(msg, struct {
		Channels []channelItem `json:"channels"`
	}{items}, nil)
}

func (b *STBBridge) handleEPGGet(msg *nats.Msg) {
	var req struct {
		Token string `json:"token"`
		Days  int    `json:"days"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if req.Token == "" {
		stbBridgeReply(msg, nil, errInvalidToken)
		return
	}
	if req.Days <= 0 {
		req.Days = 3
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key, sub, err := b.resolveKey(ctx, req.Token)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if sub.Status != domain.SubscriptionActive {
		stbBridgeReply(msg, nil, errSuspended)
		return
	}

	chs, err := b.channels.FindByPackage(ctx, key.PackageID)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}

	xmltv := buildXMLTV(chs, req.Days)
	stbBridgeReply(msg, struct {
		XMLTV string `json:"xmltv"`
	}{xmltv}, nil)
}

func (b *STBBridge) handleChannelResolve(msg *nats.Msg) {
	var req struct {
		Token     string `json:"token"`
		ChannelID string `json:"channelID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if req.Token == "" || req.ChannelID == "" {
		stbBridgeReply(msg, nil, errInvalidToken)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key, sub, err := b.resolveKey(ctx, req.Token)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if sub.Status != domain.SubscriptionActive {
		stbBridgeReply(msg, nil, errSuspended)
		return
	}
	_ = key // ownership already verified by resolveKey

	ch, err := b.channels.FindByID(ctx, req.ChannelID)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if ch == nil || !ch.Active {
		stbBridgeReply(msg, nil, errChannelNotFound)
		return
	}

	stbBridgeReply(msg, struct {
		StreamURL string `json:"streamURL"`
	}{ch.StreamURL}, nil)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// resolveKey validates a token string and returns the key + subscription.
// Returns an error if the token is revoked, the subscription is not found, etc.
func (b *STBBridge) resolveKey(ctx context.Context, token string) (*domain.SubscriptionKey, *domain.Subscription, error) {
	key, err := b.keys.FindByToken(ctx, token)
	if err != nil {
		return nil, nil, errInvalidToken
	}
	if !key.IsActive() {
		return nil, nil, errInvalidToken
	}
	sub, err := b.subs.FindByID(ctx, key.SubscriptionID)
	if err != nil {
		return nil, nil, err
	}
	return key, sub, nil
}

// buildXMLTV generates a minimal XMLTV document for the given channels.
// In production this would include real programme data; currently returns channel stubs.
func buildXMLTV(channels []*domain.Channel, days int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n<tv>")
	for _, ch := range channels {
		if !ch.Active {
			continue
		}
		epgID := ch.EPGSource
		if epgID == "" {
			epgID = ch.ID
		}
		sb.WriteString(fmt.Sprintf("\n  <channel id=%q><display-name>%s</display-name></channel>",
			epgID, xmlEscape(ch.Name)))
	}
	// Programme stubs (real EPG data would come from an external source).
	now := time.Now().UTC().Truncate(time.Hour)
	for i := 0; i < days*24; i++ {
		start := now.Add(time.Duration(i) * time.Hour)
		stop := start.Add(time.Hour)
		for _, ch := range channels {
			if !ch.Active {
				continue
			}
			epgID := ch.EPGSource
			if epgID == "" {
				epgID = ch.ID
			}
			sb.WriteString(fmt.Sprintf(
				"\n  <programme start=%q stop=%q channel=%q><title lang=\"lt\">%s</title></programme>",
				start.Format("20060102150405 +0000"),
				stop.Format("20060102150405 +0000"),
				epgID,
				xmlEscape(ch.Name),
			))
		}
	}
	sb.WriteString("\n</tv>")
	return sb.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// ── Bridge helpers ────────────────────────────────────────────────────────────

type stbEnvelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

var (
	errInvalidToken   = &stbBridgeError{"invalid or revoked token"}
	errSuspended      = &stbBridgeError{"subscription suspended"}
	errChannelNotFound = &stbBridgeError{"channel not found"}
)

type stbBridgeError struct{ msg string }

func (e *stbBridgeError) Error() string { return e.msg }

func stbBridgeReply(msg *nats.Msg, data any, err error) {
	var env stbEnvelope
	if err != nil {
		env = stbEnvelope{Error: err.Error()}
	} else {
		env = stbEnvelope{Data: data}
	}
	b, merr := json.Marshal(env)
	if merr != nil {
		log.Printf("stb bridge: marshal reply: %v", merr)
		return
	}
	if err := msg.Respond(b); err != nil {
		log.Printf("stb bridge: respond: %v", err)
	}
}
