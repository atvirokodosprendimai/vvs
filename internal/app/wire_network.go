package app

import (
	"context"
	"log"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/arista"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/mikrotik"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/netbox"
	infrahttp "github.com/atvirokodosprendimai/vvs/internal/infrastructure/http"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"

	customercommands "github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/commands"
	networkdomain "github.com/atvirokodosprendimai/vvs/internal/modules/network/domain"

	networkhttp "github.com/atvirokodosprendimai/vvs/internal/modules/network/adapters/http"
	networkpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/network/adapters/persistence"
	networkcommands "github.com/atvirokodosprendimai/vvs/internal/modules/network/app/commands"
	networkqueries "github.com/atvirokodosprendimai/vvs/internal/modules/network/app/queries"
	networkservices "github.com/atvirokodosprendimai/vvs/internal/modules/network/app/services"
	networksubscribers "github.com/atvirokodosprendimai/vvs/internal/modules/network/app/subscribers"

	devicehttp "github.com/atvirokodosprendimai/vvs/internal/modules/device/adapters/http"
	devicepersistence "github.com/atvirokodosprendimai/vvs/internal/modules/device/adapters/persistence"
	devicecommands "github.com/atvirokodosprendimai/vvs/internal/modules/device/app/commands"
	devicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/device/app/queries"
)

type networkWired struct {
	createCustomer *customercommands.CreateCustomerHandler
	routerRepo     networkdomain.RouterRepository
	prefixRepo     networkdomain.PrefixRepository
	syncARP        *networkcommands.SyncCustomerARPHandler
	createRouter   *networkcommands.CreateRouterHandler
	updateRouter   *networkcommands.UpdateRouterHandler
	deleteRouter   *networkcommands.DeleteRouterHandler
	listRouters    *networkqueries.ListRoutersHandler
	getRouter      *networkqueries.GetRouterHandler
	routes         infrahttp.ModuleRoutes
}

func wireNetwork(
	gdb *gormsqlite.DB,
	pub events.EventPublisher,
	sub events.EventSubscriber,
	cust *customerWired,
	cfg Config,
) *networkWired {
	routerRepo := networkpersistence.NewGormRouterRepository(gdb, []byte(cfg.RouterEncKey))
	prefixRepo := networkpersistence.NewGormPrefixRepository(gdb)

	provisioner := &provisionerDispatcher{
		mikrotik: mikrotik.New(),
		arista:   arista.New(),
	}

	var ipamProvider networkdomain.IPAMProvider
	if cfg.NetBoxURL != "" && cfg.NetBoxToken != "" {
		nbClient := netbox.New(cfg.NetBoxURL, cfg.NetBoxToken)
		ipamProvider = networkservices.NewIPAllocatorService(prefixRepo, nbClient)
		log.Printf("NetBox IPAM configured: %s", cfg.NetBoxURL)
	}

	// createCustomerCmd lives here because it needs ipamProvider from network config.
	createCustomerCmd := customercommands.NewCreateCustomerHandler(cust.repo, pub, ipamProvider)

	syncARPCmd := networkcommands.NewSyncCustomerARPHandler(
		&customerARPBridge{repo: cust.repo}, routerRepo, provisioner, ipamProvider, pub,
	)

	createRouterCmd := networkcommands.NewCreateRouterHandler(routerRepo, pub)
	updateRouterCmd := networkcommands.NewUpdateRouterHandler(routerRepo, pub)
	deleteRouterCmd := networkcommands.NewDeleteRouterHandler(routerRepo, pub)
	listRoutersQuery := networkqueries.NewListRoutersHandler(routerRepo)
	getRouterQuery   := networkqueries.NewGetRouterHandler(routerRepo)

	var routes infrahttp.ModuleRoutes
	// createCustomerCmd always available; inject it regardless of network enabled state.
	if cust.routes != nil {
		cust.routes.WithCreateCmd(createCustomerCmd)
	}

	if cfg.IsEnabled("network") {
		if cust.routes != nil {
			cust.routes.WithReader(gdb.R)
		}
		routes = networkhttp.NewHandlers(
			createRouterCmd, updateRouterCmd, deleteRouterCmd,
			listRoutersQuery, getRouterQuery, syncARPCmd, prefixRepo, sub,
		)
		arpWorker := networksubscribers.NewARPWorker(syncARPCmd)
		go arpWorker.Run(context.Background(), sub)
		log.Printf("module enabled: network")
	}

	return &networkWired{
		createCustomer: createCustomerCmd,
		routerRepo:     routerRepo,
		prefixRepo:     prefixRepo,
		syncARP:        syncARPCmd,
		createRouter:   createRouterCmd,
		updateRouter:   updateRouterCmd,
		deleteRouter:   deleteRouterCmd,
		listRouters:    listRoutersQuery,
		getRouter:      getRouterQuery,
		routes:         routes,
	}
}

// ── Device ────────────────────────────────────────────────────────────────────

type deviceWired struct {
	repo        *devicepersistence.GormDeviceRepository
	registerCmd *devicecommands.RegisterDeviceHandler
	deployCmd   *devicecommands.DeployDeviceHandler
	returnCmd   *devicecommands.ReturnDeviceHandler
	decommCmd   *devicecommands.DecommissionDeviceHandler
	updateCmd   *devicecommands.UpdateDeviceHandler
	listQuery   *devicequeries.ListDevicesHandler
	getQuery    *devicequeries.GetDeviceHandler
	routes      infrahttp.ModuleRoutes
}

func wireDevice(gdb *gormsqlite.DB, pub events.EventPublisher, sub events.EventSubscriber, cfg Config) *deviceWired {
	deviceRepo := devicepersistence.NewGormDeviceRepository(gdb)

	registerDeviceCmd    := devicecommands.NewRegisterDeviceHandler(deviceRepo, pub)
	deployDeviceCmd      := devicecommands.NewDeployDeviceHandler(deviceRepo, pub)
	returnDeviceCmd      := devicecommands.NewReturnDeviceHandler(deviceRepo, pub)
	decommissionDeviceCmd := devicecommands.NewDecommissionDeviceHandler(deviceRepo, pub)
	updateDeviceCmd      := devicecommands.NewUpdateDeviceHandler(deviceRepo, pub)
	listDevicesQuery     := devicequeries.NewListDevicesHandler(gdb)
	getDeviceQuery       := devicequeries.NewGetDeviceHandler(gdb)

	var routes infrahttp.ModuleRoutes
	if cfg.IsEnabled("device") {
		routes = devicehttp.NewDeviceHandlers(
			registerDeviceCmd, deployDeviceCmd, returnDeviceCmd,
			decommissionDeviceCmd, updateDeviceCmd,
			listDevicesQuery, getDeviceQuery,
			sub, pub,
		)
		log.Printf("module enabled: device")
	}

	return &deviceWired{
		repo:        deviceRepo,
		registerCmd: registerDeviceCmd,
		deployCmd:   deployDeviceCmd,
		returnCmd:   returnDeviceCmd,
		decommCmd:   decommissionDeviceCmd,
		updateCmd:   updateDeviceCmd,
		listQuery:   listDevicesQuery,
		getQuery:    getDeviceQuery,
		routes:      routes,
	}
}
