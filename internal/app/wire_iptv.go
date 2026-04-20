package app

import (
	"context"
	"log"

	infrahttp "github.com/atvirokodosprendimai/vvs/internal/infrastructure/http"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"

	dockerpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/persistence"
	iptvhttp        "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/adapters/http"
	iptvpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/adapters/persistence"
	iptvcommands    "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/app/commands"
	iptvqueries     "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/app/queries"
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

func wireIPTV(gdb *gormsqlite.DB, swarmNodes *dockerpersistence.GormSwarmNodeRepository) *iptvWired {
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
	if swarmNodes != nil {
		nodeLookup = &swarmNodeSSHAdapter{repo: swarmNodes}
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
