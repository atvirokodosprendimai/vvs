package app

import (
	"context"
	"log"

	infrahttp "github.com/atvirokodosprendimai/vvs/internal/infrastructure/http"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"

	dockerpersistence   "github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/persistence"
	customerpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/customer/adapters/persistence"
	customerdomain      "github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
	iptvhttp            "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/adapters/http"
	iptvpersistence     "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/adapters/persistence"
	iptvcommands        "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/app/commands"
	iptvqueries         "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/app/queries"
	shareddomain        "github.com/atvirokodosprendimai/vvs/internal/shared/domain"
)

type iptvWired struct {
	channelRepo  *iptvpersistence.ChannelRepository
	packageRepo  *iptvpersistence.PackageRepository
	subRepo      *iptvpersistence.SubscriptionRepository
	stbRepo      *iptvpersistence.STBRepository
	keyRepo      *iptvpersistence.SubscriptionKeyRepository
	epgRepo      *iptvpersistence.EPGProgrammeRepository
	providerRepo *iptvpersistence.ChannelProviderRepository
	routes       infrahttp.ModuleRoutes
}

// swarmNodeSSHAdapter wraps the docker swarm node repository and implements NodeSSHLookup.
type swarmNodeSSHAdapter struct {
	repo *dockerpersistence.GormSwarmNodeRepository
}

func (a *swarmNodeSSHAdapter) FindByID(ctx context.Context, id string) (*iptvhttp.NodeSSHInfo, error) {
	n, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user := n.SshUser
	if user == "" {
		user = "root"
	}
	port := n.SshPort
	if port == 0 {
		port = 22
	}
	return &iptvhttp.NodeSSHInfo{
		Host:   n.SshHost,
		User:   user,
		Port:   port,
		SSHKey: n.SshKey,
	}, nil
}

// swarmClustersAdapter implements iptvhttp.SwarmClustersLookup.
type swarmClustersAdapter struct {
	repo *dockerpersistence.GormSwarmClusterRepository
}

func (a *swarmClustersAdapter) FindAll(ctx context.Context) ([]iptvhttp.ClusterOption, error) {
	clusters, err := a.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]iptvhttp.ClusterOption, len(clusters))
	for i, c := range clusters {
		out[i] = iptvhttp.ClusterOption{ID: c.ID, Name: c.Name}
	}
	return out, nil
}

// swarmNodesLookupAdapter implements iptvhttp.SwarmNodesLookup.
type swarmNodesLookupAdapter struct {
	repo *dockerpersistence.GormSwarmNodeRepository
}

func (a *swarmNodesLookupAdapter) FindByClusterID(ctx context.Context, clusterID string) ([]iptvhttp.NodeOption, error) {
	nodes, err := a.repo.FindByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	out := make([]iptvhttp.NodeOption, len(nodes))
	for i, n := range nodes {
		out[i] = iptvhttp.NodeOption{ID: n.ID, Name: n.Name, VpnIP: n.VpnIP}
	}
	return out, nil
}

// swarmNetworksAdapter implements iptvhttp.SwarmNetworksLookup.
type swarmNetworksAdapter struct {
	repo *dockerpersistence.GormSwarmNetworkRepository
}

func (a *swarmNetworksAdapter) FindByClusterID(ctx context.Context, clusterID string) ([]iptvhttp.NetworkOption, error) {
	nets, err := a.repo.FindByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	out := make([]iptvhttp.NetworkOption, len(nets))
	for i, n := range nets {
		out[i] = iptvhttp.NetworkOption{ID: n.ID, Name: n.Name}
	}
	return out, nil
}

// customerLookupAdapter implements iptvhttp.CustomerLookup.
type customerLookupAdapter struct {
	repo *customerpersistence.GormCustomerRepository
}

func (a *customerLookupAdapter) FindAll(ctx context.Context, search string) ([]iptvhttp.CustomerOption, error) {
	customers, _, err := a.repo.FindAll(ctx, customerdomain.CustomerFilter{Search: search}, shareddomain.NewPagination(1, 1000))
	if err != nil {
		return nil, err
	}
	out := make([]iptvhttp.CustomerOption, len(customers))
	for i, c := range customers {
		out[i] = iptvhttp.CustomerOption{ID: c.ID, Name: c.CompanyName}
	}
	return out, nil
}

func wireIPTV(gdb *gormsqlite.DB, docker *dockerWired, cust *customerWired) *iptvWired {
	channelRepo  := iptvpersistence.NewChannelRepository(gdb)
	packageRepo  := iptvpersistence.NewPackageRepository(gdb)
	subRepo      := iptvpersistence.NewSubscriptionRepository(gdb)
	stbRepo      := iptvpersistence.NewSTBRepository(gdb)
	keyRepo      := iptvpersistence.NewSubscriptionKeyRepository(gdb)
	epgRepo      := iptvpersistence.NewEPGProgrammeRepository(gdb)
	providerRepo := iptvpersistence.NewChannelProviderRepository(gdb)
	stackRepo    := iptvpersistence.NewIPTVStackRepository(gdb)
	stackChRepo  := iptvpersistence.NewIPTVStackChannelRepository(gdb)

	var nodeLookup iptvhttp.NodeSSHLookup
	var clusterLookup iptvhttp.SwarmClustersLookup
	var nodeSLookup iptvhttp.SwarmNodesLookup
	var networkLookup iptvhttp.SwarmNetworksLookup
	var custLookup iptvhttp.CustomerLookup
	if docker != nil {
		if docker.swarmNodeRepo != nil {
			nodeLookup = &swarmNodeSSHAdapter{repo: docker.swarmNodeRepo}
			nodeSLookup = &swarmNodesLookupAdapter{repo: docker.swarmNodeRepo}
		}
		if docker.swarmClusterRepo != nil {
			clusterLookup = &swarmClustersAdapter{repo: docker.swarmClusterRepo}
		}
		if docker.swarmNetworkRepo != nil {
			networkLookup = &swarmNetworksAdapter{repo: docker.swarmNetworkRepo}
		}
	}
	if cust != nil && cust.repo != nil {
		custLookup = &customerLookupAdapter{repo: cust.repo}
	}

	routes := iptvhttp.NewIPTVHandlers(
		iptvcommands.NewCreateChannelHandler(channelRepo),
		iptvcommands.NewUpdateChannelHandler(channelRepo),
		iptvcommands.NewDeleteChannelHandler(channelRepo),
		iptvcommands.NewCreatePackageHandler(packageRepo),
		iptvcommands.NewUpdatePackageHandler(packageRepo),
		iptvcommands.NewDeletePackageHandler(packageRepo),
		iptvcommands.NewAddChannelToPackageHandler(packageRepo),
		iptvcommands.NewRemoveChannelFromPackageHandler(packageRepo),
		iptvcommands.NewCreateSubscriptionHandler(subRepo, keyRepo, packageRepo),
		iptvcommands.NewSuspendSubscriptionHandler(subRepo),
		iptvcommands.NewReactivateSubscriptionHandler(subRepo),
		iptvcommands.NewCancelSubscriptionHandler(subRepo),
		iptvcommands.NewRevokeSubscriptionKeyHandler(keyRepo),
		iptvcommands.NewReissueSubscriptionKeyHandler(keyRepo),
		iptvcommands.NewAssignSTBHandler(stbRepo),
		iptvcommands.NewDeleteSTBHandler(stbRepo),
		// Provider commands
		iptvcommands.NewCreateChannelProviderHandler(providerRepo),
		iptvcommands.NewDeleteChannelProviderHandler(providerRepo),
		// Stack commands
		iptvcommands.NewCreateIPTVStackHandler(stackRepo),
		iptvcommands.NewDeleteIPTVStackHandler(stackRepo, stackChRepo),
		iptvcommands.NewAddChannelToIPTVStackHandler(stackRepo, stackChRepo),
		iptvcommands.NewRemoveChannelFromIPTVStackHandler(stackRepo, stackChRepo),
		iptvcommands.NewDeployIPTVStackHandler(stackRepo, stackChRepo, providerRepo, channelRepo),
		// Queries
		iptvqueries.NewListChannelsHandler(channelRepo),
		iptvqueries.NewGetChannelHandler(channelRepo),
		iptvqueries.NewListPackagesHandler(packageRepo),
		iptvqueries.NewListSubscriptionsHandler(subRepo, packageRepo),
		iptvqueries.NewListSTBsHandler(stbRepo),
		iptvqueries.NewListChannelProvidersHandler(providerRepo),
		iptvqueries.NewListIPTVStacksHandler(stackRepo, stackChRepo),
		iptvqueries.NewGetIPTVStackChannelsHandler(stackChRepo, channelRepo, providerRepo),
		iptvcommands.NewImportEPGHandler(epgRepo),
		nodeLookup,
		stackRepo,
		clusterLookup,
		nodeSLookup,
		networkLookup,
		custLookup,
	)

	log.Printf("module wired: iptv")

	return &iptvWired{
		channelRepo:  channelRepo,
		packageRepo:  packageRepo,
		subRepo:      subRepo,
		stbRepo:      stbRepo,
		keyRepo:      keyRepo,
		epgRepo:      epgRepo,
		providerRepo: providerRepo,
		routes:       routes,
	}
}
