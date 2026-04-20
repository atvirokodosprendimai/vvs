---
tldr: Each IPTV channel has multiple source providers; VVS manages a Docker Swarm stack that transcodes internal sources to HLS and serves them alongside external provider URLs.
---

# IPTV Multi-Provider Channels

A channel can be sourced from multiple providers — internal (ffmpeg transcoding) or external (direct URL). VVS manages a Docker Swarm `IPTVStack` that auto-regenerates its compose when providers or channel assignments change.

## Target

ISP operates IPTV channels that may have multiple upstream sources: their own transcoding infrastructure and/or third-party providers. VVS needs to:
- Track providers per channel, each with a URL template containing `{channel}` and optionally `{token}` placeholders
- Deploy a managed swarm stack (Caddy + N ffmpeg) for internal HLS delivery
- Reconstruct provider URLs for playlist generation to STBs

## Behaviour

### Channel Providers

- `Channel.StreamURL` is kept as legacy default; providers are additive
- A `ChannelProvider` belongs to one channel; a channel has zero or many providers
- Provider fields:
  - `Name` — human label ("Sky HD", "Internal ffmpeg", "Backup")
  - `URLTemplate` — URL with `{channel}` and optional `{token}` placeholder  
    e.g. `http://host:8080/{token}/{channel}/stream.m3u8`  
    e.g. `http://host/{channel}/index.m3u8`
  - `Type` — `internal` | `external`
  - `Priority` — lower = preferred; used for playlist ordering and failover
  - `Active` bool
- At URL-construction time VVS substitutes `{channel}` with the channel's slug and `{token}` with the provider's token field

### Channel Slug

- `Channel` gains a `Slug` field — URL-safe identifier used as `{channel}` in templates and as the filesystem path segment under `/dev/shm/{slug}/`
- Default: slugified form of `Channel.Name` (`BBC One` → `bbc-one`)
- Editable; must be unique within the VVS instance

### IPTVStack (managed)

- `IPTVStack` entity: cluster, target node, WAN macvlan network, overlay network, set of `{ChannelID, ProviderID}` pairs
- Stack status: `pending` | `deploying` | `running` | `error`
- When any of the following change, VVS marks the stack `pending` and offers re-deploy:
  - A provider is added/removed/edited for a channel assigned to this stack
  - A channel is added/removed from the stack
- Re-deploy rewrites `docker-compose.yml` on the node and runs `docker compose up -d --remove-orphans`

### Generated compose

For each `{channel, provider}` where `type=internal`:
```yaml
ffmpeg-{slug}:
  image: jrottenberg/ffmpeg:4.4-alpine
  network_mode: host          # multicast access; no IP needed
  restart: unless-stopped
  volumes:
    - /dev/shm/{slug}:/data
  entrypoint: >
    sh -c "mkdir -p /data && ffmpeg -re -i {resolved_source_url}
    -c:v copy -c:a copy -f hls
    -hls_time 2 -hls_list_size 5
    -hls_flags delete_segments+append_list
    -hls_segment_filename /data/seg%d.ts
    /data/stream.m3u8"
```

One Caddy service (always present):
```yaml
caddy:
  image: caddy:alpine
  entrypoint: sh -c "caddy file-server --root /data --browse --listen :80"
  restart: unless-stopped
  volumes:
    - /dev/shm:/data           # serves all channel subdirs
  networks:
    wan:
      ipv4_address: {wan_ip}   # public IP from macvlan
    {overlay_name}:
```

Networks block references both as `external: true`.

### Playlist integration

- When VVS generates an M3U playlist for a subscription/STB:
  - For channels with an assigned internal provider → URL = `http://{caddy_ip}/{slug}/stream.m3u8`
  - For channels with an assigned external provider → URL constructed from `URLTemplate` substitution
  - Fallback: `Channel.StreamURL` if no provider is assigned

## Design

### New tables

```
iptv_channel_providers
  id TEXT PK
  channel_id TEXT FK → iptv_channels.id
  name TEXT
  url_template TEXT
  token TEXT          -- substituted as {token}
  type TEXT           -- internal | external
  priority INTEGER DEFAULT 0
  active BOOLEAN DEFAULT true
  created_at DATETIME
  updated_at DATETIME

iptv_stacks
  id TEXT PK
  name TEXT
  cluster_id TEXT FK → swarm_clusters.id
  node_id TEXT FK → swarm_nodes.id
  wan_network_id TEXT FK → swarm_networks.id
  overlay_network_id TEXT FK → swarm_networks.id
  wan_ip TEXT
  status TEXT DEFAULT 'pending'
  last_deployed_at DATETIME
  created_at DATETIME
  updated_at DATETIME

iptv_stack_channels
  id TEXT PK
  stack_id TEXT FK → iptv_stacks.id
  channel_id TEXT FK → iptv_channels.id
  provider_id TEXT FK → iptv_channel_providers.id
```

### URL template substitution

```go
func ResolveURL(template, channelSlug, token string) string {
    s := strings.ReplaceAll(template, "{channel}", channelSlug)
    s = strings.ReplaceAll(s, "{token}", token)
    return s
}
```

### Compose generator

```go
func GenerateIPTVCompose(stack IPTVStack, channels []IPTVStackChannel) string
```

Iterates `channels`, emits one `ffmpeg-{slug}` service per internal provider, one `caddy` service, networks block. Stored at `/opt/vvs/stacks/{stack.Name}/docker-compose.yml` on the node via SSH (same path pattern as SwarmStack).

### CQRS

- `CreateIPTVStackCommand` / `UpdateIPTVStackCommand` / `DeleteIPTVStackCommand`
- `DeployIPTVStackCommand` — generates YAML, SSH-writes file, runs `docker compose up -d --remove-orphans`
- `AddChannelToIPTVStackCommand` / `RemoveChannelFromIPTVStackCommand` → marks stack `pending`
- `CreateChannelProviderCommand` / `UpdateChannelProviderCommand` / `DeleteChannelProviderCommand`
- `ListIPTVStacksQuery` / `GetIPTVStackQuery`
- `ListChannelProvidersQuery`

## Verification

- Creating an internal provider for channel "BBC One" (slug: `bbc-one`) and generating compose produces a `ffmpeg-bbc-one` service with the resolved source URL and volume `/dev/shm/bbc-one:/data`
- Caddy service mounts `/dev/shm:/data` and is attached to both WAN macvlan and overlay
- After deploy, `http://{wan_ip}/bbc-one/stream.m3u8` returns 200 (once ffmpeg has written first segments)
- Adding a channel to a running stack marks it `pending`; re-deploy adds the new ffmpeg service without restarting existing ones (`--remove-orphans`)
- External provider URL `http://sky.tv/{token}/{channel}/stream.m3u8` with slug=`bbc-one` and token=`abc123` resolves to `http://sky.tv/abc123/bbc-one/stream.m3u8`
- M3U playlist for a subscription with internal provider uses `http://{caddy_ip}/{slug}/stream.m3u8`

## Friction

- `network_mode: host` on ffmpeg containers means they can't join user-defined Docker networks — acceptable since they communicate with Caddy only via `/dev/shm`, not via network
- `/dev/shm` size is limited by host RAM (default 50% of RAM on Linux). High channel counts need either a larger host or `--shm-size` set on the container
- `{[?]}` HLS segment cleanup: `delete_segments` flag removes old segments but a crashed ffmpeg leaves stale `.ts` files. Consider a cleanup sidecar or inotify-based watchdog
- `{[?]}` ffmpeg restart storms: if all N ffmpeg containers restart simultaneously (node reboot), the node briefly has no streams. A staggered start delay per service could help

## Interactions

- Depends on [[spec - docker swarm - swarm cluster management with ssh transport and overlay macvlan networks]]
- Extends `internal/modules/iptv/domain/channel.go` (adds `Slug` field + ChannelProvider domain type)
- Re-uses `composeUp` SSH deploy pattern from swarm_stack_commands.go
- Affects M3U playlist generation (subscriptions must resolve provider URLs)

## Mapping

> [[internal/modules/iptv/domain/channel.go]]
> [[internal/modules/iptv/adapters/persistence/models.go]]
> [[internal/modules/docker/app/commands/swarm_stack_commands.go]]

## Future

- `{[?]}` Load-balancer prefix routing: a second Caddy in front mapping `/prefix/{channel}/*` to either internal Caddy or external provider — enables per-subscription provider selection at the URL level
- `{[?]}` Provider failover: if primary provider's stream goes stale (no new `.ts` segments), auto-switch to next priority provider
- `{[?]}` Token rotation: encrypted storage of provider tokens, rotation UI
