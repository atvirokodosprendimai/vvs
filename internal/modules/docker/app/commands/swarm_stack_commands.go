package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/dockerclient"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/google/uuid"
)

// composePath returns the canonical path for a stack's compose file on a node.
func composePath(stackName string) string {
	return fmt.Sprintf("/opt/vvs/stacks/%s/docker-compose.yml", stackName)
}

// composeUp writes the compose YAML to the node and runs `docker compose up -d`.
// If reg is non-nil, docker login is run on the node before compose up.
func composeUp(node *domain.SwarmNode, stackName, composeYAML string, reg *domain.ContainerRegistry, progress func(string)) error {
	dir := fmt.Sprintf("/opt/vvs/stacks/%s", stackName)
	path := composePath(stackName)

	// Write compose file
	escapedYAML := strings.ReplaceAll(composeYAML, "'", `'"'"'`)
	writeCmd := fmt.Sprintf("mkdir -p %s && printf '%%s' '%s' > %s", dir, escapedYAML, path)
	if _, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, writeCmd); err != nil {
		return fmt.Errorf("write compose file: %w", err)
	}
	if progress != nil {
		progress("Compose file written — starting containers…")
	}

	// docker login if registry configured
	if reg != nil {
		if progress != nil {
			progress(fmt.Sprintf("Logging in to registry %s…", reg.URL))
		}
		loginCmd := fmt.Sprintf("echo %s | docker login %s -u %s --password-stdin 2>&1",
			shellQuote(reg.Password), shellQuote(reg.URL), shellQuote(reg.Username))
		if out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, loginCmd); err != nil {
			return fmt.Errorf("docker login: %w\n%s", err, out)
		}
		if progress != nil {
			progress("Registry login OK")
		}
	}

	// Run docker compose up -d
	upCmd := fmt.Sprintf("docker compose -f %s up -d --remove-orphans 2>&1", path)
	out, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, upCmd)
	if err != nil {
		return fmt.Errorf("docker compose up: %w\n%s", err, out)
	}
	if progress != nil && strings.TrimSpace(out) != "" {
		progress(strings.TrimSpace(out))
	}
	return nil
}

// composeDown runs `docker compose down` on the node.
func composeDown(node *domain.SwarmNode, stackName string) error {
	path := composePath(stackName)
	cmd := fmt.Sprintf("docker compose -f %s down 2>&1 || true", path)
	_, err := dockerclient.ExecSSH(node.SshHost, node.SshUser, node.SshPort, node.SshKey, cmd)
	return err
}

// ── DeploySwarmStack ──────────────────────────────────────────────────────────

type DeploySwarmStackCommand struct {
	ClusterID    string
	TargetNodeID string // specific node to run compose on
	Name         string
	ComposeYAML  string
	RegistryID   string // optional; empty = no registry auth
	Routes       []RouteInput
}

type RouteInput struct {
	Hostname    string
	Port        int
	StripPrefix bool
}

type DeploySwarmStackHandler struct {
	clusterRepo  domain.SwarmClusterRepository
	nodeRepo     domain.SwarmNodeRepository
	stackRepo    domain.SwarmStackRepository
	registryRepo domain.ContainerRegistryRepository
	factory      domain.SwarmClientFactory
	publisher    events.EventPublisher
	progress     func(msg string)
}

func NewDeploySwarmStackHandler(
	clusterRepo domain.SwarmClusterRepository,
	nodeRepo domain.SwarmNodeRepository,
	stackRepo domain.SwarmStackRepository,
	registryRepo domain.ContainerRegistryRepository,
	factory domain.SwarmClientFactory,
	pub events.EventPublisher,
) *DeploySwarmStackHandler {
	return &DeploySwarmStackHandler{
		clusterRepo:  clusterRepo,
		nodeRepo:     nodeRepo,
		stackRepo:    stackRepo,
		registryRepo: registryRepo,
		factory:      factory,
		publisher:    pub,
	}
}

func (h *DeploySwarmStackHandler) WithProgress(fn func(msg string)) *DeploySwarmStackHandler {
	cp := *h
	cp.progress = fn
	return &cp
}

func (h *DeploySwarmStackHandler) emit(msg string) {
	if h.progress != nil {
		h.progress(msg)
	}
}

func (h *DeploySwarmStackHandler) Handle(ctx context.Context, cmd DeploySwarmStackCommand) (*domain.SwarmStack, error) {
	stack, err := domain.NewSwarmStack(cmd.ClusterID, cmd.TargetNodeID, cmd.Name, cmd.ComposeYAML)
	if err != nil {
		return nil, err
	}
	stack.RegistryID = cmd.RegistryID
	if err := h.stackRepo.Save(ctx, stack); err != nil {
		return nil, fmt.Errorf("save stack: %w", err)
	}

	// Resolve target node
	node, err := h.resolveTargetNode(ctx, cmd.ClusterID, cmd.TargetNodeID)
	if err != nil {
		stack.MarkError(err.Error())
		_ = h.stackRepo.Save(ctx, stack)
		return stack, fmt.Errorf("resolve target node: %w", err)
	}

	// Resolve registry if configured
	var reg *domain.ContainerRegistry
	if cmd.RegistryID != "" {
		reg, err = h.registryRepo.FindByID(ctx, cmd.RegistryID)
		if err != nil {
			stack.MarkError(fmt.Sprintf("find registry: %v", err))
			_ = h.stackRepo.Save(ctx, stack)
			return stack, nil
		}
	}

	h.emit(fmt.Sprintf("Deploying %q on %s (%s)…", cmd.Name, node.Name, node.SshHost))

	if err := composeUp(node, cmd.Name, cmd.ComposeYAML, reg, h.emit); err != nil {
		stack.MarkError(err.Error())
		_ = h.stackRepo.Save(ctx, stack)
		return stack, nil // error stored on stack, not returned
	}

	stack.MarkRunning()
	if err := h.stackRepo.Save(ctx, stack); err != nil {
		return nil, fmt.Errorf("save stack running: %w", err)
	}

	// Persist routes
	for _, r := range cmd.Routes {
		route, err := domain.NewSwarmRoute(stack.ID, r.Hostname, r.Port, r.StripPrefix)
		if err != nil {
			continue
		}
		_ = h.stackRepo.SaveRoute(ctx, route)
	}

	h.emit("Stack deployed")
	_ = h.publisher.Publish(ctx, events.SwarmStackDeployed.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmStackDeployed.String(),
		AggregateID: stack.ID, OccurredAt: time.Now().UTC(),
	})
	return stack, nil
}

// resolveTargetNode returns the target node by ID, or falls back to manager if ID is empty.
func (h *DeploySwarmStackHandler) resolveTargetNode(ctx context.Context, clusterID, nodeID string) (*domain.SwarmNode, error) {
	if nodeID != "" {
		return h.nodeRepo.FindByID(ctx, nodeID)
	}
	// Fallback: first available manager node
	nodes, err := h.nodeRepo.FindByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.Role == domain.SwarmNodeManager && n.VpnIP != "" {
			return n, nil
		}
	}
	return nil, fmt.Errorf("no target node specified and no active manager found in cluster %s", clusterID)
}

// ── UpdateSwarmStack ──────────────────────────────────────────────────────────

type UpdateSwarmStackCommand struct {
	StackID     string
	ComposeYAML string
	RegistryID  string // optional; empty = keep existing RegistryID from stored stack
}

type UpdateSwarmStackHandler struct {
	nodeRepo     domain.SwarmNodeRepository
	stackRepo    domain.SwarmStackRepository
	registryRepo domain.ContainerRegistryRepository
	factory      domain.SwarmClientFactory
	progress     func(msg string)
}

func NewUpdateSwarmStackHandler(
	nodeRepo domain.SwarmNodeRepository,
	stackRepo domain.SwarmStackRepository,
	registryRepo domain.ContainerRegistryRepository,
	factory domain.SwarmClientFactory,
) *UpdateSwarmStackHandler {
	return &UpdateSwarmStackHandler{nodeRepo: nodeRepo, stackRepo: stackRepo, registryRepo: registryRepo, factory: factory}
}

func (h *UpdateSwarmStackHandler) WithProgress(fn func(msg string)) *UpdateSwarmStackHandler {
	cp := *h
	cp.progress = fn
	return &cp
}

func (h *UpdateSwarmStackHandler) emit(msg string) {
	if h.progress != nil {
		h.progress(msg)
	}
}

func (h *UpdateSwarmStackHandler) Handle(ctx context.Context, cmd UpdateSwarmStackCommand) (*domain.SwarmStack, error) {
	stack, err := h.stackRepo.FindByID(ctx, cmd.StackID)
	if err != nil {
		return nil, err
	}

	stack.UpdateYAML(cmd.ComposeYAML)
	if cmd.RegistryID != "" {
		stack.RegistryID = cmd.RegistryID
	}
	if err := h.stackRepo.Save(ctx, stack); err != nil {
		return nil, fmt.Errorf("save stack: %w", err)
	}

	h.emit(fmt.Sprintf("Updating stack %q…", stack.Name))

	// Find target node
	var targetNode *domain.SwarmNode
	if stack.TargetNodeID != "" {
		targetNode, err = h.nodeRepo.FindByID(ctx, stack.TargetNodeID)
		if err != nil {
			stack.MarkError(fmt.Sprintf("target node not found: %v", err))
			_ = h.stackRepo.Save(ctx, stack)
			return stack, nil
		}
	} else {
		// Legacy stacks without TargetNodeID — fall back to manager node
		nodes, err := h.nodeRepo.FindByClusterID(ctx, stack.ClusterID)
		if err != nil {
			stack.MarkError(err.Error())
			_ = h.stackRepo.Save(ctx, stack)
			return stack, nil
		}
		for _, n := range nodes {
			if n.Role == domain.SwarmNodeManager && n.VpnIP != "" {
				targetNode = n
				break
			}
		}
	}
	if targetNode == nil {
		stack.MarkError("no target node available")
		_ = h.stackRepo.Save(ctx, stack)
		return stack, fmt.Errorf("no target node for stack %s", stack.Name)
	}

	// Resolve registry if configured
	var reg *domain.ContainerRegistry
	if stack.RegistryID != "" {
		reg, err = h.registryRepo.FindByID(ctx, stack.RegistryID)
		if err != nil {
			stack.MarkError(fmt.Sprintf("find registry: %v", err))
			_ = h.stackRepo.Save(ctx, stack)
			return stack, nil
		}
	}

	if err := composeUp(targetNode, stack.Name, cmd.ComposeYAML, reg, h.emit); err != nil {
		stack.MarkError(err.Error())
		_ = h.stackRepo.Save(ctx, stack)
		return stack, nil
	}

	stack.MarkRunning()
	if err := h.stackRepo.Save(ctx, stack); err != nil {
		return nil, fmt.Errorf("save stack running: %w", err)
	}
	h.emit("Stack updated")
	return stack, nil
}

// ── RemoveSwarmStack ──────────────────────────────────────────────────────────

type RemoveSwarmStackCommand struct {
	StackID string
}

type RemoveSwarmStackHandler struct {
	nodeRepo  domain.SwarmNodeRepository
	stackRepo domain.SwarmStackRepository
	factory   domain.SwarmClientFactory
	publisher events.EventPublisher
}

func NewRemoveSwarmStackHandler(
	nodeRepo domain.SwarmNodeRepository,
	stackRepo domain.SwarmStackRepository,
	factory domain.SwarmClientFactory,
	pub events.EventPublisher,
) *RemoveSwarmStackHandler {
	return &RemoveSwarmStackHandler{nodeRepo: nodeRepo, stackRepo: stackRepo, factory: factory, publisher: pub}
}

func (h *RemoveSwarmStackHandler) Handle(ctx context.Context, cmd RemoveSwarmStackCommand) error {
	stack, err := h.stackRepo.FindByID(ctx, cmd.StackID)
	if err != nil {
		return err
	}

	// Find target node and run docker compose down
	var targetNode *domain.SwarmNode
	if stack.TargetNodeID != "" {
		targetNode, _ = h.nodeRepo.FindByID(ctx, stack.TargetNodeID)
	}
	if targetNode == nil {
		// Fall back to any manager node
		nodes, err := h.nodeRepo.FindByClusterID(ctx, stack.ClusterID)
		if err == nil {
			for _, n := range nodes {
				if n.Role == domain.SwarmNodeManager && n.VpnIP != "" {
					targetNode = n
					break
				}
			}
		}
	}
	if targetNode != nil {
		_ = composeDown(targetNode, stack.Name)
	}

	if err := h.stackRepo.Delete(ctx, stack.ID); err != nil {
		return fmt.Errorf("delete stack record: %w", err)
	}

	_ = h.publisher.Publish(ctx, events.SwarmStackRemoved.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: events.SwarmStackRemoved.String(),
		AggregateID: stack.ID, OccurredAt: time.Now().UTC(),
	})
	return nil
}
