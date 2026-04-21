---
tldr: Deploy apps from a Gitea repo — VVS clones, builds the Docker image, pushes to Gitea registry, and runs the container with UI-defined env vars, ports, volumes, and networks
---

# Docker App — Git Source Deploy

Extension of the Docker orchestrator. Operators register a Gitea repo URL; VVS clones it, builds the image from the Dockerfile, pushes to the Gitea container registry, then runs the container using deployment config defined entirely in the VVS UI.

## Target

ISP operators who host custom apps (bots, daemons, portals, automation scripts) in Gitea repos. Instead of SSH + `docker build` + `docker run` by hand, they get a one-click build-and-deploy pipeline inside VVS. No separate CI/CD tool needed.

## Behaviour

### App Registration

- Operator creates a `DockerApp` in VVS with:
  - Gitea repo URL (e.g. `https://gitea.example.com/owner/repo`)
  - Branch (default: `main`)
  - Gitea registry credentials (username + password — used for both git auth and registry push/pull)
  - Build args (optional K=V pairs passed as `--build-arg`)
  - Deployment config: env vars, port bindings, volume mounts, network attachments, restart policy
- The repo must contain a `Dockerfile` at the root; no compose file needed
- `ContainerName` is derived from `DockerApp.Name` (slugified); container is always replaced on redeploy

### Build Pipeline (triggered manually or via webhook)

1. Clone repo to `os.MkdirTemp` — shallow clone (`--depth 1`), branch as specified
   - Credentials embedded in URL: `https://user:pass@gitea.host/owner/repo` (never written to disk)
2. Build image via Docker SDK `ImageBuild` — tar the cloned directory as build context
   - Image tag: `gitea.host/owner/repo:latest` (also tagged `:<yyyymmdd-hhmmss>` for history)
   - Build args passed through
3. Push image to Gitea registry via Docker SDK `ImagePush` with registry auth
4. Stop and remove existing container (same `ContainerName`) if running
5. Create and start new container with configured env, ports, volumes, networks, restart policy
6. `os.RemoveAll(tmpdir)` on finish (deferred — always runs even on error)
7. Each step streams build output lines to UI via SSE; status updates after each step

### Status Lifecycle

`idle → building → pushing → deploying → running | error`

- On error: `ErrorMsg` stored, pipeline stops, tmp dir cleaned
- Running container can be stopped (→ `stopped`) or removed (→ `idle`)
- Redeploy always goes through full pipeline from `building`

### Deployment Config (UI-defined)

All runtime config lives in VVS, not in the repo:

| Field | Type | Notes |
|-------|------|-------|
| EnvVars | K=V list | Injected as container env |
| Ports | `{host, container, proto}` list | e.g. `8080:80/tcp` |
| Volumes | `{host, container}` list | Bind mounts only |
| Networks | string list | Names of existing Docker networks |
| RestartPolicy | enum | `no \| always \| unless-stopped \| on-failure` |

### Webhook (optional)

- VVS exposes `POST /docker/apps/{id}/webhook` — triggers full build pipeline
- "Register Webhook" button in app detail calls Gitea API `POST /repos/{owner}/{repo}/hooks` pointing at the VVS URL
- No secret verification in v1 — webhook endpoint is unauthenticated (obscurity via ID only)

## Design

### Entity

```go
type DockerApp struct {
    ID            string
    Name          string        // human label; slug used as ContainerName
    RepoURL       string        // https://gitea.host/owner/repo
    Branch        string        // default: main
    RegUser       string        // Gitea username (git auth + registry)
    RegPass       string        // AES-encrypted; same key as DockerEncKey
    BuildArgs     []KV          // JSON [{Key, Value}]
    EnvVars       []KV          // JSON [{Key, Value}]
    Ports         []PortMap     // JSON [{Host, Container, Proto}]
    Volumes       []VolumeMount // JSON [{Host, Container}]
    Networks      []string      // JSON []string
    RestartPolicy string
    ContainerName string        // slugified Name; unique per VVS instance
    ImageRef      string        // last successfully pushed image ref
    Status        string        // idle|building|pushing|deploying|running|error|stopped
    ErrorMsg      string
    LastBuiltAt   time.Time
}
```

### Build Goroutine

```
BuildAppCmd
  → set status = building
  → clone repo (os.MkdirTemp + git clone --depth 1)
  → publish build log lines → isp.docker.app.build.{id}
  → ImageBuild (tar buildContext, BuildArgs, tag=imageRef)
  → set status = pushing
  → ImagePush (registryAuth = base64(user:pass))
  → set status = deploying
  → ContainerStop + ContainerRemove (if exists, same ContainerName)
  → ContainerCreate (Env, PortBindings, HostConfig.Binds, NetworkingConfig, RestartPolicy)
  → ContainerStart
  → set status = running | error
  → defer os.RemoveAll(tmpdir)
```

Build log lines flow: goroutine reads Docker API response stream → publishes each line to NATS `isp.docker.app.build.{id}` → SSE handler subscribes and patches `#app-build-log` element.

### Image Tag Strategy

- Primary tag: `gitea.host/owner/repo:latest` — always updated on successful push
- Secondary tag: `gitea.host/owner/repo:YYYYMMDD-HHMMSS` — kept for rollback visibility (not auto-cleaned in v1)

### Registry Auth

`types.ImagePushOptions.RegistryAuth` = base64-encoded JSON `{"username":"...","password":"...","serveraddress":"https://gitea.host"}` — standard Docker registry auth format.

Same format used for `docker login` credentials cache; no extra library needed.

### Git Clone Auth

URL-embedded credentials: `https://user:pass@gitea.host/owner/repo`  
Constructed in-memory, passed to `git clone` subprocess (`os/exec`).  
`RegPass` decrypted in-memory, used, not stored in any temp file.  
Requires `git` binary in the VVS container image.

### NATS Subjects

```
isp.docker.app.all          — app list changed (create/update/delete)
isp.docker.app.status       — single app status change
isp.docker.app.build.{id}   — build/push/deploy log line (per app ID)
```

### UI

- Nav: add "Apps" item under Docker section (guarded by `ModuleDocker`)
- App list page: name, status badge (colour per status), last built timestamp, image ref, Build / Stop / Remove buttons
- App form (create/edit):
  - **Source** card: repo URL, branch, registry user, registry password (masked)
  - **Build** card: build args K=V table (add/remove rows)
  - **Runtime** card: env vars K=V table, ports table `{host:container/proto}`, volumes table `{host:container}`, network multi-select (populated from existing Docker networks), restart policy dropdown
- App detail page: build log terminal (dark, auto-scroll, SSE), status timeline, "Register Gitea Webhook" button

## Verification

- Create app with public Gitea repo containing a `FROM alpine` Dockerfile → Build runs → image appears in Gitea Packages → container starts → status = running
- Env var `FOO=bar` configured → `docker exec <name> env` shows `FOO=bar`
- Port `8080:80/tcp` configured → `curl localhost:8080` reaches container
- Volume `/tmp/data:/data` configured → file written in container at `/data` persists at host `/tmp/data`
- Network `my-overlay` configured → container attached to that network
- Redeploy → old container removed, new container started with updated image
- Invalid Gitea credentials → build fails at push step, `ErrorMsg` set, status = error, tmp dir cleaned
- Repo has no Dockerfile → `docker build` fails, error surfaced in log stream
- "Register Gitea Webhook" → hook appears in Gitea repo settings → push to repo triggers build pipeline

## Friction

- **docker.sock access**: VVS must mount `/var/run/docker.sock`; same requirement as existing Docker module
- **Image accumulation**: secondary `:YYYYMMDD-HHMMSS` tags grow unboundedly in v1 — no auto-prune; operator must manage manually via Gitea UI
- **Build duration**: large images take minutes; status stays `building` until complete; no cancel in v1
- **Webhook unauthenticated**: endpoint secured only by app ID UUID; acceptable for internal Gitea, not for public exposure
- **git binary required**: `os/exec git clone` needs `git` installed in VVS container image; add to Dockerfile
- **Private Gitea with self-signed cert**: `git clone` will fail SSL verification; workaround: `GIT_SSL_NO_VERIFY=1` env var on the subprocess — surface as a per-app toggle

## Interactions

- Extends [[spec - docker - multi-node orchestrator with compose yaml and live logs]]
- Reuses `DockerEncKey` (AES-256-GCM) for `RegPass` encryption
- Same SSE log streaming pattern as container logs (`isp.docker.logs.*`)
- `ModuleDocker` permission gates all app routes

## Mapping

> [[internal/modules/docker/domain/docker_client.go]]
> [[internal/modules/docker/adapters/http/handlers.go]]
> [[internal/modules/docker/adapters/http/templates.templ]]
> [[internal/app/wire_docker.go]]

## Future

{[!] Build cancellation — cancel running build pipeline mid-step}
{[!] Image pruning — auto-remove old tags beyond N most recent}
{[?] Webhook HMAC secret — verify Gitea webhook signature}
{[?] Build cache — reuse Docker layer cache between builds via named volume}
{[?] Multi-Dockerfile — specify Dockerfile path within repo (not just root)}
{[?] Swarm stack deploy — after push, deploy as Swarm stack on a selected cluster instead of local container}
{[?] Git SSL CA — per-app option to trust self-signed Gitea cert}
