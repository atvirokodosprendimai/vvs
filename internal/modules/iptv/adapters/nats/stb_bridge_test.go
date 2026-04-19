package nats_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	natspkg "github.com/vvs/isp/internal/infrastructure/nats"
	iptvnats "github.com/vvs/isp/internal/modules/iptv/adapters/nats"
	"github.com/vvs/isp/internal/modules/iptv/domain"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubKeyReader struct {
	key *domain.SubscriptionKey
	err error
}

func (s *stubKeyReader) FindByToken(_ context.Context, _ string) (*domain.SubscriptionKey, error) {
	return s.key, s.err
}

type stubSubReader struct {
	sub *domain.Subscription
	err error
}

func (s *stubSubReader) FindByID(_ context.Context, _ string) (*domain.Subscription, error) {
	return s.sub, s.err
}

type stubChannelReader struct {
	byPackage []*domain.Channel
	byID      map[string]*domain.Channel
}

type stubEPGReader struct {
	data map[string][2]*domain.EPGProgramme
}

func (s *stubEPGReader) ListCurrentAndNext(_ context.Context, _ []string) (map[string][2]*domain.EPGProgramme, error) {
	return s.data, nil
}

type stubSTBByMACReader struct {
	stb *domain.STB
	err error
}

func (s *stubSTBByMACReader) FindByMAC(_ context.Context, _ string) (*domain.STB, error) {
	return s.stb, s.err
}

type stubSubsByCustomerReader struct {
	subs []*domain.Subscription
}

func (s *stubSubsByCustomerReader) ListForCustomer(_ context.Context, _ string) ([]*domain.Subscription, error) {
	return s.subs, nil
}

type stubKeysBySubReader struct {
	keys []*domain.SubscriptionKey
}

func (s *stubKeysBySubReader) FindBySubscriptionID(_ context.Context, _ string) ([]*domain.SubscriptionKey, error) {
	return s.keys, nil
}

func newStubChannelReader() *stubChannelReader {
	return &stubChannelReader{byID: make(map[string]*domain.Channel)}
}

func (s *stubChannelReader) FindByPackage(_ context.Context, _ string) ([]*domain.Channel, error) {
	return s.byPackage, nil
}

func (s *stubChannelReader) FindByID(_ context.Context, id string) (*domain.Channel, error) {
	ch, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return ch, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

type bridgeFixture struct {
	clientNC    *natsgo.Conn
	keys        *stubKeyReader
	subs        *stubSubReader
	channels    *stubChannelReader
	epg         *stubEPGReader
	stbsByMAC   *stubSTBByMACReader
	subsByCustomer *stubSubsByCustomerReader
	keysBySub   *stubKeysBySubReader
}

func startSTBBridge(t *testing.T) *bridgeFixture {
	t.Helper()
	ns, serverNC, err := natspkg.StartEmbedded("127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ns.Shutdown() })
	t.Cleanup(func() { serverNC.Close() })

	fix := &bridgeFixture{
		keys:           &stubKeyReader{},
		subs:           &stubSubReader{},
		channels:       newStubChannelReader(),
		epg:            &stubEPGReader{},
		stbsByMAC:      &stubSTBByMACReader{},
		subsByCustomer: &stubSubsByCustomerReader{},
		keysBySub:      &stubKeysBySubReader{},
	}

	bridge := iptvnats.NewSTBBridge(serverNC, fix.keys, fix.subs, fix.channels, fix.epg,
		fix.stbsByMAC, fix.subsByCustomer, fix.keysBySub)
	require.NoError(t, bridge.Register())
	t.Cleanup(bridge.Close)

	clientNC, err := natsgo.Connect(fmt.Sprintf("nats://%s", ns.Addr().String()))
	require.NoError(t, err)
	t.Cleanup(func() { clientNC.Close() })
	fix.clientNC = clientNC

	return fix
}

func rpcSTB(t *testing.T, nc *natsgo.Conn, subject string, req any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(req)
	msg, err := nc.Request(subject, b, 2*time.Second)
	require.NoError(t, err)
	var env map[string]any
	require.NoError(t, json.Unmarshal(msg.Data, &env))
	return env
}

func activeKey() *domain.SubscriptionKey {
	return &domain.SubscriptionKey{
		ID:             "key-1",
		SubscriptionID: "sub-1",
		CustomerID:     "cust-1",
		PackageID:      "pkg-1",
		Token:          "abc123",
	}
}

func activeSub() *domain.Subscription {
	return &domain.Subscription{
		ID:         "sub-1",
		CustomerID: "cust-1",
		PackageID:  "pkg-1",
		Status:     domain.SubscriptionActive,
	}
}

// ── KeyValidate ───────────────────────────────────────────────────────────────

func TestSTBBridge_KeyValidate_Valid(t *testing.T) {
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectKeyValidate, map[string]string{"token": "abc123"})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	assert.Equal(t, "cust-1", data["customerID"])
	assert.Equal(t, "sub-1", data["subscriptionID"])
	assert.Equal(t, true, data["active"])
}

func TestSTBBridge_KeyValidate_RevokedKey(t *testing.T) {
	fix := startSTBBridge(t)
	k := activeKey()
	k.Revoke()
	fix.keys.key = k
	fix.subs.sub = activeSub()

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectKeyValidate, map[string]string{"token": "abc123"})
	assert.NotEmpty(t, env["error"])
}

func TestSTBBridge_KeyValidate_EmptyToken(t *testing.T) {
	fix := startSTBBridge(t)

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectKeyValidate, map[string]string{"token": ""})
	assert.NotEmpty(t, env["error"])
}

// ── PlaylistGet ───────────────────────────────────────────────────────────────

func TestSTBBridge_PlaylistGet_ReturnsActiveChannels(t *testing.T) {
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()
	fix.channels.byPackage = []*domain.Channel{
		{ID: "ch-1", Name: "CNN", Active: true, Category: "News"},
		{ID: "ch-2", Name: "Inactive", Active: false},
	}

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectPlaylistGet, map[string]string{"token": "abc123"})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	channels := data["channels"].([]any)
	assert.Len(t, channels, 1, "only active channels returned")
	ch := channels[0].(map[string]any)
	assert.Equal(t, "ch-1", ch["id"])
	assert.Equal(t, "CNN", ch["name"])
}

func TestSTBBridge_PlaylistGet_SuspendedSubscription(t *testing.T) {
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	sub := activeSub()
	sub.Status = domain.SubscriptionSuspended
	fix.subs.sub = sub

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectPlaylistGet, map[string]string{"token": "abc123"})
	assert.NotEmpty(t, env["error"])
}

// ── ChannelResolve ────────────────────────────────────────────────────────────

func TestSTBBridge_ChannelResolve_ReturnsStreamURL(t *testing.T) {
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()
	// Bridge now uses FindByPackage for entitlement check, not FindByID.
	fix.channels.byPackage = []*domain.Channel{
		{ID: "ch-5", Name: "BBC", StreamURL: "http://stream.example.com/bbc", Active: true},
	}

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectChannelResolve, map[string]string{
		"token":     "abc123",
		"channelID": "ch-5",
	})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	assert.Equal(t, "http://stream.example.com/bbc", data["streamURL"])
}

func TestSTBBridge_ChannelResolve_NotInPackage(t *testing.T) {
	// Token is valid but channelID is not in the subscriber's package — entitlement bypass prevention.
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()
	fix.channels.byPackage = []*domain.Channel{
		{ID: "ch-allowed", Name: "CNN", Active: true},
	}

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectChannelResolve, map[string]string{
		"token":     "abc123",
		"channelID": "ch-not-in-package",
	})
	assert.NotEmpty(t, env["error"])
}

func TestSTBBridge_ChannelResolve_InactiveChannel(t *testing.T) {
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()
	fix.channels.byPackage = []*domain.Channel{
		{ID: "ch-inactive", Active: false},
	}

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectChannelResolve, map[string]string{
		"token":     "abc123",
		"channelID": "ch-inactive",
	})
	assert.NotEmpty(t, env["error"])
}

// ── EPGGet ────────────────────────────────────────────────────────────────────

// ── EPGShort ──────────────────────────────────────────────────────────────────

func TestSTBBridge_EPGShort_ReturnsCurrentAndNext(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()
	fix.channels.byPackage = []*domain.Channel{
		{ID: "ch-1", Name: "BBC", EPGSource: "bbc.co.uk", Active: true},
	}
	fix.epg.data = map[string][2]*domain.EPGProgramme{
		"bbc.co.uk": {
			{ID: "p1", ChannelEPGID: "bbc.co.uk", Title: "News", StartTime: now, StopTime: now.Add(time.Hour)},
			{ID: "p2", ChannelEPGID: "bbc.co.uk", Title: "Film", StartTime: now.Add(time.Hour), StopTime: now.Add(2 * time.Hour)},
		},
	}

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectEPGShort, map[string]string{"token": "abc123"})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	progs := data["programmes"].([]any)
	require.Len(t, progs, 1)
	entry := progs[0].(map[string]any)
	assert.Equal(t, "bbc.co.uk", entry["channelEPGID"])
	current := entry["current"].(map[string]any)
	assert.Equal(t, "News", current["title"])
	next := entry["next"].(map[string]any)
	assert.Equal(t, "Film", next["title"])
}

func TestSTBBridge_EPGShort_NoSlotsWhenNoData(t *testing.T) {
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()
	fix.channels.byPackage = []*domain.Channel{
		{ID: "ch-1", Name: "BBC", EPGSource: "bbc.co.uk", Active: true},
	}
	// epg.data is nil — no programmes loaded

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectEPGShort, map[string]string{"token": "abc123"})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	progs := data["programmes"].([]any)
	require.Len(t, progs, 1, "channel still appears even without EPG data")
	entry := progs[0].(map[string]any)
	assert.Equal(t, "bbc.co.uk", entry["channelEPGID"])
	_, hasCurrent := entry["current"]
	assert.False(t, hasCurrent, "no current slot when no programme data")
}

// ── EPGGet ────────────────────────────────────────────────────────────────────

func TestSTBBridge_EPGGet_ReturnsXMLTV(t *testing.T) {
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()
	fix.channels.byPackage = []*domain.Channel{
		{ID: "ch-1", Name: "BBC", EPGSource: "bbc.co.uk", Active: true},
	}

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectEPGGet, map[string]any{
		"token": "abc123",
		"days":  1,
	})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	xmltv, ok := data["xmltv"].(string)
	require.True(t, ok)
	assert.Contains(t, xmltv, `<tv>`)
	assert.Contains(t, xmltv, `bbc.co.uk`)
}

// ── ConfigGet ─────────────────────────────────────────────────────────────────

func TestSTBBridge_ConfigGet_ByToken(t *testing.T) {
	fix := startSTBBridge(t)
	fix.keys.key = activeKey()
	fix.subs.sub = activeSub()

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectConfigGet, map[string]string{"token": "abc123"})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	assert.Equal(t, "abc123", data["token"])
	assert.Equal(t, true, data["active"])
}

func TestSTBBridge_ConfigGet_ByMAC(t *testing.T) {
	fix := startSTBBridge(t)
	fix.stbsByMAC.stb = &domain.STB{ID: "stb-1", CustomerID: "cust-1", MAC: "00:1A:2B:3C:4D:5E"}
	fix.subsByCustomer.subs = []*domain.Subscription{activeSub()}
	fix.keysBySub.keys = []*domain.SubscriptionKey{activeKey()}
	fix.keys.key = activeKey() // needed for subsequent resolveKey after finding by MAC
	fix.subs.sub = activeSub()

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectConfigGet, map[string]string{"mac": "00:1A:2B:3C:4D:5E"})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	assert.Equal(t, "abc123", data["token"])
	assert.Equal(t, true, data["active"])
}

func TestSTBBridge_ConfigGet_MACNotFound(t *testing.T) {
	fix := startSTBBridge(t)
	fix.stbsByMAC.err = fmt.Errorf("not found")

	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectConfigGet, map[string]string{"mac": "00:1A:2B:3C:4D:5E"})
	assert.NotEmpty(t, env["error"])
}

func TestSTBBridge_ConfigGet_EmptyRequest(t *testing.T) {
	fix := startSTBBridge(t)
	env := rpcSTB(t, fix.clientNC, iptvnats.SubjectConfigGet, map[string]string{})
	assert.NotEmpty(t, env["error"])
}
