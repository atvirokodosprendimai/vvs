---
spec: eidos/spec - iptv-multi-provider - channels have multiple source providers with managed swarm stack.md
status: active
---

# Push: IPTV Multi-Provider Channels

## Change Inventory

### 1. Domain — Channel.Slug + ChannelProvider

**Status: MISSING**

- `internal/modules/iptv/domain/channel.go`
  - Add `Slug string` field to `Channel`
  - Add `ChannelProvider` struct: ID, ChannelID, Name, URLTemplate, Token, Type, Priority, Active, timestamps
  - Add `ProviderType` const: `internal` | `external`
  - Add `ResolveURL(template, slug, token string) string` helper
  - Add `ChannelProviderRepository` interface: Save/FindByID/FindByChannelID/Delete
  - `ChannelRepository.Save` already exists — no change needed

### 2. Domain — IPTVStack + IPTVStackChannel

**Status: MISSING**

- `internal/modules/iptv/domain/iptv_stack.go` (new file)
  - `IPTVStack`: ID, Name, ClusterID, NodeID, WANNetworkID, OverlayNetworkID, WanIP, Status, LastDeployedAt, timestamps
  - `IPTVStackChannel`: ID, StackID, ChannelID, ProviderID
  - `IPTVStackStatus` consts: pending/deploying/running/error
  - `GenerateIPTVCompose(stack *IPTVStack, channels []IPTVStackChannelDetail) string`
    - `IPTVStackChannelDetail`: channel slug + resolved source URL + provider type
  - `IPTVStackRepository` interface: Save/FindByID/FindAll/Delete
  - `IPTVStackChannelRepository` interface: Save/FindByStackID/FindByStackIDAndChannelID/Delete

### 3. Persistence — Migrations

**Status: MISSING**

- `internal/modules/iptv/migrations/011_channel_slug.sql` — ALTER TABLE iptv_channels ADD COLUMN slug TEXT NOT NULL DEFAULT ''
- `internal/modules/iptv/migrations/012_channel_providers.sql` — CREATE TABLE iptv_channel_providers
- `internal/modules/iptv/migrations/013_iptv_stacks.sql` — CREATE TABLE iptv_stacks + iptv_stack_channels

### 4. Persistence — GORM models + repositories

**Status: MISSING**

- `internal/modules/iptv/adapters/persistence/models.go` — add ChannelProviderModel, IPTVStackModel, IPTVStackChannelModel; update ChannelModel with Slug
- `internal/modules/iptv/adapters/persistence/repositories.go` — add GormChannelProviderRepository, GormIPTVStackRepository, GormIPTVStackChannelRepository

### 5. CQRS — ChannelProvider commands + queries

**Status: MISSING**

- `internal/modules/iptv/app/commands/channel_provider.go` (new)
  - CreateChannelProviderHandler / UpdateChannelProviderHandler / DeleteChannelProviderHandler
- `internal/modules/iptv/app/queries/list_channel_providers.go` (new)
  - ListChannelProvidersHandler(channelID) → []ChannelProviderReadModel

### 6. CQRS — IPTVStack commands + queries

**Status: MISSING**

- `internal/modules/iptv/app/commands/iptv_stack.go` (new)
  - CreateIPTVStackHandler / UpdateIPTVStackHandler / DeleteIPTVStackHandler
  - DeployIPTVStackHandler — generates YAML, SSH-writes to node, runs docker compose up -d --remove-orphans
  - AddChannelToIPTVStackHandler / RemoveChannelFromIPTVStackHandler → marks stack pending
- `internal/modules/iptv/app/queries/list_iptv_stacks.go` (new)
  - ListIPTVStacksHandler / GetIPTVStackHandler

### 7. HTTP — Channel providers UI (on channel detail page)

**Status: MISSING**

- `internal/modules/iptv/adapters/http/handlers.go` — new routes for provider CRUD
- `internal/modules/iptv/adapters/http/templates.templ` — providers section on channel detail page

### 8. HTTP — IPTV Stacks UI

**Status: MISSING**

- New routes: /iptv/stacks, /iptv/stacks/new, /iptv/stacks/{id}
- Templates: stack list page, create form, detail page with channel assignment + deploy button

### 9. Playlist integration — handleChannelResolve

**Status: DIVERGED**

- `internal/modules/iptv/adapters/nats/stb_bridge.go:391` — currently returns `ch.StreamURL` directly
- Needs: check `ChannelProviderRepository.FindByChannelID(ch.ID)` → pick lowest priority active provider → `ResolveURL(template, ch.Slug, token)` → fallback to `ch.StreamURL`
- STBBridge needs `providerRepo domain.ChannelProviderRepository` injected

### 10. Wiring

**Status: MISSING**

- `internal/app/wire_iptv.go` — wire new repos + handlers + inject into STBBridge

## Skipped (future scope)

- Token rotation / encrypted token storage
- Provider failover on stale segments
- Load-balancer prefix routing
