package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/google/uuid"
)

// ── DeploySwarmStack ──────────────────────────────────────────────────────────

type DeploySwarmStackCommand struct {
	ClusterID   string
	Name        string
	ComposeYAML string
	Routes      []RouteInput
}

type RouteInput struct {
	Hostname    string
	Port        int
	StripPrefix bool
}

type DeploySwarmStackHandler struct {
	clusterRepo domain.SwarmClusterRepository
	nodeRepo    domain.SwarmNodeRepository
	stackRepo   domain.SwarmStackRepository
	factory     domain.SwarmClientFactory
	publisher   events.EventPublisher
	progress    func(msg string)
}

func NewDeploySwarmStackHandler(
	clusterRepo domain.SwarmClusterRepository,
	nodeRepo domain.SwarmNodeRepository,
	stackRepo domain.SwarmStackRepository,
	factory domain.SwarmClientFactory,
	pub events.EventPublisher,
) *DeploySwarmStackHandler {
	return &DeploySwarmStackHandler{
		clusterRepo: clusterRepo,
		nodeRepo:    nodeRepo,
		stackRepo:   stackRepo,
		factory:     factory,
		publisher:   pub,
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
	stack, err := domain.NewSwarmStack(cmd.ClusterID, cmd.Name, cmd.ComposeYAML)
	if err != nil {
		return nil, err
	}
	if err := h.stackRepo.Save(ctx, stack); err != nil {
		return nil, fmt.Errorf("save stack: %w", err)
	}

	h.emit(fmt.Sprintf("Deploying stack %q…", cmd.Name))

	managerNode, err := h.findManagerNode(ctx, cmd.ClusterID)
	if err != nil {
		stack.MarkError(err.Error())
		_ = h.stackRepo.Save(ctx, stack)
		return stack, fmt.Errorf("find manager node: %w", err)
	}

	client, err := h.factory.ForSwarmNode(managerNode)
	if err != nil {
		stack.MarkError(err.Error())
		_ = h.stackRepo.Save(ctx, stack)
		return stack, fmt.Errorf("create docker client: %w", err)
	}

	if err := client.StackDeploy(ctx, cmd.Name, cmd.ComposeYAML); err != nil {
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

func (h *DeploySwarmStackHandler) findManagerNode(ctx context.Context, clusterID string) (*domain.SwarmNode, error) {
	nodes, err := h.nodeRepo.FindByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.Role == domain.SwarmNodeManager && n.VpnIP != "" {
			return n, nil
		}
	}
	return nil, fmt.Errorf("no active manager node in cluster %s", clusterID)
}

// ── UpdateSwarmStack ──────────────────────────────────────────────────────────

type UpdateSwarmStackCommand struct {
	StackID     string
	ComposeYAML string
}

type UpdateSwarmStackHandler struct {
	nodeRepo  domain.SwarmNodeRepository
	stackRepo domain.SwarmStackRepository
	factory   domain.SwarmClientFactory
	progress  func(msg string)
}

func NewUpdateSwarmStackHandler(
	nodeRepo domain.SwarmNodeRepository,
	stackRepo domain.SwarmStackRepository,
	factory domain.SwarmClientFactory,
) *UpdateSwarmStackHandler {
	return &UpdateSwarmStackHandler{nodeRepo: nodeRepo, stackRepo: stackRepo, factory: factory}
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
	if err := h.stackRepo.Save(ctx, stack); err != nil {
		return nil, fmt.Errorf("save stack: %w", err)
	}

	h.emit(fmt.Sprintf("Updating stack %q…", stack.Name))

	nodes, err := h.nodeRepo.FindByClusterID(ctx, stack.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("find cluster nodes: %w", err)
	}
	var managerNode *domain.SwarmNode
	for _, n := range nodes {
		if n.Role == domain.SwarmNodeManager && n.VpnIP != "" {
			managerNode = n
			break
		}
	}
	if managerNode == nil {
		stack.MarkError("no active manager node")
		_ = h.stackRepo.Save(ctx, stack)
		return stack, fmt.Errorf("no active manager node in cluster %s", stack.ClusterID)
	}

	client, err := h.factory.ForSwarmNode(managerNode)
	if err != nil {
		stack.MarkError(err.Error())
		_ = h.stackRepo.Save(ctx, stack)
		return stack, fmt.Errorf("create docker client: %w", err)
	}

	if err := client.StackDeploy(ctx, stack.Name, cmd.ComposeYAML); err != nil {
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

	nodes, err := h.nodeRepo.FindByClusterID(ctx, stack.ClusterID)
	if err == nil {
		for _, n := range nodes {
			if n.Role == domain.SwarmNodeManager && n.VpnIP != "" {
				client, err := h.factory.ForSwarmNode(n)
				if err == nil {
					_ = client.StackRemove(ctx, stack.Name)
				}
				break
			}
		}
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
