package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/dockerclient"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
	"github.com/google/uuid"
)

// ── CreateIPTVStack ───────────────────────────────────────────────────────────

type CreateIPTVStackCommand struct {
	Name               string
	ClusterID          string
	NodeID             string
	WANNetworkID       string
	OverlayNetworkID   string
	WANNetworkName     string
	OverlayNetworkName string
	WanIP              string
}

type CreateIPTVStackHandler struct {
	repo domain.IPTVStackRepository
}

func NewCreateIPTVStackHandler(repo domain.IPTVStackRepository) *CreateIPTVStackHandler {
	return &CreateIPTVStackHandler{repo: repo}
}

func (h *CreateIPTVStackHandler) Handle(ctx context.Context, cmd CreateIPTVStackCommand) (*domain.IPTVStack, error) {
	now := time.Now().UTC()
	s := &domain.IPTVStack{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		Name:               cmd.Name,
		ClusterID:          cmd.ClusterID,
		NodeID:             cmd.NodeID,
		WANNetworkID:       cmd.WANNetworkID,
		OverlayNetworkID:   cmd.OverlayNetworkID,
		WANNetworkName:     cmd.WANNetworkName,
		OverlayNetworkName: cmd.OverlayNetworkName,
		WanIP:              cmd.WanIP,
		Status:             domain.IPTVStackPending,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := h.repo.Save(ctx, s); err != nil {
		return nil, fmt.Errorf("save iptv stack: %w", err)
	}
	return s, nil
}

// ── UpdateIPTVStack ───────────────────────────────────────────────────────────

type UpdateIPTVStackCommand struct {
	ID                 string
	Name               string
	WANNetworkID       string
	OverlayNetworkID   string
	WANNetworkName     string
	OverlayNetworkName string
	WanIP              string
}

type UpdateIPTVStackHandler struct {
	repo domain.IPTVStackRepository
}

func NewUpdateIPTVStackHandler(repo domain.IPTVStackRepository) *UpdateIPTVStackHandler {
	return &UpdateIPTVStackHandler{repo: repo}
}

func (h *UpdateIPTVStackHandler) Handle(ctx context.Context, cmd UpdateIPTVStackCommand) error {
	s, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	s.Name = cmd.Name
	s.WANNetworkID = cmd.WANNetworkID
	s.OverlayNetworkID = cmd.OverlayNetworkID
	s.WANNetworkName = cmd.WANNetworkName
	s.OverlayNetworkName = cmd.OverlayNetworkName
	s.WanIP = cmd.WanIP
	s.Status = domain.IPTVStackPending
	s.UpdatedAt = time.Now().UTC()
	return h.repo.Save(ctx, s)
}

// ── DeleteIPTVStack ───────────────────────────────────────────────────────────

type DeleteIPTVStackCommand struct{ ID string }

type DeleteIPTVStackHandler struct {
	repo        domain.IPTVStackRepository
	channelRepo domain.IPTVStackChannelRepository
}

func NewDeleteIPTVStackHandler(repo domain.IPTVStackRepository, channelRepo domain.IPTVStackChannelRepository) *DeleteIPTVStackHandler {
	return &DeleteIPTVStackHandler{repo: repo, channelRepo: channelRepo}
}

func (h *DeleteIPTVStackHandler) Handle(ctx context.Context, cmd DeleteIPTVStackCommand) error {
	_ = h.channelRepo.DeleteByStackID(ctx, cmd.ID) // best-effort cascade
	return h.repo.Delete(ctx, cmd.ID)
}

// ── AddChannelToIPTVStack ─────────────────────────────────────────────────────

type AddChannelToIPTVStackCommand struct {
	StackID    string
	ChannelID  string
	ProviderID string
}

type AddChannelToIPTVStackHandler struct {
	stackRepo   domain.IPTVStackRepository
	channelRepo domain.IPTVStackChannelRepository
}

func NewAddChannelToIPTVStackHandler(stackRepo domain.IPTVStackRepository, channelRepo domain.IPTVStackChannelRepository) *AddChannelToIPTVStackHandler {
	return &AddChannelToIPTVStackHandler{stackRepo: stackRepo, channelRepo: channelRepo}
}

func (h *AddChannelToIPTVStackHandler) Handle(ctx context.Context, cmd AddChannelToIPTVStackCommand) error {
	sc := &domain.IPTVStackChannel{
		ID:         uuid.Must(uuid.NewV7()).String(),
		StackID:    cmd.StackID,
		ChannelID:  cmd.ChannelID,
		ProviderID: cmd.ProviderID,
	}
	if err := h.channelRepo.Save(ctx, sc); err != nil {
		return fmt.Errorf("save stack channel: %w", err)
	}
	// Mark stack pending so user knows re-deploy is needed
	stack, err := h.stackRepo.FindByID(ctx, cmd.StackID)
	if err != nil {
		return nil // not fatal
	}
	stack.Status = domain.IPTVStackPending
	stack.UpdatedAt = time.Now().UTC()
	_ = h.stackRepo.Save(ctx, stack)
	return nil
}

// ── RemoveChannelFromIPTVStack ────────────────────────────────────────────────

type RemoveChannelFromIPTVStackCommand struct {
	StackID   string
	ChannelID string
}

type RemoveChannelFromIPTVStackHandler struct {
	stackRepo   domain.IPTVStackRepository
	channelRepo domain.IPTVStackChannelRepository
}

func NewRemoveChannelFromIPTVStackHandler(stackRepo domain.IPTVStackRepository, channelRepo domain.IPTVStackChannelRepository) *RemoveChannelFromIPTVStackHandler {
	return &RemoveChannelFromIPTVStackHandler{stackRepo: stackRepo, channelRepo: channelRepo}
}

func (h *RemoveChannelFromIPTVStackHandler) Handle(ctx context.Context, cmd RemoveChannelFromIPTVStackCommand) error {
	sc, err := h.channelRepo.FindByStackIDAndChannelID(ctx, cmd.StackID, cmd.ChannelID)
	if err != nil {
		return err
	}
	if err := h.channelRepo.Delete(ctx, sc.ID); err != nil {
		return err
	}
	stack, err := h.stackRepo.FindByID(ctx, cmd.StackID)
	if err != nil {
		return nil
	}
	stack.Status = domain.IPTVStackPending
	stack.UpdatedAt = time.Now().UTC()
	_ = h.stackRepo.Save(ctx, stack)
	return nil
}

// ── DeployIPTVStack ───────────────────────────────────────────────────────────

// DeployIPTVStackCommand provides the stack ID plus the resolved SSH credentials
// for the target node (resolved at the wire/handler layer from the SwarmNode).
type DeployIPTVStackCommand struct {
	StackID     string
	NodeHost    string
	NodeUser    string
	NodePort    int
	NodeSSHKey  []byte
}

type DeployIPTVStackHandler struct {
	stackRepo    domain.IPTVStackRepository
	channelRepo  domain.IPTVStackChannelRepository
	providerRepo domain.ChannelProviderRepository
	channelLookup channelSlugLookup
	progress     func(string)
}

type channelSlugLookup interface {
	FindByID(ctx context.Context, id string) (*domain.Channel, error)
}

func NewDeployIPTVStackHandler(
	stackRepo domain.IPTVStackRepository,
	channelRepo domain.IPTVStackChannelRepository,
	providerRepo domain.ChannelProviderRepository,
	channelLookup channelSlugLookup,
) *DeployIPTVStackHandler {
	return &DeployIPTVStackHandler{
		stackRepo:    stackRepo,
		channelRepo:  channelRepo,
		providerRepo: providerRepo,
		channelLookup: channelLookup,
	}
}

func (h *DeployIPTVStackHandler) WithProgress(fn func(string)) *DeployIPTVStackHandler {
	cp := *h
	cp.progress = fn
	return &cp
}

func (h *DeployIPTVStackHandler) emit(msg string) {
	if h.progress != nil {
		h.progress(msg)
	}
}

func (h *DeployIPTVStackHandler) Handle(ctx context.Context, cmd DeployIPTVStackCommand) error {
	stack, err := h.stackRepo.FindByID(ctx, cmd.StackID)
	if err != nil {
		return fmt.Errorf("load stack: %w", err)
	}

	assignments, err := h.channelRepo.FindByStackID(ctx, cmd.StackID)
	if err != nil {
		return fmt.Errorf("load stack channels: %w", err)
	}

	h.emit(fmt.Sprintf("Building compose for %d channels…", len(assignments)))

	details := make([]domain.IPTVStackChannelDetail, 0, len(assignments))
	for _, sc := range assignments {
		ch, err := h.channelLookup.FindByID(ctx, sc.ChannelID)
		if err != nil {
			continue
		}
		slug := ch.Slug
		if slug == "" {
			slug = domain.Slugify(ch.Name)
		}
		if sc.ProviderID == "" {
			// No provider — skip from compose
			continue
		}
		prov, err := h.providerRepo.FindByID(ctx, sc.ProviderID)
		if err != nil {
			continue
		}
		resolvedURL := domain.ResolveProviderURL(prov.URLTemplate, slug, prov.Token)
		details = append(details, domain.IPTVStackChannelDetail{
			ChannelSlug:  slug,
			ProviderType: prov.Type,
			ResolvedURL:  resolvedURL,
		})
	}

	composeYAML := domain.GenerateIPTVCompose(stack, details)
	deployPath := fmt.Sprintf("/opt/vvs/stacks/%s", stack.Name)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", deployPath)
	writeCmd := fmt.Sprintf("cat > %s/docker-compose.yml << 'EOFCOMPOSE'\n%s\nEOFCOMPOSE", deployPath, composeYAML)
	upCmd := fmt.Sprintf("docker compose -f %s/docker-compose.yml up -d --remove-orphans", deployPath)

	stack.Status = domain.IPTVStackDeploying
	stack.UpdatedAt = time.Now().UTC()
	_ = h.stackRepo.Save(ctx, stack)

	h.emit("Creating stack directory…")
	if _, err := dockerclient.ExecSSH(cmd.NodeHost, cmd.NodeUser, cmd.NodePort, cmd.NodeSSHKey, mkdirCmd); err != nil {
		return h.fail(ctx, stack, fmt.Errorf("mkdir: %w", err))
	}

	h.emit("Writing docker-compose.yml…")
	if _, err := dockerclient.ExecSSH(cmd.NodeHost, cmd.NodeUser, cmd.NodePort, cmd.NodeSSHKey, writeCmd); err != nil {
		return h.fail(ctx, stack, fmt.Errorf("write compose: %w", err))
	}

	h.emit("Running docker compose up -d --remove-orphans…")
	if _, err := dockerclient.ExecSSH(cmd.NodeHost, cmd.NodeUser, cmd.NodePort, cmd.NodeSSHKey, upCmd); err != nil {
		return h.fail(ctx, stack, fmt.Errorf("compose up: %w", err))
	}

	now := time.Now().UTC()
	stack.Status = domain.IPTVStackRunning
	stack.LastDeployedAt = &now
	stack.UpdatedAt = now
	_ = h.stackRepo.Save(ctx, stack)
	h.emit("Stack deployed successfully")
	return nil
}

func (h *DeployIPTVStackHandler) fail(ctx context.Context, stack *domain.IPTVStack, err error) error {
	stack.Status = domain.IPTVStackError
	stack.UpdatedAt = time.Now().UTC()
	_ = h.stackRepo.Save(ctx, stack)
	return err
}
