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
	routes infrahttp.ModuleRoutes
}

func wireDocker(
	gdb *gormsqlite.DB,
	nc *nats.Conn,
	pub events.EventPublisher,
	sub events.EventSubscriber,
	cfg Config,
) *dockerWired {
	encKey := []byte(cfg.DockerEncKey)
	nodeRepo := dockerpersistence.NewGormDockerNodeRepository(gdb, encKey)
	serviceRepo := dockerpersistence.NewGormDockerServiceRepository(gdb)

	factory := &dockerclientadapter.Factory{}
	logStreamer := dockerservices.NewLogStreamer(nc, factory)
	_ = logStreamer // available for future NATS-based consumers

	createNodeCmd := dockercommands.NewCreateNodeHandler(nodeRepo, pub)
	updateNodeCmd := dockercommands.NewUpdateNodeHandler(nodeRepo, pub)
	deleteNodeCmd := dockercommands.NewDeleteNodeHandler(nodeRepo, serviceRepo, pub)
	deployServiceCmd := dockercommands.NewDeployServiceHandler(nodeRepo, serviceRepo, factory, pub)
	stopServiceCmd := dockercommands.NewStopServiceHandler(nodeRepo, serviceRepo, factory, pub)
	startServiceCmd := dockercommands.NewStartServiceHandler(nodeRepo, serviceRepo, factory, pub)
	removeServiceCmd := dockercommands.NewRemoveServiceHandler(nodeRepo, serviceRepo, factory, pub)

	listNodesQuery := dockerqueries.NewListNodesHandler(nodeRepo)
	getNodeQuery := dockerqueries.NewGetNodeHandler(nodeRepo)
	listServicesQuery := dockerqueries.NewListServicesHandler(serviceRepo, nodeRepo)
	getServiceQuery := dockerqueries.NewGetServiceHandler(serviceRepo, nodeRepo)

	var routes infrahttp.ModuleRoutes
	if cfg.IsEnabled("docker") {
		routes = dockerhttp.NewHandlers(
			createNodeCmd, updateNodeCmd, deleteNodeCmd,
			deployServiceCmd, stopServiceCmd, startServiceCmd, removeServiceCmd,
			listNodesQuery, getNodeQuery, listServicesQuery, getServiceQuery,
			sub, nodeRepo, serviceRepo, factory,
		)
		log.Printf("module enabled: docker")
	}

	return &dockerWired{routes: routes}
}
