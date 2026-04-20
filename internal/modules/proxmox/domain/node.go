package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNodeNameRequired  = errors.New("node name is required")
	ErrHostRequired      = errors.New("host is required")
	ErrUserRequired      = errors.New("user is required")
	ErrTokenIDRequired   = errors.New("token ID is required")
	ErrNodeNotFound      = errors.New("node not found")
	ErrNodeHasVMs        = errors.New("node has VMs — remove or reassign them first")
)

// ProxmoxNode stores connection details for a Proxmox VE node.
// TokenSecret is stored encrypted at rest (AES-256-GCM, see persistence layer).
type ProxmoxNode struct {
	ID          string
	Name        string // human label, e.g. "pve-01"
	NodeName    string // Proxmox cluster node name, e.g. "pve"
	Host        string // IP or hostname
	Port        int    // default 8006
	User        string // e.g. "root@pam"
	TokenID     string // e.g. "vvs"
	TokenSecret string // encrypted at rest
	InsecureTLS bool   // skip TLS verification (self-signed certs)
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NodeConn holds connection parameters passed to VMProvisioner per-call.
type NodeConn struct {
	NodeName    string
	Host        string
	Port        int
	User        string
	TokenID     string
	TokenSecret string
	InsecureTLS bool
}

func NewProxmoxNode(name, nodeName, host string, port int, user, tokenID, tokenSecret, notes string, insecureTLS bool) (*ProxmoxNode, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNodeNameRequired
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, ErrHostRequired
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return nil, ErrUserRequired
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, ErrTokenIDRequired
	}
	if port <= 0 {
		port = 8006
	}
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		nodeName = host // fallback: use host as node name
	}

	now := time.Now().UTC()
	return &ProxmoxNode{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Name:        name,
		NodeName:    nodeName,
		Host:        host,
		Port:        port,
		User:        user,
		TokenID:     tokenID,
		TokenSecret: tokenSecret,
		InsecureTLS: insecureTLS,
		Notes:       strings.TrimSpace(notes),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (n *ProxmoxNode) Update(name, nodeName, host string, port int, user, tokenID, tokenSecret, notes string, insecureTLS bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNodeNameRequired
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ErrHostRequired
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return ErrUserRequired
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return ErrTokenIDRequired
	}
	if port <= 0 {
		port = 8006
	}
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		nodeName = host
	}

	n.Name = name
	n.NodeName = nodeName
	n.Host = host
	n.Port = port
	n.User = user
	n.TokenID = tokenID
	if tokenSecret != "" {
		n.TokenSecret = tokenSecret
	}
	n.InsecureTLS = insecureTLS
	n.Notes = strings.TrimSpace(notes)
	n.UpdatedAt = time.Now().UTC()
	return nil
}

// ToConn projects the node into a NodeConn for passing to VMProvisioner.
func (n *ProxmoxNode) ToConn() NodeConn {
	return NodeConn{
		NodeName:    n.NodeName,
		Host:        n.Host,
		Port:        n.Port,
		User:        n.User,
		TokenID:     n.TokenID,
		TokenSecret: n.TokenSecret,
		InsecureTLS: n.InsecureTLS,
	}
}
