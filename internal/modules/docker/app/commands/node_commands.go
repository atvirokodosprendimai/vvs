package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	"github.com/google/uuid"
)

// ── CreateNode ────────────────────────────────────────────────────────────────

type CreateNodeCommand struct {
	Name    string
	Host    string
	IsLocal bool
	TLSCert []byte
	TLSKey  []byte
	TLSCA   []byte
	Notes   string
}

type CreateNodeHandler struct {
	repo      domain.DockerNodeRepository
	publisher events.EventPublisher
}

func NewCreateNodeHandler(repo domain.DockerNodeRepository, pub events.EventPublisher) *CreateNodeHandler {
	return &CreateNodeHandler{repo: repo, publisher: pub}
}

func (h *CreateNodeHandler) Handle(ctx context.Context, cmd CreateNodeCommand) (*domain.DockerNode, error) {
	node, err := domain.NewDockerNode(cmd.Name, cmd.Host, cmd.IsLocal)
	if err != nil {
		return nil, err
	}
	node.TLSCert = cmd.TLSCert
	node.TLSKey = cmd.TLSKey
	node.TLSCA = cmd.TLSCA
	node.Notes = cmd.Notes

	if err := h.repo.Save(ctx, node); err != nil {
		return nil, err
	}
	h.publish(ctx, events.DockerNodeCreated, "docker.node.created", node.ID, node.Name)
	return node, nil
}

// ── UpdateNode ────────────────────────────────────────────────────────────────

type UpdateNodeCommand struct {
	ID      string
	Name    string
	Host    string
	IsLocal bool
	TLSCert []byte
	TLSKey  []byte
	TLSCA   []byte
	Notes   string
}

type UpdateNodeHandler struct {
	repo      domain.DockerNodeRepository
	publisher events.EventPublisher
}

func NewUpdateNodeHandler(repo domain.DockerNodeRepository, pub events.EventPublisher) *UpdateNodeHandler {
	return &UpdateNodeHandler{repo: repo, publisher: pub}
}

func (h *UpdateNodeHandler) Handle(ctx context.Context, cmd UpdateNodeCommand) (*domain.DockerNode, error) {
	node, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := node.Update(cmd.Name, cmd.Host, cmd.IsLocal, cmd.TLSCert, cmd.TLSKey, cmd.TLSCA, cmd.Notes); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, node); err != nil {
		return nil, err
	}
	h.publish(ctx, events.DockerNodeUpdated, "docker.node.updated", node.ID, node.Name)
	return node, nil
}

// ── DeleteNode ────────────────────────────────────────────────────────────────

type DeleteNodeCommand struct {
	ID string
}

type DeleteNodeHandler struct {
	repo        domain.DockerNodeRepository
	serviceRepo domain.DockerServiceRepository
	publisher   events.EventPublisher
}

func NewDeleteNodeHandler(repo domain.DockerNodeRepository, serviceRepo domain.DockerServiceRepository, pub events.EventPublisher) *DeleteNodeHandler {
	return &DeleteNodeHandler{repo: repo, serviceRepo: serviceRepo, publisher: pub}
}

func (h *DeleteNodeHandler) Handle(ctx context.Context, cmd DeleteNodeCommand) error {
	services, err := h.serviceRepo.FindByNodeID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if len(services) > 0 {
		return domain.ErrNodeHasServices
	}
	if err := h.repo.Delete(ctx, cmd.ID); err != nil {
		return err
	}
	h.publish(ctx, events.DockerNodeDeleted, "docker.node.deleted", cmd.ID, "")
	return nil
}

// ── shared publish helper ─────────────────────────────────────────────────────

func (h *CreateNodeHandler) publish(ctx context.Context, subj events.Subject, t, id, name string) {
	data, _ := json.Marshal(map[string]string{"id": id, "name": name})
	h.publisher.Publish(ctx, subj.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: t,
		AggregateID: id, OccurredAt: time.Now().UTC(), Data: data,
	})
}
func (h *UpdateNodeHandler) publish(ctx context.Context, subj events.Subject, t, id, name string) {
	data, _ := json.Marshal(map[string]string{"id": id, "name": name})
	h.publisher.Publish(ctx, subj.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: t,
		AggregateID: id, OccurredAt: time.Now().UTC(), Data: data,
	})
}
func (h *DeleteNodeHandler) publish(ctx context.Context, subj events.Subject, t, id, name string) {
	data, _ := json.Marshal(map[string]string{"id": id, "name": name})
	h.publisher.Publish(ctx, subj.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: t,
		AggregateID: id, OccurredAt: time.Now().UTC(), Data: data,
	})
}
