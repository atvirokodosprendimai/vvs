package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/google/uuid"
)

// ── DeployService ─────────────────────────────────────────────────────────────

type DeployServiceCommand struct {
	NodeID      string
	Name        string
	ComposeYAML string
}

type DeployServiceHandler struct {
	nodeRepo    domain.DockerNodeRepository
	serviceRepo domain.DockerServiceRepository
	factory     domain.DockerClientFactory
	publisher   events.EventPublisher
}

func NewDeployServiceHandler(
	nodeRepo domain.DockerNodeRepository,
	serviceRepo domain.DockerServiceRepository,
	factory domain.DockerClientFactory,
	pub events.EventPublisher,
) *DeployServiceHandler {
	return &DeployServiceHandler{
		nodeRepo:    nodeRepo,
		serviceRepo: serviceRepo,
		factory:     factory,
		publisher:   pub,
	}
}

func (h *DeployServiceHandler) Handle(ctx context.Context, cmd DeployServiceCommand) (*domain.DockerService, error) {
	node, err := h.nodeRepo.FindByID(ctx, cmd.NodeID)
	if err != nil {
		return nil, err
	}

	svc, err := domain.NewDockerService(cmd.NodeID, cmd.Name, cmd.ComposeYAML)
	if err != nil {
		return nil, err
	}

	// Save as "deploying" before attempting (allows status tracking)
	if err := h.serviceRepo.Save(ctx, svc); err != nil {
		return nil, err
	}
	h.publishService(ctx, events.DockerServiceDeployed, "docker.service.deployed", svc)

	// Deploy via Docker API
	client, err := h.factory.ForNode(node)
	if err != nil {
		svc.MarkError(fmt.Sprintf("build docker client: %s", err))
		_ = h.serviceRepo.Save(ctx, svc)
		h.publishServiceStatus(ctx, svc)
		return svc, nil
	}

	if deployErr := client.Deploy(ctx, cmd.Name, cmd.ComposeYAML); deployErr != nil {
		svc.MarkError(deployErr.Error())
	} else {
		svc.MarkRunning()
	}

	if err := h.serviceRepo.Save(ctx, svc); err != nil {
		return nil, err
	}
	h.publishServiceStatus(ctx, svc)
	return svc, nil
}

// ── StopService ───────────────────────────────────────────────────────────────

type StopServiceCommand struct{ ID string }

type StopServiceHandler struct {
	nodeRepo    domain.DockerNodeRepository
	serviceRepo domain.DockerServiceRepository
	factory     domain.DockerClientFactory
	publisher   events.EventPublisher
}

func NewStopServiceHandler(
	nodeRepo domain.DockerNodeRepository,
	serviceRepo domain.DockerServiceRepository,
	factory domain.DockerClientFactory,
	pub events.EventPublisher,
) *StopServiceHandler {
	return &StopServiceHandler{nodeRepo: nodeRepo, serviceRepo: serviceRepo, factory: factory, publisher: pub}
}

func (h *StopServiceHandler) Handle(ctx context.Context, cmd StopServiceCommand) error {
	svc, err := h.serviceRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	node, err := h.nodeRepo.FindByID(ctx, svc.NodeID)
	if err != nil {
		return err
	}
	client, err := h.factory.ForNode(node)
	if err != nil {
		return err
	}

	containers, err := client.ListContainers(ctx, svc.Name)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	for _, c := range containers {
		if c.State == "running" {
			if err := client.StopContainer(ctx, c.ID); err != nil {
				return fmt.Errorf("stop %s: %w", c.Name, err)
			}
		}
	}

	svc.MarkStopped()
	if err := h.serviceRepo.Save(ctx, svc); err != nil {
		return err
	}
	h.publishStatus(ctx, svc)
	return nil
}

// ── StartService ──────────────────────────────────────────────────────────────

type StartServiceCommand struct{ ID string }

type StartServiceHandler struct {
	nodeRepo    domain.DockerNodeRepository
	serviceRepo domain.DockerServiceRepository
	factory     domain.DockerClientFactory
	publisher   events.EventPublisher
}

func NewStartServiceHandler(
	nodeRepo domain.DockerNodeRepository,
	serviceRepo domain.DockerServiceRepository,
	factory domain.DockerClientFactory,
	pub events.EventPublisher,
) *StartServiceHandler {
	return &StartServiceHandler{nodeRepo: nodeRepo, serviceRepo: serviceRepo, factory: factory, publisher: pub}
}

func (h *StartServiceHandler) Handle(ctx context.Context, cmd StartServiceCommand) error {
	svc, err := h.serviceRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	node, err := h.nodeRepo.FindByID(ctx, svc.NodeID)
	if err != nil {
		return err
	}
	client, err := h.factory.ForNode(node)
	if err != nil {
		return err
	}

	containers, err := client.ListContainers(ctx, svc.Name)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	for _, c := range containers {
		if c.State != "running" {
			if err := client.StartContainer(ctx, c.ID); err != nil {
				return fmt.Errorf("start %s: %w", c.Name, err)
			}
		}
	}

	svc.MarkRunning()
	if err := h.serviceRepo.Save(ctx, svc); err != nil {
		return err
	}
	h.publishStatus(ctx, svc)
	return nil
}

// ── RemoveService ─────────────────────────────────────────────────────────────

type RemoveServiceCommand struct{ ID string }

type RemoveServiceHandler struct {
	nodeRepo    domain.DockerNodeRepository
	serviceRepo domain.DockerServiceRepository
	factory     domain.DockerClientFactory
	publisher   events.EventPublisher
}

func NewRemoveServiceHandler(
	nodeRepo domain.DockerNodeRepository,
	serviceRepo domain.DockerServiceRepository,
	factory domain.DockerClientFactory,
	pub events.EventPublisher,
) *RemoveServiceHandler {
	return &RemoveServiceHandler{nodeRepo: nodeRepo, serviceRepo: serviceRepo, factory: factory, publisher: pub}
}

func (h *RemoveServiceHandler) Handle(ctx context.Context, cmd RemoveServiceCommand) error {
	svc, err := h.serviceRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	node, err := h.nodeRepo.FindByID(ctx, svc.NodeID)
	if err != nil {
		return err
	}

	svc.MarkRemoving()
	_ = h.serviceRepo.Save(ctx, svc)

	client, err := h.factory.ForNode(node)
	if err == nil {
		_ = client.RemoveContainers(ctx, svc.Name)
	}

	if err := h.serviceRepo.Delete(ctx, svc.ID); err != nil {
		return err
	}
	h.publishRemoved(ctx, svc)
	return nil
}

// ── publish helpers ───────────────────────────────────────────────────────────

func (h *DeployServiceHandler) publishService(ctx context.Context, subj events.Subject, t string, svc *domain.DockerService) {
	data, _ := json.Marshal(map[string]string{"id": svc.ID, "name": svc.Name, "nodeId": svc.NodeID})
	h.publisher.Publish(ctx, subj.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: t,
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(), Data: data,
	})
}
func (h *DeployServiceHandler) publishServiceStatus(ctx context.Context, svc *domain.DockerService) {
	data, _ := json.Marshal(map[string]string{"id": svc.ID, "status": string(svc.Status), "errorMsg": svc.ErrorMsg})
	h.publisher.Publish(ctx, events.DockerServiceStatusChanged.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "docker.service.status_changed",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(), Data: data,
	})
}
func (h *StopServiceHandler) publishStatus(ctx context.Context, svc *domain.DockerService) {
	data, _ := json.Marshal(map[string]string{"id": svc.ID, "status": string(svc.Status)})
	h.publisher.Publish(ctx, events.DockerServiceStatusChanged.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "docker.service.status_changed",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(), Data: data,
	})
}
func (h *StartServiceHandler) publishStatus(ctx context.Context, svc *domain.DockerService) {
	data, _ := json.Marshal(map[string]string{"id": svc.ID, "status": string(svc.Status)})
	h.publisher.Publish(ctx, events.DockerServiceStatusChanged.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "docker.service.status_changed",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(), Data: data,
	})
}
func (h *RemoveServiceHandler) publishRemoved(ctx context.Context, svc *domain.DockerService) {
	data, _ := json.Marshal(map[string]string{"id": svc.ID, "name": svc.Name})
	h.publisher.Publish(ctx, events.DockerServiceRemoved.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "docker.service.removed",
		AggregateID: svc.ID, OccurredAt: time.Now().UTC(), Data: data,
	})
}
