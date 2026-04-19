package app

import (
	"log"

	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"

	iptvhttp        "github.com/vvs/isp/internal/modules/iptv/adapters/http"
	iptvpersistence "github.com/vvs/isp/internal/modules/iptv/adapters/persistence"
	iptvcommands    "github.com/vvs/isp/internal/modules/iptv/app/commands"
	iptvqueries     "github.com/vvs/isp/internal/modules/iptv/app/queries"
)

type iptvWired struct {
	channelRepo *iptvpersistence.ChannelRepository
	packageRepo *iptvpersistence.PackageRepository
	subRepo     *iptvpersistence.SubscriptionRepository
	stbRepo     *iptvpersistence.STBRepository
	keyRepo     *iptvpersistence.SubscriptionKeyRepository
	epgRepo     *iptvpersistence.EPGProgrammeRepository
	routes      infrahttp.ModuleRoutes
}

func wireIPTV(gdb *gormsqlite.DB) *iptvWired {
	channelRepo := iptvpersistence.NewChannelRepository(gdb)
	packageRepo := iptvpersistence.NewPackageRepository(gdb)
	subRepo     := iptvpersistence.NewSubscriptionRepository(gdb)
	stbRepo     := iptvpersistence.NewSTBRepository(gdb)
	keyRepo     := iptvpersistence.NewSubscriptionKeyRepository(gdb)
	epgRepo     := iptvpersistence.NewEPGProgrammeRepository(gdb)

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
		iptvqueries.NewListChannelsHandler(channelRepo),
		iptvqueries.NewListPackagesHandler(packageRepo),
		iptvqueries.NewListSubscriptionsHandler(subRepo, packageRepo),
		iptvqueries.NewListSTBsHandler(stbRepo),
		iptvcommands.NewImportEPGHandler(epgRepo),
	)

	log.Printf("module wired: iptv")

	return &iptvWired{
		channelRepo: channelRepo,
		packageRepo: packageRepo,
		subRepo:     subRepo,
		stbRepo:     stbRepo,
		keyRepo:     keyRepo,
		epgRepo:     epgRepo,
		routes:      routes,
	}
}
