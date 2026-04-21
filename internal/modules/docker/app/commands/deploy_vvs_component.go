package commands

import (
	"context"
	"fmt"
	"strings"
	"text/template"
	"bytes"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/dockerclient"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// ── DeployVVSComponent ────────────────────────────────────────────────────────

type DeployVVSComponentCommand struct {
	// Target
	ClusterID  string
	NodeID     string
	Component  domain.VVSComponentType
	Source     domain.VVSDeploySource

	// Image source
	ImageURL   string
	RegistryID string

	// Git source
	GitURL string
	GitRef string

	// Runtime config
	NATSUrl string
	Port    int
	EnvVars map[string]string
}

type DeployVVSComponentHandler struct {
	deployRepo   domain.VVSDeploymentRepository
	nodeRepo     domain.SwarmNodeRepository
	registryRepo domain.ContainerRegistryRepository
	progress     func(string)
}

func NewDeployVVSComponentHandler(
	deployRepo domain.VVSDeploymentRepository,
	nodeRepo domain.SwarmNodeRepository,
	registryRepo domain.ContainerRegistryRepository,
) *DeployVVSComponentHandler {
	return &DeployVVSComponentHandler{
		deployRepo:   deployRepo,
		nodeRepo:     nodeRepo,
		registryRepo: registryRepo,
	}
}

func (h *DeployVVSComponentHandler) WithProgress(fn func(string)) *DeployVVSComponentHandler {
	cp := *h
	cp.progress = fn
	return &cp
}

func (h *DeployVVSComponentHandler) emit(msg string) {
	if h.progress != nil {
		h.progress(msg)
	}
}

func (h *DeployVVSComponentHandler) Handle(ctx context.Context, cmd DeployVVSComponentCommand) (*domain.VVSDeployment, error) {
	dep, err := domain.NewVVSDeployment(
		cmd.ClusterID, cmd.NodeID, cmd.Component, cmd.Source, cmd.NATSUrl, cmd.Port,
	)
	if err != nil {
		return nil, err
	}
	dep.ImageURL = cmd.ImageURL
	dep.RegistryID = cmd.RegistryID
	dep.GitURL = cmd.GitURL
	dep.GitRef = cmd.GitRef
	if cmd.EnvVars != nil {
		dep.EnvVars = cmd.EnvVars
	}

	if err := h.deployRepo.Save(ctx, dep); err != nil {
		return nil, fmt.Errorf("save deployment: %w", err)
	}

	// Run deployment in background — caller can track via status
	go func() {
		bgCtx := context.Background()
		if err := h.runDeploy(bgCtx, dep); err != nil {
			dep.MarkError(err.Error())
			_ = h.deployRepo.Save(bgCtx, dep)
			return
		}
		dep.MarkRunning()
		_ = h.deployRepo.Save(bgCtx, dep)
	}()

	return dep, nil
}

// RedeployVVSComponentCommand triggers redeployment of an existing deployment.
type RedeployVVSComponentCommand struct {
	DeploymentID string
}

type RedeployVVSComponentHandler struct {
	deployRepo   domain.VVSDeploymentRepository
	nodeRepo     domain.SwarmNodeRepository
	registryRepo domain.ContainerRegistryRepository
	progress     func(string)
}

func NewRedeployVVSComponentHandler(
	deployRepo domain.VVSDeploymentRepository,
	nodeRepo domain.SwarmNodeRepository,
	registryRepo domain.ContainerRegistryRepository,
) *RedeployVVSComponentHandler {
	return &RedeployVVSComponentHandler{
		deployRepo:   deployRepo,
		nodeRepo:     nodeRepo,
		registryRepo: registryRepo,
	}
}

func (h *RedeployVVSComponentHandler) Handle(ctx context.Context, cmd RedeployVVSComponentCommand) error {
	dep, err := h.deployRepo.FindByID(ctx, cmd.DeploymentID)
	if err != nil {
		return err
	}

	// Run in background
	go func() {
		bgCtx := context.Background()
		helper := &DeployVVSComponentHandler{
			deployRepo:   h.deployRepo,
			nodeRepo:     h.nodeRepo,
			registryRepo: h.registryRepo,
		}
		if err := helper.runDeploy(bgCtx, dep); err != nil {
			dep.MarkError(err.Error())
			_ = h.deployRepo.Save(bgCtx, dep)
			return
		}
		dep.MarkRunning()
		_ = h.deployRepo.Save(bgCtx, dep)
	}()
	return nil
}

// DeleteVVSDeploymentHandler removes a deployment record and runs docker compose down.
type DeleteVVSDeploymentCommand struct {
	ID string
}

type DeleteVVSDeploymentHandler struct {
	deployRepo domain.VVSDeploymentRepository
	nodeRepo   domain.SwarmNodeRepository
}

func NewDeleteVVSDeploymentHandler(
	deployRepo domain.VVSDeploymentRepository,
	nodeRepo domain.SwarmNodeRepository,
) *DeleteVVSDeploymentHandler {
	return &DeleteVVSDeploymentHandler{deployRepo: deployRepo, nodeRepo: nodeRepo}
}

func (h *DeleteVVSDeploymentHandler) Handle(ctx context.Context, cmd DeleteVVSDeploymentCommand) error {
	dep, err := h.deployRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	node, err := h.nodeRepo.FindByID(ctx, dep.NodeID)
	if err == nil {
		_ = composeDownAt(node, dep.ComposePath())
	}
	return h.deployRepo.Delete(ctx, dep.ID)
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (h *DeployVVSComponentHandler) runDeploy(ctx context.Context, dep *domain.VVSDeployment) error {
	node, err := h.nodeRepo.FindByID(ctx, dep.NodeID)
	if err != nil {
		return fmt.Errorf("find node %s: %w", dep.NodeID, err)
	}

	h.emit(fmt.Sprintf("Deploying vvs-%s on %s (%s)…", dep.Component, node.Name, node.SshHost))

	// docker login if registry configured
	if dep.RegistryID != "" {
		reg, err := h.registryRepo.FindByID(ctx, dep.RegistryID)
		if err != nil {
			return fmt.Errorf("find registry: %w", err)
		}
		h.emit(fmt.Sprintf("Logging in to registry %s…", reg.URL))
		loginCmd := fmt.Sprintf("echo %s | docker login %s -u %s --password-stdin 2>&1",
			shellQuote(reg.Password), shellQuote(reg.URL), shellQuote(reg.Username))
		if out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, loginCmd); err != nil {
			return fmt.Errorf("docker login: %w\n%s", err, out)
		}
		h.emit("Registry login OK")
	}

	var imageRef string

	switch dep.Source {
	case domain.VVSDeployImage:
		imageRef = dep.ImageURL
		h.emit(fmt.Sprintf("Pulling %s…", imageRef))
		pullCmd := fmt.Sprintf("docker pull %s 2>&1", shellQuote(imageRef))
		if out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, pullCmd); err != nil {
			return fmt.Errorf("docker pull: %w\n%s", err, out)
		}
		h.emit("Image pulled")

	case domain.VVSDeployGit:
		buildDir := fmt.Sprintf("/tmp/vvs-build-%s-%d", dep.ID, time.Now().Unix())
		gitRef := dep.GitRef
		if gitRef == "" {
			gitRef = "main"
		}
		imageRef = fmt.Sprintf("vvs-%s-local-%s", dep.Component, dep.ID[:8])

		h.emit(fmt.Sprintf("Cloning %s @ %s…", dep.GitURL, gitRef))
		cloneCmd := fmt.Sprintf("git clone --depth=1 --branch %s %s %s 2>&1",
			shellQuote(gitRef), shellQuote(dep.GitURL), shellQuote(buildDir))
		if out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, cloneCmd); err != nil {
			return fmt.Errorf("git clone: %w\n%s", err, out)
		}

		h.emit("Building Docker image…")
		buildCmd := fmt.Sprintf("docker build -t %s %s 2>&1", shellQuote(imageRef), shellQuote(buildDir))
		if out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, buildCmd); err != nil {
			return fmt.Errorf("docker build: %w\n%s", err, out)
		}

		// cleanup build dir
		_, _ = dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey,
			fmt.Sprintf("rm -rf %s", shellQuote(buildDir)))
		h.emit("Image built")
	}

	composeYAML, err := generateVVSCompose(dep, imageRef)
	if err != nil {
		return fmt.Errorf("generate compose: %w", err)
	}

	h.emit("Writing compose file…")
	dir := dep.ComposeDir()
	path := dep.ComposePath()
	escapedYAML := strings.ReplaceAll(composeYAML, "'", `'"'"'`)
	writeCmd := fmt.Sprintf("mkdir -p %s && printf '%%s' '%s' > %s", dir, escapedYAML, path)
	if out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, writeCmd); err != nil {
		return fmt.Errorf("write compose file: %w\n%s", err, out)
	}

	h.emit("Starting container…")
	upCmd := fmt.Sprintf("docker compose -f %s up -d --remove-orphans 2>&1", shellQuote(path))
	if out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, upCmd); err != nil {
		return fmt.Errorf("docker compose up: %w\n%s", err, out)
	}

	h.emit(fmt.Sprintf("vvs-%s deployed on port %d", dep.Component, dep.Port))
	return nil
}

// composeDownAt runs docker compose down for a compose file path.
func composeDownAt(node *domain.SwarmNode, composePath string) error {
	cmd := fmt.Sprintf("docker compose -f %s down 2>&1 || true", shellQuote(composePath))
	_, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, cmd)
	return err
}

// shellQuote wraps s in single quotes for shell safety (non-recursive, breaks on embedded singles).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

var vvsComposeTmpl = template.Must(template.New("vvs-compose").Parse(`services:
  {{ .ServiceName }}:
    image: {{ .Image }}
    restart: always
    ports:
      - "{{ .Port }}:8080"
    environment:
      - NATS_URL={{ .NATSUrl }}
{{ range $k, $v := .Extra }}      - {{ $k }}={{ $v }}
{{ end }}`))

type vvsComposeData struct {
	ServiceName string
	Image       string
	Port        int
	NATSUrl     string
	Extra       map[string]string
}

func generateVVSCompose(dep *domain.VVSDeployment, imageRef string) (string, error) {
	data := vvsComposeData{
		ServiceName: dep.ServiceName(),
		Image:       imageRef,
		Port:        dep.Port,
		NATSUrl:     dep.NATSUrl,
		Extra:       dep.EnvVars,
	}
	var buf bytes.Buffer
	if err := vvsComposeTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
