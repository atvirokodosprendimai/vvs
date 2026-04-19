---
tldr: Implement the standard IPTV STB API interface — getConfig(mac|token), EPG domain with real programme data, HLS segment proxy, DVR skeleton
status: active
---

# Plan: IPTV STB Full API — EPG, HLS Proxy, getConfig, DVR

## Context

- Prior work: [[IPTV Module + STB Portal]] — COMPLETE (commits 39294d4→41ab944)
- Current `cmd/stb` has: token-in-URL auth, M3U8 playlists (siptv/tvzone/tvip), stub XMLTV EPG, 302 stream redirect, Stalker/MAG handshake
- This plan: implement the **standard IPTV panel interface** that all real STB integrations expect

## The Standard Interface

Every IPTV STB integration (Tivimate, Smarters, SIPTV, MAG, Formuler, IPTV Player) implements the same conceptual API:

| Method | What | Current state |
|--------|------|---------------|
| `getConfig(mac\|token)` | Device config — server URL, token, EPG URL, timezone | Partial (Stalker handshake only, no MAC→token lookup) |
| `getPlaylist(token)` | Channels list with metadata | ✓ Done |
| `getChannel(token, channelID)` | Stream access — CDN proxy or redirect | Only 302 redirect, no proxy |
| `getDVR(token, channel, startUnix)` | Timeshift/recording playback | Not implemented |
| `getEPG(token)` | Full XMLTV EPG | Stub only (hourly channel-name slots) |
| `getEPGShort(token)` | Current+next per channel | Not implemented |

---

## Phase 1 — EPG Domain Layer — status: completed

Real programme data is the highest-value addition. Everything EPG-related depends on this domain layer.

1. [x] `internal/modules/iptv/domain/epg_programme.go`
   ```go
   type EPGProgramme struct {
       ID           string
       ChannelEPGID string    // matches Channel.EPGSource (tvg-id)
       Title        string
       Description  string
       StartTime    time.Time
       StopTime     time.Time
       Category     string
       Rating       string    // e.g. "TV-PG"
   }

   type EPGProgrammeRepository interface {
       Save(ctx, p *EPGProgramme) error
       BulkSave(ctx, ps []*EPGProgramme) error
       ListForChannel(ctx, channelEPGID string, from, to time.Time) ([]*EPGProgramme, error)
       ListCurrentAndNext(ctx, channelEPGIDs []string) (map[string][2]*EPGProgramme, error)
       DeleteBefore(ctx, before time.Time) error  // for cleanup
   }
   ```

2. [x] `internal/modules/iptv/migrations/002_epg_programmes.sql`
   ```sql
   CREATE TABLE iptv_epg_programmes (
     id TEXT PRIMARY KEY,
     channel_epg_id TEXT NOT NULL,
     title TEXT NOT NULL,
     description TEXT,
     start_time INTEGER NOT NULL,  -- Unix timestamp
     stop_time INTEGER NOT NULL,
     category TEXT,
     rating TEXT,
     INDEX(channel_epg_id, start_time),
     INDEX(start_time)
   );
   ```

---

## Phase 2 — EPG Persistence + XMLTV Import — status: completed

3. [x] `internal/modules/iptv/adapters/persistence/epg_programme_repository.go`
   - GORM model with `TableName() = "iptv_epg_programmes"`
   - `ListForChannel`: `WHERE channel_epg_id=? AND start_time >= ? AND stop_time <= ? ORDER BY start_time`
   - `ListCurrentAndNext`: `WHERE channel_epg_id IN (?) AND stop_time >= now() ORDER BY start_time LIMIT 2 per channel` (use subquery or Go-side grouping)
   - `BulkSave`: upsert on `(channel_epg_id, start_time)` unique pair

4. [x] XMLTV parser `internal/modules/iptv/adapters/xmltv/parser.go`
   - Parse standard XMLTV format (`<channel>`, `<programme start= stop= channel=>`)
   - Returns `[]EPGProgramme` slices
   - Handle timezone offsets in XMLTV timestamps (`20060102150405 +0200`)

5. [x] EPG import command `internal/modules/iptv/app/commands/epg_import.go`
   ```go
   type ImportEPGFromURLHandler struct { epgRepo domain.EPGProgrammeRepository }
   type ImportEPGCommand struct { URL string; DaysAhead int }
   // Fetches XMLTV from URL, parses, bulk-saves
   ```

6. [x] Admin endpoint: `POST /api/iptv/epg/import` in `adapters/http/handlers.go`
   - => JSON body {url, days_ahead}; returns {Imported, Skipped}; 202-style synchronous for now
   - Form field: EPG URL + days ahead (default 7)
   - Fires import in goroutine → streams progress via SSE or just returns 202 Accepted
   - Add "EPG Import" button to IPTV settings/admin page

---

## Phase 3 — EPG NATS Subjects + Real STB Endpoints — status: open

7. [ ] Add `isp.stb.rpc.epg.short` NATS subject to `stb_bridge.go`
   - Request: `{token}` → resolveKey → get packageID → get channels → get current+next for each
   - Response: `[{channelEPGID, current:{title,start,stop}, next:{title,start,stop}}]`

8. [ ] Replace stub `buildXMLTV` in `stb_bridge.go` with real query
   - `isp.stb.rpc.epg.get` already exists — replace body with real `ListForChannel` calls
   - Still generate valid XMLTV envelope; programmes now have real title/description/times

9. [ ] New STB endpoints in `cmd/stb/main.go`:
   - `GET /epg/{token}/now.json` — short EPG JSON (current+next for all channels); Content-Type: application/json
   - `GET /epg/{token}/{channelID}.json` — full EPG for one channel (7 days default)
   - `GET /epg/{token}.xml` — already exists; now backed by real data

10. [ ] Add `GetEPGShort(ctx, token)` to `stb_client.go`

---

## Phase 4 — getConfig with MAC Lookup — status: open

Currently Stalker `handshake` only returns a config if you already have a token. Real MAG boxes connect MAC-first to get their token.

11. [ ] New NATS subject `isp.stb.rpc.config.get` in `stb_bridge.go`
    - Request: `{token?: string, mac?: string}` — one of the two required
    - MAC path: `STBRepo.FindByMAC(mac)` → CustomerID → `ListForCustomer` subscriptions → find active → `FindBySubscriptionID` keys → find active key
    - Token path: `FindByToken(token)` → same resolveKey logic
    - Response: `{token, serverURL, epgURL, timezone, active}`

12. [ ] New interface on `STBBridge`: add `STBByMACReader` (interface, not concrete repo)
    ```go
    type stbByMACReader interface {
        FindByMAC(ctx, mac string) (*domain.STB, error)
    }
    ```

13. [ ] Add `GetConfig(ctx, token, mac string)` to `stb_client.go`

14. [ ] New endpoint in `cmd/stb/main.go`: `GET /getconfig`
    - Accept `?mac=` or `?token=` query param
    - Also accept MAC from `X-STB-MAC` header (MAG boxes send this)
    - Returns JSON: `{"server_url":..., "token":..., "epg_url":..., "timezone":"Europe/Vilnius"}`

15. [ ] Enhance Stalker `handshake` in `stalkerHandler` to use MAC
    - MAG box sends `X-STB-MAC` header on every request
    - On handshake: if no `?token=`, try MAC lookup to find token
    - Return token in handshake response so box uses it for subsequent calls

---

## Phase 5 — HLS Segment Proxy (getChannel) — status: open

Currently `streamHandler` just does 302 redirect. For CDN caching + proper access control, need a transparent HLS proxy.

16. [ ] `internal/modules/iptv/stbproxy/proxy.go` — HLS proxy package
    ```
    Input:  GET /stream/{token}/{channelID}
    Output: proxied m3u8 manifest with rewritten segment URLs
    
    Manifest rewrite:
    Original:  https://upstream-cdn.com/live/channel.ts
    Rewritten: https://stb.example.com/seg/{token}/{channelID}/{hash}.ts
    
    Segment endpoint:
    GET /seg/{token}/{channelID}/{segmentHash}.ts → proxy to upstream
    ```
    - Validate token on manifest fetch (not per-segment, for perf)
    - Cache manifest for 1-2s (HLS segment duration)
    - Optional: LRU cache for `.ts` segments (configurable max size, default 0 = disabled)
    - Configurable via env: `STB_PROXY_ENABLED=true`, `STB_PROXY_CACHE_MB=256`

17. [ ] Update `streamHandler` to detect `Accept: application/x-mpegURL` or `?proxy=1`
    - Without proxy flag: keep 302 redirect (default — lowest latency)
    - With proxy flag or env `STB_PROXY_ENABLED=true`: serve proxied manifest

18. [ ] Update `cmd/stb/main.go` routes:
    - Add `GET /seg/{token}/{channelID}/{segmentHash}` — segment proxy endpoint
    - Segment handler: validate `{segmentHash}` via short-lived signed URL (HMAC-SHA256 with token+channelID+expiry)

---

## Phase 6 — DVR Skeleton — status: open

Minimal: define the interface, domain entity, and endpoint. Full recording engine is a future phase.

19. [ ] `internal/modules/iptv/domain/dvr_recording.go`
    ```go
    type DVRRecording struct {
        ID        string
        ChannelID string
        StartTime time.Time
        StopTime  time.Time
        Status    string  // scheduled/recording/available/expired
        StoragePath string
    }
    type DVRRecordingRepository interface { ... }
    ```

20. [ ] `GET /dvr/{token}/{channelID}/{startUnix}` in `cmd/stb/main.go`
    - For now: returns 501 Not Implemented with JSON `{"error":"dvr not enabled"}`
    - Ensures clients don't get unexpected errors; operator knows the endpoint exists

21. [ ] NATS subject `isp.stb.rpc.dvr.get` — stub returning "not enabled"

---

## Verification

```bash
# Build still clean after all phases
go build ./...
go test ./internal/modules/iptv/...

# Phase 1-2: EPG data import
# 1. Admin → /iptv/epg/import → submit public XMLTV URL
# 2. Check /iptv/channels — EPG source IDs match imported programmes

# Phase 3: Real EPG from STB
# curl "http://localhost:8082/epg/{token}/now.json" → [{channelEPGID, current:{...}, next:{...}}]
# curl "http://localhost:8082/epg/{token}.xml" → real programmes, not "Channel Name" stubs

# Phase 4: MAC-based config
# curl "http://localhost:8082/getconfig?mac=00:1A:2B:3C:4D:5E" → {token, serverURL, epgURL}
# MAG box: set portal URL = http://localhost:8082 → box loads config via MAC header

# Phase 5: HLS proxy
# STB_PROXY_ENABLED=true ./bin/vvs-stb serve
# curl "http://localhost:8082/stream/{token}/{channelID}" → m3u8 manifest with /seg/... URLs
# curl /seg/{token}/{channelID}/{hash}.ts → proxied segment bytes

# Phase 6: DVR stub
# curl "http://localhost:8082/dvr/{token}/{channelID}/1713513600" → 501 {"error":"dvr not enabled"}
```

## Adjustments

## Progress Log

- 2026-04-19: Plan created — 6 phases, 21 actions; builds on completed IPTV module
- 2026-04-19: Phase 1+2 complete (commit d0aa934) — EPG domain, migration, persistence, XMLTV parser (6 tests), import command, admin endpoint; `go build ./...` clean
