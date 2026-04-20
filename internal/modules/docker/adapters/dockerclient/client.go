package dockerclient

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/compose-spec/compose-go/v2/graph"
	"github.com/compose-spec/compose-go/v2/loader"
	composetypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockervolume "github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Client implements domain.DockerClient using the Docker SDK.
type Client struct {
	inner *dockerclient.Client
}

// Factory implements domain.DockerClientFactory.
type Factory struct{}

func (f *Factory) ForNode(node *domain.DockerNode) (domain.DockerClient, error) {
	return New(node.Host, node.TLSCert, node.TLSKey, node.TLSCA)
}

// New creates a Docker client for the given host.
// For local nodes, pass the unix socket path (e.g. "unix:///var/run/docker.sock").
// For remote nodes, pass a tcp:// URL and optional TLS PEM bytes.
func New(host string, tlsCert, tlsKey, tlsCA []byte) (*Client, error) {
	opts := []dockerclient.Opt{
		dockerclient.WithHost(host),
		dockerclient.WithAPIVersionNegotiation(),
	}

	if len(tlsCert) > 0 && len(tlsKey) > 0 {
		tlsCfg, err := tlsConfigFromBytes(tlsCA, tlsCert, tlsKey)
		if err != nil {
			return nil, fmt.Errorf("build TLS config: %w", err)
		}
		opts = append(opts, dockerclient.WithHTTPClient(&http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		}))
	}

	inner, err := dockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("create docker client for %s: %w", host, err)
	}
	return &Client{inner: inner}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.inner.Ping(ctx)
	return err
}

// Deploy creates networks, volumes, then containers for a compose project.
// Containers are started in dependency order.
func (c *Client) Deploy(ctx context.Context, projectName, composeYAML string) error {
	project, err := parseCompose(ctx, projectName, composeYAML)
	if err != nil {
		return err
	}

	// Create project-scoped networks
	for netName, netCfg := range project.Networks {
		fullName := projectName + "_" + netName
		if bool(netCfg.External) {
			continue // pre-existing external network
		}
		_, err := c.inner.NetworkCreate(ctx, fullName, network.CreateOptions{
			Driver: netCfg.Driver,
			Labels: composeLabels(projectName, "network", netName),
		})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create network %s: %w", fullName, err)
		}
	}

	// Create named volumes
	for volName, volCfg := range project.Volumes {
		if bool(volCfg.External) {
			continue
		}
		fullName := projectName + "_" + volName
		_, err := c.inner.VolumeCreate(ctx, dockervolume.CreateOptions{
			Name:   fullName,
			Driver: volCfg.Driver,
			Labels: composeLabels(projectName, "volume", volName),
		})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create volume %s: %w", fullName, err)
		}
	}

	// Start services in dependency order
	return graph.InDependencyOrder(ctx, project, func(ctx context.Context, svcName string, svc composetypes.ServiceConfig) error {
		return c.createAndStartContainer(ctx, projectName, svcName, svc, project)
	})
}

func (c *Client) createAndStartContainer(ctx context.Context, projectName, svcName string, svc composetypes.ServiceConfig, project *composetypes.Project) error {
	containerName := projectName + "_" + svcName + "_1"

	// Pull image if not present
	if svc.Image != "" {
		reader, err := c.inner.ImagePull(ctx, svc.Image, image.PullOptions{})
		if err != nil {
			return fmt.Errorf("pull image %s: %w", svc.Image, err)
		}
		io.Copy(io.Discard, reader) //nolint:errcheck
		reader.Close()
	}

	// Build port bindings
	portSet, portBindings, err := buildPortBindings(svc.Ports)
	if err != nil {
		return fmt.Errorf("build port bindings for %s: %w", svcName, err)
	}

	// Build environment
	env := make([]string, 0, len(svc.Environment))
	for k, v := range svc.Environment {
		if v != nil {
			env = append(env, k+"="+*v)
		}
	}

	// Build network config: connect to first project network
	netConfig := &network.NetworkingConfig{}
	for netName := range svc.Networks {
		fullNetName := projectName + "_" + netName
		netConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			fullNetName: {},
		}
		break
	}
	if len(svc.Networks) == 0 && len(project.Networks) > 0 {
		// Default network
		for netName := range project.Networks {
			fullNetName := projectName + "_" + netName
			netConfig.EndpointsConfig = map[string]*network.EndpointSettings{
				fullNetName: {},
			}
			break
		}
	}

	labels := composeLabels(projectName, "service", svcName)
	labels["com.docker.compose.container-number"] = "1"

	// Remove existing container with same name (idempotent redeploy)
	_ = c.inner.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})

	resp, err := c.inner.ContainerCreate(ctx,
		&container.Config{
			Image:        svc.Image,
			Env:          env,
			Labels:       labels,
			ExposedPorts: portSet,
		},
		&container.HostConfig{
			PortBindings: portBindings,
			RestartPolicy: container.RestartPolicy{
				Name: restartPolicy(svc.Restart),
			},
		},
		netConfig,
		nil,
		containerName,
	)
	if err != nil {
		return fmt.Errorf("create container %s: %w", containerName, err)
	}

	if err := c.inner.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", containerName, err)
	}
	return nil
}

func (c *Client) ListContainers(ctx context.Context, projectName string) ([]domain.ContainerInfo, error) {
	f := filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+projectName))
	list, err := c.inner.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ContainerInfo, len(list))
	for i, c := range list {
		ports := make([]string, len(c.Ports))
		for j, p := range c.Ports {
			ports[j] = fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
			if p.PublicPort > 0 {
				ports[j] = fmt.Sprintf("%s->%d/%s", fmt.Sprintf("0.0.0.0:%d", p.PublicPort), p.PrivatePort, p.Type)
			}
		}
		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		out[i] = domain.ContainerInfo{
			ID:     c.ID,
			Name:   name,
			Image:  c.Image,
			Status: c.Status,
			State:  c.State,
			Ports:  ports,
		}
	}
	return out, nil
}

func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.inner.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	return c.inner.ContainerStop(ctx, containerID, container.StopOptions{})
}

func (c *Client) RemoveContainers(ctx context.Context, projectName string) error {
	f := filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+projectName))
	list, err := c.inner.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return err
	}
	for _, ct := range list {
		if err := c.inner.ContainerRemove(ctx, ct.ID, container.RemoveOptions{Force: true}); err != nil {
			return fmt.Errorf("remove container %s: %w", ct.ID[:12], err)
		}
	}
	return nil
}

// StreamLogs returns a ReadCloser that streams stdout+stderr from a container.
// The stream uses Docker's multiplexed format; use StripMultiplexHeader to read lines.
func (c *Client) StreamLogs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error) {
	return c.inner.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: false,
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func parseCompose(ctx context.Context, projectName, yaml string) (*composetypes.Project, error) {
	details := composetypes.ConfigDetails{
		ConfigFiles: []composetypes.ConfigFile{
			{Content: []byte(yaml), Filename: "docker-compose.yml"},
		},
		WorkingDir:  "/",
		Environment: map[string]string{"COMPOSE_PROJECT_NAME": projectName},
	}
	project, err := loader.LoadWithContext(ctx, details, func(o *loader.Options) {
		o.SkipValidation = false
		o.SkipNormalization = false
	})
	if err != nil {
		return nil, fmt.Errorf("parse compose YAML: %w", err)
	}
	project.Name = projectName
	return project, nil
}

func tlsConfigFromBytes(caBytes, certBytes, keyBytes []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse TLS cert/key: %w", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if len(caBytes) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
		tlsCfg.RootCAs = pool
	}
	return tlsCfg, nil
}

func composeLabels(projectName, resourceType, name string) map[string]string {
	return map[string]string{
		"com.docker.compose.project":       projectName,
		"com.docker.compose.resource_type": resourceType,
		"com.docker.compose.name":          name,
	}
}

func buildPortBindings(ports []composetypes.ServicePortConfig) (nat.PortSet, nat.PortMap, error) {
	portSet := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		containerPort, err := nat.NewPort(proto, fmt.Sprintf("%d", p.Target))
		if err != nil {
			return nil, nil, err
		}
		portSet[containerPort] = struct{}{}
		if p.Published != "" {
			portBindings[containerPort] = []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: p.Published},
			}
		}
	}
	return portSet, portBindings, nil
}

func restartPolicy(policy string) container.RestartPolicyMode {
	switch policy {
	case "always":
		return container.RestartPolicyAlways
	case "unless-stopped":
		return container.RestartPolicyUnlessStopped
	case "on-failure":
		return container.RestartPolicyOnFailure
	default:
		return container.RestartPolicyDisabled
	}
}

// ReadMultiplexLine reads one line from a Docker multiplexed log stream.
// Docker log streams are framed: [stream_type(1)] [padding(3)] [size(4)] [payload].
// Returns io.EOF when the stream ends.
func ReadMultiplexLine(r *bufio.Reader) (string, error) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(r, header); err != nil {
		return "", err
	}
	size := binary.BigEndian.Uint32(header[4:])
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", err
	}
	return strings.TrimRight(string(payload), "\n\r"), nil
}
