package app

import (
	"log"

	"github.com/nats-io/nats.go"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	infrahttp "github.com/atvirokodosprendimai/vvs/internal/infrastructure/http"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"

	dockerhttp "github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/http"
	dockerclientadapter "github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/dockerclient"
	dockerpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/docker/adapters/persistence"
	dockercommands "github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/commands"
	dockerqueries "github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/queries"
	dockerservices "github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/services"
)

type dockerWired struct {
	routes       infrahttp.ModuleRoutes
	swarmRoutes  infrahttp.ModuleRoutes
	swarmNodeRepo *dockerpersistence.GormSwarmNodeRepository
}

func wireDocker(
	gdb *gormsqlite.DB,
	nc *nats.Conn,
	pub events.EventPublisher,
	sub events.EventSubscriber,
	cfg Config,
) *dockerWired {
	encKey := []byte(cfg.DockerEncKey)

	// ── Docker node/service repos + commands ──────────────────────────────────
	nodeRepo := dockerpersistence.NewGormDockerNodeRepository(gdb, encKey)
	serviceRepo := dockerpersistence.NewGormDockerServiceRepository(gdb)

	factory := &dockerclientadapter.Factory{}
	logStreamer := dockerservices.NewLogStreamer(nc, factory)
	_ = logStreamer // available for future NATS-based consumers

	createNodeCmd := dockercommands.NewCreateNodeHandler(nodeRepo, pub)
	updateNodeCmd := dockercommands.NewUpdateNodeHandler(nodeRepo, pub)
	deleteNodeCmd := dockercommands.NewDeleteNodeHandler(nodeRepo, serviceRepo, pub)
	deployServiceCmd := dockercommands.NewDeployServiceHandler(nodeRepo, serviceRepo, factory, pub)
	updateServiceCmd := dockercommands.NewUpdateServiceHandler(nodeRepo, serviceRepo, factory, pub)
	stopServiceCmd := dockercommands.NewStopServiceHandler(nodeRepo, serviceRepo, factory, pub)
	startServiceCmd := dockercommands.NewStartServiceHandler(nodeRepo, serviceRepo, factory, pub)
	removeServiceCmd := dockercommands.NewRemoveServiceHandler(nodeRepo, serviceRepo, factory, pub)

	listNodesQuery := dockerqueries.NewListNodesHandler(nodeRepo)
	getNodeQuery := dockerqueries.NewGetNodeHandler(nodeRepo)
	listServicesQuery := dockerqueries.NewListServicesHandler(serviceRepo, nodeRepo)
	getServiceQuery := dockerqueries.NewGetServiceHandler(serviceRepo, nodeRepo)

	// ── Swarm repos + commands ────────────────────────────────────────────────
	swarmFactory := &dockerclientadapter.SSHSwarmFactory{}
	clusterRepo := dockerpersistence.NewGormSwarmClusterRepository(gdb, encKey)
	swarmNodeRepo := dockerpersistence.NewGormSwarmNodeRepository(gdb, encKey)
	networkRepo := dockerpersistence.NewGormSwarmNetworkRepository(gdb)
	stackRepo := dockerpersistence.NewGormSwarmStackRepository(gdb)

	createClusterCmd := dockercommands.NewCreateSwarmClusterHandler(clusterRepo, pub)
	importClusterCmd := dockercommands.NewImportSwarmClusterHandler(clusterRepo, pub)
	initSwarmCmd := dockercommands.NewInitSwarmHandler(clusterRepo, swarmNodeRepo, swarmFactory, pub)
	deleteClusterCmd := dockercommands.NewDeleteSwarmClusterHandler(clusterRepo, pub)
	updateHetznerConfigCmd := dockercommands.NewUpdateClusterHetznerConfigHandler(clusterRepo)

	createSwarmNodeCmd := dockercommands.NewCreateSwarmNodeHandler(swarmNodeRepo)
	provisionNodeCmd := dockercommands.NewProvisionSwarmNodeHandler(swarmNodeRepo, clusterRepo, pub)
	addNodeCmd := dockercommands.NewAddSwarmNodeHandler(clusterRepo, swarmNodeRepo, swarmFactory, pub)
	removeNodeCmd := dockercommands.NewRemoveSwarmNodeHandler(clusterRepo, swarmNodeRepo, swarmFactory, pub)
	orderHetznerCmd := dockercommands.NewOrderHetznerNodeHandler(clusterRepo, createSwarmNodeCmd, provisionNodeCmd, initSwarmCmd, addNodeCmd)

	createNetworkCmd := dockercommands.NewCreateSwarmNetworkHandler(clusterRepo, swarmNodeRepo, networkRepo, swarmFactory, pub)
	deleteNetworkCmd := dockercommands.NewDeleteSwarmNetworkHandler(clusterRepo, swarmNodeRepo, networkRepo, swarmFactory, pub)
	updateReservedIPCmd := dockercommands.NewUpdateSwarmNetworkReservedIPsHandler(networkRepo)

	deployStackCmd := dockercommands.NewDeploySwarmStackHandler(clusterRepo, swarmNodeRepo, stackRepo, swarmFactory, pub)
	updateStackCmd := dockercommands.NewUpdateSwarmStackHandler(swarmNodeRepo, stackRepo, swarmFactory)
	removeStackCmd := dockercommands.NewRemoveSwarmStackHandler(swarmNodeRepo, stackRepo, swarmFactory, pub)

	listClustersQuery := dockerqueries.NewListSwarmClustersHandler(clusterRepo, swarmNodeRepo)
	getClusterQuery := dockerqueries.NewGetSwarmClusterHandler(clusterRepo)
	listSwarmNodesQuery := dockerqueries.NewListSwarmNodesHandler(swarmNodeRepo)
	getSwarmNodeQuery := dockerqueries.NewGetSwarmNodeHandler(swarmNodeRepo)
	listNetworksQuery := dockerqueries.NewListSwarmNetworksHandler(networkRepo)
	getNetworkQuery := dockerqueries.NewGetSwarmNetworkHandler(networkRepo)
	listStacksQuery := dockerqueries.NewListSwarmStacksHandler(stackRepo, swarmNodeRepo)
	getStackQuery := dockerqueries.NewGetSwarmStackHandler(stackRepo, swarmNodeRepo)

	var routes infrahttp.ModuleRoutes
	var swarmRoutes infrahttp.ModuleRoutes

	if cfg.IsEnabled("docker") {
		routes = dockerhttp.NewHandlers(
			createNodeCmd, updateNodeCmd, deleteNodeCmd,
			deployServiceCmd, updateServiceCmd, stopServiceCmd, startServiceCmd, removeServiceCmd,
			listNodesQuery, getNodeQuery, listServicesQuery, getServiceQuery,
			sub, nodeRepo, serviceRepo, factory,
		)
		swarmRoutes = dockerhttp.NewSwarmHandlers(
			createClusterCmd, importClusterCmd, initSwarmCmd, deleteClusterCmd,
			updateHetznerConfigCmd,
			provisionNodeCmd, addNodeCmd, removeNodeCmd, createSwarmNodeCmd,
			orderHetznerCmd,
			createNetworkCmd, deleteNetworkCmd, updateReservedIPCmd,
			deployStackCmd, updateStackCmd, removeStackCmd,
			listClustersQuery, getClusterQuery,
			listSwarmNodesQuery, getSwarmNodeQuery,
			listNetworksQuery, getNetworkQuery,
			listStacksQuery, getStackQuery,
			networkRepo,
			clusterRepo,
		)
		log.Printf("module enabled: docker")
	}

	return &dockerWired{routes: routes, swarmRoutes: swarmRoutes, swarmNodeRepo: swarmNodeRepo}
}
