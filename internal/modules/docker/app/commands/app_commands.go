package commands

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	dockerbuild "github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// ── BuildDockerApp ─────────────────────────────────────────────────────────────

type BuildDockerAppCommand struct {
	AppID string
}

type BuildDockerAppHandler struct {
	appRepo   domain.DockerAppRepository
	publisher events.EventPublisher
}

func NewBuildDockerAppHandler(
	appRepo domain.DockerAppRepository,
	pub events.EventPublisher,
) *BuildDockerAppHandler {
	return &BuildDockerAppHandler{appRepo: appRepo, publisher: pub}
}

// Handle triggers the build pipeline in a goroutine and returns immediately.
// Progress is streamed via NATS isp.docker.app.build.{appID}.
func (h *BuildDockerAppHandler) Handle(ctx context.Context, cmd BuildDockerAppCommand) error {
	app, err := h.appRepo.FindByID(ctx, cmd.AppID)
	if err != nil {
		return err
	}
	app.MarkBuilding()
	if err := h.appRepo.Save(ctx, app); err != nil {
		return err
	}
	h.publishStatus(ctx, app)

	go func() {
		bgCtx := context.Background()
		if err := h.runPipeline(bgCtx, app); err != nil {
			app.MarkError(err.Error())
			_ = h.appRepo.Save(bgCtx, app)
			h.publishStatus(bgCtx, app)
		}
	}()
	return nil
}

func (h *BuildDockerAppHandler) runPipeline(ctx context.Context, app *domain.DockerApp) error {
	logSubj := events.DockerAppBuildLog.Format(app.ID)
	emit := func(line string) {
		h.publisher.Publish(ctx, logSubj, events.DomainEvent{Type: "log", Data: []byte(line)})
	}

	// ── 1. Clone ──────────────────────────────────────────────────────────────
	tmpDir, err := os.MkdirTemp("", "vvs-app-build-*")
	if err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneURL := injectCredentials(app.RepoURL, app.RegUser, app.RegPass)
	emit(fmt.Sprintf("Cloning %s @ %s…", app.RepoURL, app.Branch))
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", app.Branch, cloneURL, tmpDir)
	cloneCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, string(out))
	}
	emit("Clone OK")

	// ── 2. Build ──────────────────────────────────────────────────────────────
	dockerCli, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost("unix:///var/run/docker.sock"),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer dockerCli.Close()

	imageTag := buildImageTag(app)
	tsTag := imageTag + ":" + time.Now().UTC().Format("20060102-150405")
	_ = tsTag // used below after push

	buildArgs := make(map[string]*string, len(app.BuildArgs))
	for _, kv := range app.BuildArgs {
		v := kv.Value
		buildArgs[kv.Key] = &v
	}

	emit("Building Docker image…")
	buildCtxTar, err := dirToTar(tmpDir)
	if err != nil {
		return fmt.Errorf("create build context tar: %w", err)
	}

	buildResp, err := dockerCli.ImageBuild(ctx, buildCtxTar, dockerbuild.ImageBuildOptions{
		Tags:      []string{imageTag + ":latest", tsTag},
		BuildArgs: buildArgs,
		Remove:    true,
	})
	if err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	defer buildResp.Body.Close()
	if err := streamDockerOutput(buildResp.Body, emit); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	emit("Build OK")

	// ── 3. Push ───────────────────────────────────────────────────────────────
	app.MarkPushing()
	_ = h.appRepo.Save(ctx, app)
	h.publishStatus(ctx, app)

	registryAuth, err := registryAuthHeader(app.RegUser, app.RegPass, registryHost(app.RepoURL))
	if err != nil {
		return fmt.Errorf("registry auth: %w", err)
	}

	emit("Pushing image to registry…")
	for _, tag := range []string{imageTag + ":latest", tsTag} {
		pushResp, err := dockerCli.ImagePush(ctx, tag, image.PushOptions{RegistryAuth: registryAuth})
		if err != nil {
			return fmt.Errorf("docker push %s: %w", tag, err)
		}
		if err := streamDockerOutput(pushResp, emit); err != nil {
			pushResp.Close()
			return fmt.Errorf("push failed: %w", err)
		}
		pushResp.Close()
	}
	emit("Push OK")

	// ── 4. Deploy ─────────────────────────────────────────────────────────────
	app.MarkDeploying(imageTag + ":latest")
	_ = h.appRepo.Save(ctx, app)
	h.publishStatus(ctx, app)

	emit("Deploying container…")
	if err := deployContainer(ctx, dockerCli, app); err != nil {
		return fmt.Errorf("deploy container: %w", err)
	}

	emit("Done")
	app.MarkRunning()
	_ = h.appRepo.Save(ctx, app)
	h.publishStatus(ctx, app)
	return nil
}

// ── StopDockerApp ─────────────────────────────────────────────────────────────

type StopDockerAppCommand struct {
	AppID string
}

type StopDockerAppHandler struct {
	appRepo   domain.DockerAppRepository
	publisher events.EventPublisher
}

func NewStopDockerAppHandler(appRepo domain.DockerAppRepository, pub events.EventPublisher) *StopDockerAppHandler {
	return &StopDockerAppHandler{appRepo: appRepo, publisher: pub}
}

func (h *StopDockerAppHandler) Handle(ctx context.Context, cmd StopDockerAppCommand) error {
	app, err := h.appRepo.FindByID(ctx, cmd.AppID)
	if err != nil {
		return err
	}
	dockerCli, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost("unix:///var/run/docker.sock"),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer dockerCli.Close()

	_ = dockerCli.ContainerStop(ctx, app.ContainerName, container.StopOptions{})
	app.MarkStopped()
	_ = h.appRepo.Save(ctx, app)
	h.publishStatus(ctx, app)
	return nil
}

// ── RemoveDockerApp ───────────────────────────────────────────────────────────

type RemoveDockerAppCommand struct {
	AppID string
}

type RemoveDockerAppHandler struct {
	appRepo   domain.DockerAppRepository
	publisher events.EventPublisher
}

func NewRemoveDockerAppHandler(appRepo domain.DockerAppRepository, pub events.EventPublisher) *RemoveDockerAppHandler {
	return &RemoveDockerAppHandler{appRepo: appRepo, publisher: pub}
}

func (h *RemoveDockerAppHandler) Handle(ctx context.Context, cmd RemoveDockerAppCommand) error {
	app, err := h.appRepo.FindByID(ctx, cmd.AppID)
	if err != nil {
		return err
	}
	dockerCli, err := dockerclient.NewClientWithOpts(
		dockerclient.WithHost("unix:///var/run/docker.sock"),
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer dockerCli.Close()

	_ = dockerCli.ContainerStop(ctx, app.ContainerName, container.StopOptions{})
	_ = dockerCli.ContainerRemove(ctx, app.ContainerName, container.RemoveOptions{Force: true})
	return h.appRepo.Delete(ctx, app.ID)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// deployContainer stops/removes any existing container and starts a new one.
func deployContainer(ctx context.Context, cli *dockerclient.Client, app *domain.DockerApp) error {
	// Remove existing (force stops + removes)
	_ = cli.ContainerStop(ctx, app.ContainerName, container.StopOptions{})
	_ = cli.ContainerRemove(ctx, app.ContainerName, container.RemoveOptions{Force: true})

	// Build env slice
	env := make([]string, 0, len(app.EnvVars))
	for _, kv := range app.EnvVars {
		env = append(env, kv.Key+"="+kv.Value)
	}

	// Build port bindings
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, pm := range app.Ports {
		proto := pm.Proto
		if proto == "" {
			proto = "tcp"
		}
		p := nat.Port(pm.Container + "/" + proto)
		exposedPorts[p] = struct{}{}
		portBindings[p] = []nat.PortBinding{{HostPort: pm.Host}}
	}

	// Build bind mounts
	binds := make([]string, 0, len(app.Volumes))
	for _, vm := range app.Volumes {
		binds = append(binds, vm.Host+":"+vm.Container)
	}

	// Build network config (connect to first network; additional networks attached after create)
	var netConfig *network.NetworkingConfig
	if len(app.Networks) > 0 {
		netConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				app.Networks[0]: {},
			},
		}
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        app.ImageRef,
			Env:          env,
			ExposedPorts: exposedPorts,
		},
		&container.HostConfig{
			PortBindings: portBindings,
			Binds:        binds,
			RestartPolicy: container.RestartPolicy{
				Name: container.RestartPolicyMode(app.RestartPolicy),
			},
		},
		netConfig,
		nil,
		app.ContainerName,
	)
	if err != nil {
		return fmt.Errorf("container create: %w", err)
	}

	// Attach additional networks
	for _, netName := range app.Networks[1:] {
		_ = cli.NetworkConnect(ctx, netName, resp.ID, nil)
	}

	return cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
}

// buildImageTag derives the registry image path from the repo URL.
// https://gitea.host/owner/repo → gitea.host/owner/repo
func buildImageTag(app *domain.DockerApp) string {
	u := strings.TrimPrefix(app.RepoURL, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimSuffix(u, ".git")
	// strip credentials if embedded
	if at := strings.Index(u, "@"); at >= 0 {
		u = u[at+1:]
	}
	return u
}

// registryHost extracts the hostname from a repo URL.
func registryHost(repoURL string) string {
	u := strings.TrimPrefix(repoURL, "https://")
	u = strings.TrimPrefix(u, "http://")
	if idx := strings.Index(u, "/"); idx >= 0 {
		u = u[:idx]
	}
	// strip user:pass@
	if at := strings.Index(u, "@"); at >= 0 {
		u = u[at+1:]
	}
	return u
}

// injectCredentials embeds user:pass into https URL.
func injectCredentials(repoURL, user, pass string) string {
	if user == "" && pass == "" {
		return repoURL
	}
	for _, scheme := range []string{"https://", "http://"} {
		if strings.HasPrefix(repoURL, scheme) {
			return scheme + user + ":" + pass + "@" + strings.TrimPrefix(repoURL, scheme)
		}
	}
	return repoURL
}

// registryAuthHeader returns base64-encoded Docker registry auth JSON.
func registryAuthHeader(user, pass, serverAddr string) (string, error) {
	auth := struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		Serveraddress string `json:"serveraddress"`
	}{Username: user, Password: pass, Serveraddress: "https://" + serverAddr}
	b, err := json.Marshal(auth)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// streamDockerOutput reads Docker daemon JSON stream and calls emit for each message line.
// Returns error if any error message is found in the stream.
func streamDockerOutput(r io.Reader, emit func(string)) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		line := scanner.Text()
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			emit(line)
			continue
		}
		if msg.Error != "" {
			return fmt.Errorf("%s", msg.Error)
		}
		if s := strings.TrimRight(msg.Stream, "\n"); s != "" {
			emit(s)
		}
	}
	return scanner.Err()
}

// dirToTar creates an in-memory tar archive of dir for use as Docker build context.
func dirToTar(dir string) (io.Reader, error) {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)

	go func() {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			hdr := &tar.Header{
				Name:    rel,
				Size:    info.Size(),
				Mode:    int64(info.Mode()),
				ModTime: info.ModTime(),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		})
		tw.Close()
		pw.CloseWithError(err)
	}()

	return pr, nil
}

func (h *BuildDockerAppHandler) publishStatus(ctx context.Context, app *domain.DockerApp) {
	h.publisher.Publish(ctx, events.DockerAppStatusChanged.String(), events.DomainEvent{
		Type:    "docker.app.status_changed",
		AggregateID: app.ID,
	})
}

func (h *StopDockerAppHandler) publishStatus(ctx context.Context, app *domain.DockerApp) {
	h.publisher.Publish(ctx, events.DockerAppStatusChanged.String(), events.DomainEvent{
		Type:    "docker.app.status_changed",
		AggregateID: app.ID,
	})
}
