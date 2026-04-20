---
tldr: Implement IPTV multi-provider channels, managed swarm stack with compose generator, and provider URL resolution for playlists
status: completed
---

# Plan: IPTV Multi-Provider Channels + Managed Swarm Stack

## Context

- Spec: [[spec - iptv-multi-provider - channels have multiple source providers with managed swarm stack]]
- Push doc: [[push - 2604201741 - iptv multi-provider channels and managed swarm stack]]

## Phases

### Phase 1 - Domain + Persistence - status: completed

1. [x] Add `Channel.Slug` field + `ChannelProvider` domain type + `ResolveURL` helper
   - => `internal/modules/iptv/domain/channel.go`: Slug, ChannelProvider, ProviderType consts, ChannelProviderRepository, ResolveProviderURL
2. [x] Add `IPTVStack` + `IPTVStackChannel` domain types + compose generator
   - => `internal/modules/iptv/domain/iptv_stack.go`
3. [x] Migrations: slug column + 3 new tables
   - => migrations/004_channel_slug.sql, 005_channel_providers.sql, 006_iptv_stacks.sql
4. [x] GORM models + repositories for providers and stacks
   - => persistence/models.go + repositories.go updated

### Phase 2 - CQRS - status: completed

5. [x] Channel provider commands
   - => `internal/modules/iptv/app/commands/channel_provider.go`
6. [x] Channel provider query
   - => `internal/modules/iptv/app/queries/list_channel_providers.go`
   - => Slug added to ChannelReadModel; GetChannelHandler added to list_channels.go
7. [x] IPTV stack commands
   - => `internal/modules/iptv/app/commands/iptv_stack.go`
   - => DeployIPTVStackHandler with WithProgress + ExecSSH
8. [x] IPTV stack queries
   - => `internal/modules/iptv/app/queries/list_iptv_stacks.go`
   - => ListIPTVStacksHandler + GetIPTVStackChannelsHandler

### Phase 3 - HTTP Layer - status: completed

9. [x] Channel providers UI — add providers panel to channel detail page
   - => GET /iptv/channels/{id}, GET /sse/iptv/channels/{id}/providers
   - => POST/DELETE /api/iptv/channels/{id}/providers[/{pid}]
   - => IPTVChannelDetailPage + IPTVChannelProviderTable templates
   - => "Providers" link added to channel table row
10. [x] IPTV stacks UI
    - => GET /iptv/stacks, GET /iptv/stacks/{id}
    - => POST /api/iptv/stacks, DELETE, /channels, /deploy
    - => IPTVStackListPage + IPTVStackTable + IPTVStackDetailPage + IPTVStackChannelTable + IPTVDeployLog

### Phase 4 - Playlist Integration + Wiring - status: completed

11. [x] Inject providerRepo into STBBridge; update handleChannelResolve
    - => channelProvidersByChannelReader interface added to stb_bridge.go
    - => handleChannelResolve: picks lowest-priority active provider → ResolveProviderURL → fallback ch.StreamURL
12. [x] Wire all new components
    - => wire_iptv.go: all repos/commands/queries wired; NodeSSHLookup via swarmNodeSSHAdapter
    - => dockerWired exposes swarmNodeRepo; wireIPTV takes it as param
    - => wire_infra.go passes providerRepo to NewSTBBridge
    - => builder.go: wireDocker called before wireIPTV

## Verification

- `go test ./internal/modules/iptv/...` passes
- Creating provider with URLTemplate `http://host/{token}/{channel}/stream.m3u8`, token=`abc`, slug=`bbc-one` → ResolveURL returns `http://host/abc/bbc-one/stream.m3u8`
- GenerateIPTVCompose with 2 internal channels produces 2 ffmpeg services + 1 caddy service
- handleChannelResolve for channel with internal provider returns `http://{wan_ip}/{slug}/stream.m3u8` (resolved from template)
- handleChannelResolve fallback to ch.StreamURL when no active providers

## Progress Log

- 2604201743 — plan created
- 2604201900 — all 12 actions complete; build clean; all IPTV tests pass; commit 5f53d25
