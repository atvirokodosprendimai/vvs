package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/hetzner"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/google/uuid"
)

// waitForSSH polls until the SSH daemon on ip:22 is ready to authenticate.
// Returns nil once a successful connection (or auth-level error) is reached,
// meaning the daemon is up and keys are injected. Retries every 10 s up to timeout.
func waitForSSH(ctx context.Context, ip string, privateKey []byte, timeout time.Duration, progress func(string)) error {
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	deadline := time.Now().Add(timeout)
	addr := ip + ":22"
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("SSH not ready on %s after %s", ip, timeout)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, dialErr := ssh.Dial("tcp", addr, cfg)
		if dialErr == nil {
			conn.Close()
			return nil
		}
		s := dialErr.Error()
		notUp := strings.Contains(s, "connection refused") ||
			strings.Contains(s, "no route to host") ||
			strings.Contains(s, "i/o timeout") ||
			strings.Contains(s, "connection reset") ||
			strings.Contains(s, "no supported methods")
		if notUp {
			if progress != nil {
				progress("Waiting for SSH daemon…")
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
			continue
		}
		// Any other error (e.g. successful handshake then auth failure) means daemon is up.
		return nil
	}
}

// SwarmNodeRoleFromString converts a string role to the domain type.
func SwarmNodeRoleFromString(role string) domain.SwarmNodeRole {
	if role == "manager" {
		return domain.SwarmNodeManager
	}
	return domain.SwarmNodeWorker
}

// ── UpdateClusterHetznerConfig ────────────────────────────────────────────────

// UpdateClusterHetznerConfigCommand sets the Hetzner provisioning config on a cluster.
type UpdateClusterHetznerConfigCommand struct {
	ClusterID     string
	APIKey        string
	SSHKeyID      int
	SSHPrivateKey []byte
	SSHPublicKey  string
}

type UpdateClusterHetznerConfigHandler struct {
	clusterRepo domain.SwarmClusterRepository
}

func NewUpdateClusterHetznerConfigHandler(clusterRepo domain.SwarmClusterRepository) *UpdateClusterHetznerConfigHandler {
	return &UpdateClusterHetznerConfigHandler{clusterRepo: clusterRepo}
}

func (h *UpdateClusterHetznerConfigHandler) Handle(ctx context.Context, cmd UpdateClusterHetznerConfigCommand) error {
	cluster, err := h.clusterRepo.FindByID(ctx, cmd.ClusterID)
	if err != nil {
		return err
	}
	cluster.SetHetznerConfig(cmd.APIKey, cmd.SSHKeyID, cmd.SSHPrivateKey, cmd.SSHPublicKey)
	return h.clusterRepo.Save(ctx, cluster)
}

// ── OrderHetznerNode ──────────────────────────────────────────────────────────

// OrderHetznerNodeCommand orders a new Hetzner VPS and fully provisions it into the swarm.
// The cluster must have HetznerAPIKey + HetznerSSHKeyID + SSHPrivateKey configured.
type OrderHetznerNodeCommand struct {
	ClusterID  string
	Name       string             // VPS name (also used as swarm node name)
	ServerType string             // e.g. "cx22", "cx32", "cx42"
	Location   string             // e.g. "nbg1", "fsn1", "hel1"
	Image      string             // e.g. "ubuntu-24.04"
	Role       domain.SwarmNodeRole
}

// OrderHetznerNodeHandler orders + provisions a node end-to-end:
//  1. Create Hetzner VPS
//  2. Poll until running → get IP
//  3. Create SwarmNode record (using cluster SSH key)
//  4. ProvisionSwarmNode (wgmesh)
//  5. AddSwarmNode (join swarm)
type OrderHetznerNodeHandler struct {
	clusterRepo domain.SwarmClusterRepository
	provision   *ProvisionSwarmNodeHandler
	addNode     *AddSwarmNodeHandler
	createNode  *CreateSwarmNodeHandler
	progress    func(string)
}

func NewOrderHetznerNodeHandler(
	clusterRepo domain.SwarmClusterRepository,
	createNode *CreateSwarmNodeHandler,
	provision *ProvisionSwarmNodeHandler,
	addNode *AddSwarmNodeHandler,
) *OrderHetznerNodeHandler {
	return &OrderHetznerNodeHandler{
		clusterRepo: clusterRepo,
		createNode:  createNode,
		provision:   provision,
		addNode:     addNode,
	}
}

func (h *OrderHetznerNodeHandler) WithProgress(fn func(string)) *OrderHetznerNodeHandler {
	cp := *h
	cp.progress = fn
	cp.provision = h.provision.WithProgress(fn)
	cp.addNode = h.addNode.WithProgress(fn)
	return &cp
}

func (h *OrderHetznerNodeHandler) emit(msg string) {
	if h.progress != nil {
		h.progress(msg)
	}
}

func (h *OrderHetznerNodeHandler) Handle(ctx context.Context, cmd OrderHetznerNodeCommand) (*domain.SwarmNode, error) {
	cluster, err := h.clusterRepo.FindByID(ctx, cmd.ClusterID)
	if err != nil {
		return nil, err
	}
	if !cluster.HasHetzner() {
		return nil, fmt.Errorf("cluster %s has no Hetzner configuration", cluster.Name)
	}
	if len(cluster.SSHPrivateKey) == 0 {
		return nil, fmt.Errorf("cluster %s has no SSH private key — cannot connect after provisioning", cluster.Name)
	}

	// 1. Create Hetzner VPS
	h.emit(fmt.Sprintf("Ordering Hetzner VPS: %s (%s in %s)…", cmd.Name, cmd.ServerType, cmd.Location))
	serverID, err := hetzner.CreateServer(ctx, cluster.HetznerAPIKey, hetzner.CreateServerRequest{
		Name:       cmd.Name,
		ServerType: cmd.ServerType,
		Image:      cmd.Image,
		Location:   cmd.Location,
		SSHKeys:    []int{cluster.HetznerSSHKeyID},
	})
	if err != nil {
		return nil, fmt.Errorf("create Hetzner server: %w", err)
	}
	h.emit(fmt.Sprintf("Server #%d created — waiting for it to start…", serverID))

	// 2. Poll until running
	ip, err := hetzner.PollUntilRunning(ctx, cluster.HetznerAPIKey, serverID, 5*time.Minute, h.emit)
	if err != nil {
		return nil, fmt.Errorf("Hetzner server not ready: %w", err)
	}
	h.emit(fmt.Sprintf("Server running at %s — waiting for SSH daemon…", ip))
	if err := waitForSSH(ctx, ip, cluster.SSHPrivateKey, 3*time.Minute, h.emit); err != nil {
		return nil, fmt.Errorf("SSH not ready: %w", err)
	}
	h.emit("SSH ready — starting provisioning…")

	// 3. Create SwarmNode record
	node, err := h.createNode.Handle(ctx, CreateSwarmNodeCommand{
		ClusterID: cmd.ClusterID,
		Name:      cmd.Name,
		SshHost:   ip,
		SshUser:   "root",
		SshPort:   22,
		SshKey:    cluster.SSHPrivateKey,
		Role:      cmd.Role,
	})
	if err != nil {
		return nil, fmt.Errorf("create node record: %w", err)
	}

	// 4. Provision wgmesh
	h.emit("Installing wgmesh on new node…")
	node, err = h.provision.Handle(ctx, ProvisionSwarmNodeCommand{NodeID: node.ID})
	if err != nil {
		return nil, fmt.Errorf("provision wgmesh: %w", err)
	}

	// 5. Join swarm
	h.emit("Joining node to swarm…")
	node, err = h.addNode.Handle(ctx, AddSwarmNodeCommand{
		ClusterID: cmd.ClusterID,
		NodeID:    node.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("join swarm: %w", err)
	}

	h.emit(fmt.Sprintf("Node %s joined swarm — ID: %s", node.Name, uuid.Must(uuid.NewV7()).String()))
	return node, nil
}
