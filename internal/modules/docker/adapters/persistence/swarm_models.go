package persistence

import (
	"encoding/json"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// ── SwarmCluster model ────────────────────────────────────────────────────────

type SwarmClusterModel struct {
	ID                       string `gorm:"primaryKey;type:text"`
	Name                     string `gorm:"type:text;not null"`
	WgmeshKey                []byte `gorm:"column:wgmesh_key"`
	ManagerToken             []byte `gorm:"column:manager_token"`
	WorkerToken              []byte `gorm:"column:worker_token"`
	AdvertiseAddr            string `gorm:"column:advertise_addr;type:text;not null;default:''"`
	Notes                    string `gorm:"type:text;not null;default:''"`
	Status                   string `gorm:"type:text;not null;default:'initializing'"`
	HetznerAPIKey            []byte `gorm:"column:hetzner_api_key"`
	HetznerSSHKeyID          int    `gorm:"column:hetzner_ssh_key_id;not null;default:0"`
	SSHPrivateKey            []byte `gorm:"column:ssh_private_key"`
	SSHPublicKey             string `gorm:"column:ssh_public_key;type:text;not null;default:''"`
	HetznerEnabledLocations  string `gorm:"column:hetzner_enabled_locations;type:text;not null;default:''"`
	HetznerEnabledServerTypes string `gorm:"column:hetzner_enabled_server_types;type:text;not null;default:''"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func (SwarmClusterModel) TableName() string { return "swarm_clusters" }

func jsonEncodeStringSlice(s []string) string {
	if len(s) == 0 {
		return ""
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func jsonDecodeStringSlice(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func toSwarmClusterModel(c *domain.SwarmCluster) *SwarmClusterModel {
	return &SwarmClusterModel{
		ID:                        c.ID,
		Name:                      c.Name,
		AdvertiseAddr:             c.AdvertiseAddr,
		Notes:                     c.Notes,
		Status:                    string(c.Status),
		HetznerSSHKeyID:           c.HetznerSSHKeyID,
		SSHPublicKey:              c.SSHPublicKey,
		HetznerEnabledLocations:   jsonEncodeStringSlice(c.EnabledLocations),
		HetznerEnabledServerTypes: jsonEncodeStringSlice(c.EnabledServerTypes),
		CreatedAt:                 c.CreatedAt,
		UpdatedAt:                 c.UpdatedAt,
	}
}

func toSwarmClusterDomain(m *SwarmClusterModel) *domain.SwarmCluster {
	return &domain.SwarmCluster{
		ID:                 m.ID,
		Name:               m.Name,
		AdvertiseAddr:      m.AdvertiseAddr,
		Notes:              m.Notes,
		Status:             domain.SwarmClusterStatus(m.Status),
		HetznerSSHKeyID:    m.HetznerSSHKeyID,
		SSHPublicKey:       m.SSHPublicKey,
		EnabledLocations:   jsonDecodeStringSlice(m.HetznerEnabledLocations),
		EnabledServerTypes: jsonDecodeStringSlice(m.HetznerEnabledServerTypes),
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}

// ── SwarmNode model ───────────────────────────────────────────────────────────

type SwarmNodeModel struct {
	ID              string `gorm:"primaryKey;type:text"`
	ClusterID       string `gorm:"column:cluster_id;type:text;not null;default:''"`
	Role            string `gorm:"type:text;not null;default:'worker'"`
	Name            string `gorm:"type:text;not null"`
	SshHost         string `gorm:"column:ssh_host;type:text;not null"`
	SshUser         string `gorm:"column:ssh_user;type:text;not null;default:'root'"`
	SshPort         int    `gorm:"column:ssh_port;not null;default:22"`
	SshKey          []byte `gorm:"column:ssh_key"`
	VpnIP           string `gorm:"column:vpn_ip;type:text;not null;default:''"`
	SwarmNodeID     string `gorm:"column:swarm_node_id;type:text;not null;default:''"`
	HetznerServerID int    `gorm:"column:hetzner_server_id;not null;default:0"`
	Status          string `gorm:"type:text;not null;default:'provisioning'"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (SwarmNodeModel) TableName() string { return "swarm_nodes" }

func toSwarmNodeModel(n *domain.SwarmNode) *SwarmNodeModel {
	return &SwarmNodeModel{
		ID:              n.ID,
		ClusterID:       n.ClusterID,
		Role:            string(n.Role),
		Name:            n.Name,
		SshHost:         n.SshHost,
		SshUser:         n.SshUser,
		SshPort:         n.SshPort,
		VpnIP:           n.VpnIP,
		SwarmNodeID:     n.SwarmNodeID,
		HetznerServerID: n.HetznerServerID,
		Status:          string(n.Status),
		CreatedAt:       n.CreatedAt,
		UpdatedAt:       n.UpdatedAt,
	}
}

func toSwarmNodeDomain(m *SwarmNodeModel) *domain.SwarmNode {
	return &domain.SwarmNode{
		ID:              m.ID,
		ClusterID:       m.ClusterID,
		Role:            domain.SwarmNodeRole(m.Role),
		Name:            m.Name,
		SshHost:         m.SshHost,
		SshUser:         m.SshUser,
		SshPort:         m.SshPort,
		VpnIP:           m.VpnIP,
		SwarmNodeID:     m.SwarmNodeID,
		HetznerServerID: m.HetznerServerID,
		Status:          domain.SwarmNodeStatus(m.Status),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// ── SwarmNetwork model ────────────────────────────────────────────────────────

type SwarmNetworkModel struct {
	ID              string `gorm:"primaryKey;type:text"`
	ClusterID       string `gorm:"column:cluster_id;type:text;not null;default:''"`
	Name            string `gorm:"type:text;not null"`
	Driver          string `gorm:"type:text;not null;default:'overlay'"`
	Subnet          string `gorm:"type:text;not null;default:''"`
	Gateway         string `gorm:"type:text;not null;default:''"`
	DhcpRangeStart  string `gorm:"column:dhcp_range_start;type:text;not null;default:''"`
	DhcpRangeEnd    string `gorm:"column:dhcp_range_end;type:text;not null;default:''"`
	Parent          string `gorm:"type:text;not null;default:''"`
	Options         string `gorm:"type:text;not null;default:'{}'"`   // JSON
	ReservedIPs     string `gorm:"column:reserved_ips;type:text;not null;default:'[]'"` // JSON
	Scope           string `gorm:"type:text;not null;default:'swarm'"`
	DockerNetworkID string `gorm:"column:docker_network_id;type:text;not null;default:''"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (SwarmNetworkModel) TableName() string { return "swarm_networks" }

func toSwarmNetworkModel(n *domain.SwarmNetwork) *SwarmNetworkModel {
	opts, _ := json.Marshal(n.Options)
	rips, _ := json.Marshal(n.ReservedIPs)
	return &SwarmNetworkModel{
		ID:              n.ID,
		ClusterID:       n.ClusterID,
		Name:            n.Name,
		Driver:          string(n.Driver),
		Subnet:          n.Subnet,
		Gateway:         n.Gateway,
		DhcpRangeStart:  n.DhcpRangeStart,
		DhcpRangeEnd:    n.DhcpRangeEnd,
		Parent:          n.Parent,
		Options:         string(opts),
		ReservedIPs:     string(rips),
		Scope:           string(n.Scope),
		DockerNetworkID: n.DockerNetworkID,
		CreatedAt:       n.CreatedAt,
		UpdatedAt:       n.UpdatedAt,
	}
}

func toSwarmNetworkDomain(m *SwarmNetworkModel) *domain.SwarmNetwork {
	n := &domain.SwarmNetwork{
		ID:              m.ID,
		ClusterID:       m.ClusterID,
		Name:            m.Name,
		Driver:          domain.SwarmNetworkDriver(m.Driver),
		Subnet:          m.Subnet,
		Gateway:         m.Gateway,
		DhcpRangeStart:  m.DhcpRangeStart,
		DhcpRangeEnd:    m.DhcpRangeEnd,
		Parent:          m.Parent,
		Scope:           domain.SwarmNetworkScope(m.Scope),
		DockerNetworkID: m.DockerNetworkID,
		Options:         make(map[string]string),
		ReservedIPs:     []domain.ReservedIP{},
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	_ = json.Unmarshal([]byte(m.Options), &n.Options)
	_ = json.Unmarshal([]byte(m.ReservedIPs), &n.ReservedIPs)
	return n
}

// ── SwarmStack model ──────────────────────────────────────────────────────────

type SwarmStackModel struct {
	ID           string `gorm:"primaryKey;type:text"`
	ClusterID    string `gorm:"column:cluster_id;type:text;not null"`
	TargetNodeID string `gorm:"column:target_node_id;type:text;not null;default:''"`
	Name         string `gorm:"type:text;not null"`
	ComposeYAML  string `gorm:"column:compose_yaml;type:text;not null;default:''"`
	RegistryID   string `gorm:"column:registry_id;type:text;not null;default:''"`
	Status       string `gorm:"type:text;not null;default:'deploying'"`
	ErrorMsg     string `gorm:"column:error_msg;type:text;not null;default:''"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (SwarmStackModel) TableName() string { return "swarm_stacks" }

func toSwarmStackModel(s *domain.SwarmStack) *SwarmStackModel {
	return &SwarmStackModel{
		ID:           s.ID,
		ClusterID:    s.ClusterID,
		TargetNodeID: s.TargetNodeID,
		Name:         s.Name,
		ComposeYAML:  s.ComposeYAML,
		RegistryID:   s.RegistryID,
		Status:       string(s.Status),
		ErrorMsg:     s.ErrorMsg,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

func toSwarmStackDomain(m *SwarmStackModel) *domain.SwarmStack {
	return &domain.SwarmStack{
		ID:           m.ID,
		ClusterID:    m.ClusterID,
		TargetNodeID: m.TargetNodeID,
		Name:         m.Name,
		ComposeYAML:  m.ComposeYAML,
		RegistryID:   m.RegistryID,
		Status:       domain.SwarmStackStatus(m.Status),
		ErrorMsg:     m.ErrorMsg,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// ── SwarmRoute model ──────────────────────────────────────────────────────────

type SwarmRouteModel struct {
	ID          string    `gorm:"primaryKey;type:text"`
	StackID     string    `gorm:"column:stack_id;type:text;not null"`
	Hostname    string    `gorm:"type:text;not null"`
	Port        int       `gorm:"not null;default:80"`
	StripPrefix bool      `gorm:"column:strip_prefix;not null;default:false"`
	CreatedAt   time.Time
}

func (SwarmRouteModel) TableName() string { return "swarm_routes" }

func toSwarmRouteModel(r *domain.SwarmRoute) *SwarmRouteModel {
	return &SwarmRouteModel{
		ID:          r.ID,
		StackID:     r.StackID,
		Hostname:    r.Hostname,
		Port:        r.Port,
		StripPrefix: r.StripPrefix,
		CreatedAt:   r.CreatedAt,
	}
}

func toSwarmRouteDomain(m *SwarmRouteModel) *domain.SwarmRoute {
	return &domain.SwarmRoute{
		ID:          m.ID,
		StackID:     m.StackID,
		Hostname:    m.Hostname,
		Port:        m.Port,
		StripPrefix: m.StripPrefix,
		CreatedAt:   m.CreatedAt,
	}
}
