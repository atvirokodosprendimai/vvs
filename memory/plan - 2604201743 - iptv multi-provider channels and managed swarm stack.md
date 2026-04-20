---
tldr: Implement IPTV multi-provider channels, managed swarm stack with compose generator, and provider URL resolution for playlists
status: active
---

# Plan: IPTV Multi-Provider Channels + Managed Swarm Stack

## Context

- Spec: [[spec - iptv-multi-provider - channels have multiple source providers with managed swarm stack]]
- Push doc: [[push - 2604201741 - iptv multi-provider channels and managed swarm stack]]

## Phases

### Phase 1 - Domain + Persistence - status: open

1. [ ] Add `Channel.Slug` field + `ChannelProvider` domain type + `ResolveURL` helper
   - `internal/modules/iptv/domain/channel.go`: add Slug, ChannelProvider struct, ProviderType consts, ChannelProviderRepository interface, ResolveURL func
2. [ ] Add `IPTVStack` + `IPTVStackChannel` domain types + compose generator
   - new file: `internal/modules/iptv/domain/iptv_stack.go`
   - IPTVStack, IPTVStackChannel, IPTVStackChannelDetail structs
   - IPTVStackStatus consts
   - GenerateIPTVCompose function
   - IPTVStackRepository + IPTVStackChannelRepository interfaces
3. [ ] Migrations: slug column + 3 new tables
   - `migrations/011_channel_slug.sql`
   - `migrations/012_channel_providers.sql`
   - `migrations/013_iptv_stacks.sql`
4. [ ] GORM models + repositories for providers and stacks
   - update ChannelModel with Slug in `adapters/persistence/models.go`
   - add ChannelProviderModel, IPTVStackModel, IPTVStackChannelModel
   - add GormChannelProviderRepository, GormIPTVStackRepository in `adapters/persistence/repositories.go`

### Phase 2 - CQRS - status: open

5. [ ] Channel provider commands
   - new file: `internal/modules/iptv/app/commands/channel_provider.go`
   - CreateChannelProviderHandler / UpdateChannelProviderHandler / DeleteChannelProviderHandler
6. [ ] Channel provider query
   - new file: `internal/modules/iptv/app/queries/list_channel_providers.go`
   - ListChannelProvidersHandler → []ChannelProviderReadModel
   - update `read_models.go` with ChannelProviderReadModel
7. [ ] IPTV stack commands
   - new file: `internal/modules/iptv/app/commands/iptv_stack.go`
   - CreateIPTVStackHandler / UpdateIPTVStackHandler / DeleteIPTVStackHandler
   - DeployIPTVStackHandler — generate YAML → SSH write → docker compose up -d --remove-orphans
   - AddChannelToIPTVStackHandler / RemoveChannelFromIPTVStackHandler → marks stack pending
   - reuse ExecSSH from dockerclient
8. [ ] IPTV stack queries
   - new file: `internal/modules/iptv/app/queries/list_iptv_stacks.go`
   - ListIPTVStacksHandler / GetIPTVStackHandler
   - update `read_models.go` with IPTVStackReadModel, IPTVStackChannelReadModel

### Phase 3 - HTTP Layer - status: open

9. [ ] Channel providers UI — add providers panel to channel detail page
   - new routes in `handlers.go`: POST /api/iptv/channels/{id}/providers, DELETE /api/iptv/channels/{id}/providers/{pid}
   - `templates.templ`: providers section on channel detail; provider create form with URLTemplate, token, type, priority
10. [ ] IPTV stacks UI
    - new routes: GET /iptv/stacks, GET /iptv/stacks/new, GET /iptv/stacks/{id}
    - POST /api/iptv/stacks, POST /api/iptv/stacks/{id}/deploy
    - POST /api/iptv/stacks/{id}/channels, DELETE /api/iptv/stacks/{id}/channels/{cid}
    - `templates.templ`: stack list page, create form, detail page with channel assignment + deploy SSE progress

### Phase 4 - Playlist Integration + Wiring - status: open

11. [ ] Inject providerRepo into STBBridge; update handleChannelResolve
    - `internal/modules/iptv/adapters/nats/stb_bridge.go`: add providerRepo field
    - handleChannelResolve: FindByChannelID → pick lowest priority active → ResolveURL → fallback ch.StreamURL
12. [ ] Wire all new components
    - `internal/app/wire_iptv.go`: repos, commands, queries, inject into STBBridge and HTTP handlers

## Verification

- `go test ./internal/modules/iptv/...` passes
- Creating provider with URLTemplate `http://host/{token}/{channel}/stream.m3u8`, token=`abc`, slug=`bbc-one` → ResolveURL returns `http://host/abc/bbc-one/stream.m3u8`
- GenerateIPTVCompose with 2 internal channels produces 2 ffmpeg services + 1 caddy service
- handleChannelResolve for channel with internal provider returns `http://{wan_ip}/{slug}/stream.m3u8` (resolved from template)
- handleChannelResolve fallback to ch.StreamURL when no active providers

## Progress Log

- 2604201743 — plan created
