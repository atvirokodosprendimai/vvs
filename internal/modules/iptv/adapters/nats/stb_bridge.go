// Package nats provides a NATS RPC bridge for the STB device API.
// STBBridge runs on vvs-core and serves playlist/EPG/validation requests from vvs-stb.
// All subjects use the isp.stb.rpc.* namespace.
package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

// Subjects served by STBBridge.
const (
	SubjectKeyValidate    = "isp.stb.rpc.key.validate"
	SubjectPlaylistGet    = "isp.stb.rpc.playlist.get"
	SubjectEPGGet         = "isp.stb.rpc.epg.get"
	SubjectEPGShort       = "isp.stb.rpc.epg.short"
	SubjectChannelResolve = "isp.stb.rpc.channel.resolve"
	SubjectConfigGet      = "isp.stb.rpc.config.get"
	SubjectDVRGet         = "isp.stb.rpc.dvr.get"
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

type channelProvidersByChannelReader interface {
	FindByChannelID(ctx context.Context, channelID string) ([]*domain.ChannelProvider, error)
}

type epgReader interface {
	ListCurrentAndNext(ctx context.Context, channelEPGIDs []string) (map[string][2]*domain.EPGProgramme, error)
	ListForChannel(ctx context.Context, channelEPGID string, from, to time.Time) ([]*domain.EPGProgramme, error)
}

type stbByMACReader interface {
	FindByMAC(ctx context.Context, mac string) (*domain.STB, error)
}

type subsByCustomerReader interface {
	ListForCustomer(ctx context.Context, customerID string) ([]*domain.Subscription, error)
}

type keysBySubscriptionReader interface {
	FindBySubscriptionID(ctx context.Context, subscriptionID string) ([]*domain.SubscriptionKey, error)
}

// STBBridge subscribes to isp.stb.rpc.* subjects and serves STB data.
// Runs on vvs-core — has direct access to SQLite via the injected repos.
type STBBridge struct {
	nc             *nats.Conn
	keys           subscriptionKeyReader
	subs           subscriptionReader
	channels       channelsByPackageReader
	providers      channelProvidersByChannelReader
	epg            epgReader
	stbsByMAC      stbByMACReader
	subsByCustomer subsByCustomerReader
	keysBySub      keysBySubscriptionReader
	nSubs          []*nats.Subscription
}

// NewSTBBridge creates a bridge. Call Register() to start serving.
func NewSTBBridge(
	nc *nats.Conn,
	keys subscriptionKeyReader,
	subs subscriptionReader,
	channels channelsByPackageReader,
	providers channelProvidersByChannelReader,
	epg epgReader,
	stbsByMAC stbByMACReader,
	subsByCustomer subsByCustomerReader,
	keysBySub keysBySubscriptionReader,
) *STBBridge {
	return &STBBridge{
		nc:             nc,
		keys:           keys,
		subs:           subs,
		channels:       channels,
		providers:      providers,
		epg:            epg,
		stbsByMAC:      stbsByMAC,
		subsByCustomer: subsByCustomer,
		keysBySub:      keysBySub,
	}
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
		{SubjectEPGShort, b.handleEPGShort},
		{SubjectChannelResolve, b.handleChannelResolve},
		{SubjectConfigGet, b.handleConfigGet},
		{SubjectDVRGet, b.handleDVRGet},
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
		Token     string `json:"token"`
		ChannelID string `json:"channelID"`
		Date      string `json:"date"` // YYYY-MM-DD; defaults to today
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if req.ChannelID == "" {
		stbBridgeReply(msg, nil, errChannelNotFound)
		return
	}

	// Parse date; default to today in UTC.
	var dayStart time.Time
	if req.Date != "" {
		parsed, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			stbBridgeReply(msg, nil, fmt.Errorf("invalid date %q: use YYYY-MM-DD", req.Date))
			return
		}
		dayStart = parsed.UTC()
	} else {
		now := time.Now().UTC()
		dayStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}
	dayEnd := dayStart.Add(24 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := b.resolveChannelWithOptionalAuth(ctx, req.Token, req.ChannelID)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}

	epgID := ch.EPGSource
	if epgID == "" {
		epgID = ch.ID
	}
	progs, err := b.epg.ListForChannel(ctx, epgID, dayStart, dayEnd)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}

	stbBridgeReply(msg, struct {
		XMLTV string `json:"xmltv"`
	}{buildChannelXMLTV(ch, progs)}, nil)
}

func (b *STBBridge) handleEPGShort(msg *nats.Msg) {
	var req struct {
		Token     string `json:"token"`
		ChannelID string `json:"channelID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	if req.ChannelID == "" {
		stbBridgeReply(msg, nil, errChannelNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := b.resolveChannelWithOptionalAuth(ctx, req.Token, req.ChannelID)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}

	epgID := ch.EPGSource
	if epgID == "" {
		epgID = ch.ID
	}

	currentNext, err := b.epg.ListCurrentAndNext(ctx, []string{epgID})
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}

	pair := currentNext[epgID]

	type progSlot struct {
		Title     string `json:"title"`
		StartTime string `json:"start"`
		StopTime  string `json:"stop"`
	}

	toSlot := func(p *domain.EPGProgramme) *progSlot {
		if p == nil {
			return nil
		}
		return &progSlot{
			Title:     p.Title,
			StartTime: p.StartTime.Format(time.RFC3339),
			StopTime:  p.StopTime.Format(time.RFC3339),
		}
	}

	stbBridgeReply(msg, struct {
		ChannelEPGID string    `json:"channelEPGID"`
		Current      *progSlot `json:"current,omitempty"`
		Next         *progSlot `json:"next,omitempty"`
	}{
		ChannelEPGID: epgID,
		Current:      toSlot(pair[0]),
		Next:         toSlot(pair[1]),
	}, nil)
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

	// Entitlement check: verify the requested channel is in the subscriber's package.
	// Using FindByPackage rather than FindByID prevents token-holders from resolving
	// channels outside their subscription.
	chs, err := b.channels.FindByPackage(ctx, key.PackageID)
	if err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}
	var ch *domain.Channel
	for _, c := range chs {
		if c.ID == req.ChannelID {
			ch = c
			break
		}
	}
	if ch == nil || !ch.Active {
		stbBridgeReply(msg, nil, errChannelNotFound)
		return
	}

	// Resolve stream URL: prefer active provider with lowest priority.
	streamURL := ch.StreamURL
	if b.providers != nil {
		if provs, err := b.providers.FindByChannelID(ctx, ch.ID); err == nil {
			for _, p := range provs { // ordered by priority ASC
				if p.Active {
					slug := ch.Slug
					if slug == "" {
						slug = domain.Slugify(ch.Name)
					}
					streamURL = domain.ResolveProviderURL(p.URLTemplate, slug, p.Token)
					break
				}
			}
		}
	}

	stbBridgeReply(msg, struct {
		StreamURL string `json:"streamURL"`
	}{streamURL}, nil)
}

func (b *STBBridge) handleConfigGet(msg *nats.Msg) {
	var req struct {
		Token string `json:"token"`
		MAC   string `json:"mac"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		stbBridgeReply(msg, nil, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var token string
	var active bool

	if req.Token != "" {
		// Token path: validate directly.
		key, sub, err := b.resolveKey(ctx, req.Token)
		if err != nil {
			stbBridgeReply(msg, nil, err)
			return
		}
		token = key.Token
		active = sub.Status == domain.SubscriptionActive
	} else if req.MAC != "" {
		// MAC path: STB → customer → subscription → key.
		stb, err := b.stbsByMAC.FindByMAC(ctx, req.MAC)
		if err != nil {
			stbBridgeReply(msg, nil, errConfigNotFound)
			return
		}
		customerSubs, err := b.subsByCustomer.ListForCustomer(ctx, stb.CustomerID)
		if err != nil {
			stbBridgeReply(msg, nil, err)
			return
		}
		var sub *domain.Subscription
		for _, s := range customerSubs {
			if s.Status == domain.SubscriptionActive {
				sub = s
				break
			}
		}
		if sub == nil {
			stbBridgeReply(msg, nil, errConfigNotFound)
			return
		}
		keys, err := b.keysBySub.FindBySubscriptionID(ctx, sub.ID)
		if err != nil {
			stbBridgeReply(msg, nil, err)
			return
		}
		for _, k := range keys {
			if k.IsActive() {
				token = k.Token
				active = true
				break
			}
		}
		if token == "" {
			stbBridgeReply(msg, nil, errConfigNotFound)
			return
		}
	} else {
		stbBridgeReply(msg, nil, errInvalidToken)
		return
	}

	stbBridgeReply(msg, struct {
		Token  string `json:"token"`
		Active bool   `json:"active"`
	}{token, active}, nil)
}

func (b *STBBridge) handleDVRGet(msg *nats.Msg) {
	stbBridgeReply(msg, nil, &stbBridgeError{"dvr not enabled"})
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// resolveChannelWithOptionalAuth looks up a channel by ID.
// If token is non-empty, validates the subscription and checks entitlement.
// If token is empty, returns the channel without auth (public EPG access).
func (b *STBBridge) resolveChannelWithOptionalAuth(ctx context.Context, token, channelID string) (*domain.Channel, error) {
	if token != "" {
		key, sub, err := b.resolveKey(ctx, token)
		if err != nil {
			return nil, err
		}
		if sub.Status != domain.SubscriptionActive {
			return nil, errSuspended
		}
		chs, err := b.channels.FindByPackage(ctx, key.PackageID)
		if err != nil {
			return nil, err
		}
		for _, ch := range chs {
			if ch.ID == channelID {
				return ch, nil
			}
		}
		return nil, errChannelNotFound
	}
	return b.channels.FindByID(ctx, channelID)
}

// buildChannelXMLTV generates a XMLTV document for a single channel with real programmes.
func buildChannelXMLTV(ch *domain.Channel, progs []*domain.EPGProgramme) string {
	epgID := ch.EPGSource
	if epgID == "" {
		epgID = ch.ID
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<tv>")
	sb.WriteString(fmt.Sprintf("\n  <channel id=\"%s\"><display-name>%s</display-name></channel>",
		xmlEscape(epgID), xmlEscape(ch.Name)))
	for _, p := range progs {
		sb.WriteString(fmt.Sprintf(
			"\n  <programme start=\"%s\" stop=\"%s\" channel=\"%s\"><title lang=\"lt\">%s</title></programme>",
			p.StartTime.Format("20060102150405 +0000"),
			p.StopTime.Format("20060102150405 +0000"),
			xmlEscape(epgID),
			xmlEscape(p.Title),
		))
	}
	sb.WriteString("\n</tv>")
	return sb.String()
}

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
		sb.WriteString(fmt.Sprintf("\n  <channel id=\"%s\"><display-name>%s</display-name></channel>",
			xmlEscape(epgID), xmlEscape(ch.Name)))
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
				"\n  <programme start=\"%s\" stop=\"%s\" channel=\"%s\"><title lang=\"lt\">%s</title></programme>",
				start.Format("20060102150405 +0000"),
				stop.Format("20060102150405 +0000"),
				xmlEscape(epgID),
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
	errInvalidToken    = &stbBridgeError{"invalid or revoked token"}
	errSuspended       = &stbBridgeError{"subscription suspended"}
	errChannelNotFound = &stbBridgeError{"channel not found"}
	errConfigNotFound  = &stbBridgeError{"device or subscription not found"}
)

type stbBridgeError struct{ msg string }

func (e *stbBridgeError) Error() string { return e.msg }

func stbBridgeReply(msg *nats.Msg, data any, err error) {
	var env stbEnvelope
	if err != nil {
		// Only expose safe sentinel messages to callers.
		// Internal DB/infrastructure errors are masked to avoid leaking details.
		var bridgeErr *stbBridgeError
		if errors.As(err, &bridgeErr) {
			env = stbEnvelope{Error: bridgeErr.msg}
		} else {
			env = stbEnvelope{Error: "internal error"}
		}
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
