package domain

import "context"

// NetworkCreateRequest is the input for creating a Docker network.
type NetworkCreateRequest struct {
	Name       string
	Driver     string
	Subnet     string
	Gateway    string
	IPRange    string // DHCP pool CIDR to constrain Docker's allocation to lower half
	Parent     string // macvlan only
	Attachable bool   // overlay only
	Options    map[string]string
}

// SwarmNodeInfo is a read model for a Docker Swarm node from the daemon.
type SwarmNodeInfo struct {
	ID       string
	Hostname string
	Role     string
	Status   string
}

// StackServiceInfo is a service within a deployed stack.
type StackServiceInfo struct {
	ID       string
	Name     string
	Replicas string // e.g. "2/2"
}

// SwarmDockerClient extends DockerClient with swarm management operations.
// All addr parameters use the node's wgmesh0 VPN IP.
type SwarmDockerClient interface {
	DockerClient

	// SwarmInit initialises a new swarm using vpnIP for all three addr flags:
	// --advertise-addr, --data-path-addr, --listen-addr
	SwarmInit(ctx context.Context, vpnIP string) (managerToken, workerToken string, err error)

	// SwarmJoin joins the node to an existing swarm.
	SwarmJoin(ctx context.Context, managerVpnIP, joinToken string) error

	// SwarmLeave removes the node from the swarm.
	SwarmLeave(ctx context.Context, force bool) error

	// SwarmNodeList returns all nodes visible from this manager.
	SwarmNodeList(ctx context.Context) ([]SwarmNodeInfo, error)

	// SwarmNodeRemove removes a node from the swarm by Docker node ID.
	SwarmNodeRemove(ctx context.Context, dockerNodeID string) error

	// NetworkCreate creates a Docker network and returns its ID.
	NetworkCreate(ctx context.Context, req NetworkCreateRequest) (string, error)

	// NetworkRemove removes a Docker network by ID.
	NetworkRemove(ctx context.Context, networkID string) error

	// StackDeploy deploys or updates a stack from a compose YAML.
	StackDeploy(ctx context.Context, name, composeYAML string) error

	// StackRemove removes a stack and all its services.
	StackRemove(ctx context.Context, name string) error

	// StackServices returns service replica info for a stack.
	StackServices(ctx context.Context, stackName string) ([]StackServiceInfo, error)
}

// SwarmClientFactory builds a SwarmDockerClient for a given SwarmNode via SSH.
type SwarmClientFactory interface {
	ForSwarmNode(node *SwarmNode) (SwarmDockerClient, error)
}
