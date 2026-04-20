package dockerclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
)

// SSHSwarmFactory implements domain.SwarmClientFactory.
// It creates a SwarmDockerClient for a SwarmNode via SSH.
type SSHSwarmFactory struct{}

func (f *SSHSwarmFactory) ForSwarmNode(node *domain.SwarmNode) (domain.SwarmDockerClient, error) {
	return NewSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey)
}

// ── Swarm operations ──────────────────────────────────────────────────────────

func (c *Client) SwarmInit(ctx context.Context, vpnIP string) (managerToken, workerToken string, err error) {
	resp, err := c.inner.SwarmInit(ctx, swarm.InitRequest{
		ListenAddr:    vpnIP + ":2377",
		AdvertiseAddr: vpnIP,
		DataPathAddr:  vpnIP,
	})
	if err != nil {
		return "", "", fmt.Errorf("swarm init: %w", err)
	}
	_ = resp // returns the node ID

	info, err := c.inner.SwarmInspect(ctx)
	if err != nil {
		return "", "", fmt.Errorf("swarm inspect after init: %w", err)
	}
	return info.JoinTokens.Manager, info.JoinTokens.Worker, nil
}

func (c *Client) SwarmJoin(ctx context.Context, managerVpnIP, joinToken string) error {
	return c.inner.SwarmJoin(ctx, swarm.JoinRequest{
		ListenAddr:    "0.0.0.0:2377",
		AdvertiseAddr: "", // will be set during join negotiation
		RemoteAddrs:   []string{managerVpnIP + ":2377"},
		JoinToken:     joinToken,
	})
}

func (c *Client) SwarmLeave(ctx context.Context, force bool) error {
	return c.inner.SwarmLeave(ctx, force)
}

func (c *Client) SwarmNodeList(ctx context.Context) ([]domain.SwarmNodeInfo, error) {
	nodes, err := c.inner.NodeList(ctx, dockertypes.NodeListOptions{})
	if err != nil {
		return nil, fmt.Errorf("node list: %w", err)
	}
	out := make([]domain.SwarmNodeInfo, len(nodes))
	for i, n := range nodes {
		out[i] = domain.SwarmNodeInfo{
			ID:       n.ID,
			Hostname: n.Description.Hostname,
			Role:     string(n.Spec.Role),
			Status:   string(n.Status.State),
		}
	}
	return out, nil
}

func (c *Client) SwarmNodeRemove(ctx context.Context, dockerNodeID string) error {
	return c.inner.NodeRemove(ctx, dockerNodeID, dockertypes.NodeRemoveOptions{Force: true})
}

// ── Network operations ────────────────────────────────────────────────────────

func (c *Client) NetworkCreate(ctx context.Context, req domain.NetworkCreateRequest) (string, error) {
	ipamConfig := []network.IPAMConfig{}
	if req.Subnet != "" {
		cfg := network.IPAMConfig{Subnet: req.Subnet}
		if req.Gateway != "" {
			cfg.Gateway = req.Gateway
		}
		if req.IPRange != "" {
			cfg.IPRange = req.IPRange
		}
		ipamConfig = append(ipamConfig, cfg)
	}

	opts := network.CreateOptions{
		Driver:     req.Driver,
		Attachable: req.Attachable,
		IPAM: &network.IPAM{
			Driver: "default",
			Config: ipamConfig,
		},
		Options: req.Options,
	}

	if req.Driver == "macvlan" && req.Parent != "" {
		if opts.Options == nil {
			opts.Options = make(map[string]string)
		}
		opts.Options["parent"] = req.Parent
	}

	resp, err := c.inner.NetworkCreate(ctx, req.Name, opts)
	if err != nil {
		return "", fmt.Errorf("network create %q: %w", req.Name, err)
	}
	return resp.ID, nil
}

func (c *Client) NetworkRemove(ctx context.Context, networkID string) error {
	return c.inner.NetworkRemove(ctx, networkID)
}

// ── Stack operations ──────────────────────────────────────────────────────────

func (c *Client) StackDeploy(ctx context.Context, name, composeYAML string) error {
	// Docker stack deploy requires the CLI — use compose-based approach via Services API
	// We parse the YAML and create/update services one by one.
	// For swarm mode: services use swarm.ServiceSpec with replicas.
	// Simpler: use `docker stack deploy` via exec through SSH — but we avoid exec().
	// Best approach: use the swarm Services API to create each service from the compose YAML.
	// For now, use the existing Deploy() method which handles compose projects.
	// TODO: replace with native swarm ServiceCreate when needed for multi-node scheduling.
	return c.Deploy(ctx, name, composeYAML)
}

func (c *Client) StackRemove(ctx context.Context, name string) error {
	return c.RemoveContainers(ctx, name)
}

func (c *Client) StackServices(ctx context.Context, stackName string) ([]domain.StackServiceInfo, error) {
	svcs, err := c.inner.ServiceList(ctx, dockertypes.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "com.docker.stack.namespace="+stackName)),
	})
	if err != nil {
		return nil, fmt.Errorf("service list for stack %q: %w", stackName, err)
	}
	out := make([]domain.StackServiceInfo, len(svcs))
	for i, s := range svcs {
		replicas := ""
		if s.Spec.Mode.Replicated != nil && s.Spec.Mode.Replicated.Replicas != nil {
			replicas = fmt.Sprintf("%d/%d", s.ServiceStatus.RunningTasks, *s.Spec.Mode.Replicated.Replicas)
		}
		// Strip the stack prefix from service name for display
		displayName := strings.TrimPrefix(s.Spec.Name, stackName+"_")
		out[i] = domain.StackServiceInfo{
			ID:       s.ID,
			Name:     displayName,
			Replicas: replicas,
		}
	}
	return out, nil
}
