package app

import (
	"log"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	infrahttp "github.com/atvirokodosprendimai/vvs/internal/infrastructure/http"
	proxmoxinfra "github.com/atvirokodosprendimai/vvs/internal/infrastructure/proxmox"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"

	proxmoxhttp "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/adapters/http"
	proxmoxpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/adapters/persistence"
	proxmoxcommands "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/app/commands"
	proxmoxqueries "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/app/queries"
)

type proxmoxWired struct {
	listVMsForCustomer *proxmoxqueries.ListVMsForCustomerHandler
	routes             infrahttp.ModuleRoutes
}

func wireProxmox(
	gdb *gormsqlite.DB,
	pub events.EventPublisher,
	sub events.EventSubscriber,
	cust *customerWired,
	cfg Config,
) *proxmoxWired {
	encKey := []byte(cfg.ProxmoxEncKey)
	nodeRepo := proxmoxpersistence.NewGormNodeRepository(gdb, encKey)
	vmRepo := proxmoxpersistence.NewGormVMRepository(gdb)
	provisioner := proxmoxinfra.New()

	createNodeCmd := proxmoxcommands.NewCreateNodeHandler(nodeRepo, pub)
	updateNodeCmd := proxmoxcommands.NewUpdateNodeHandler(nodeRepo, pub)
	deleteNodeCmd := proxmoxcommands.NewDeleteNodeHandler(nodeRepo, vmRepo, pub)
	createVMCmd := proxmoxcommands.NewCreateVMHandler(nodeRepo, vmRepo, provisioner, pub)
	suspendVMCmd := proxmoxcommands.NewSuspendVMHandler(nodeRepo, vmRepo, provisioner, pub)
	resumeVMCmd := proxmoxcommands.NewResumeVMHandler(nodeRepo, vmRepo, provisioner, pub)
	restartVMCmd := proxmoxcommands.NewRestartVMHandler(nodeRepo, vmRepo, provisioner, pub)
	deleteVMCmd := proxmoxcommands.NewDeleteVMHandler(nodeRepo, vmRepo, provisioner, pub)
	assignVMCustomerCmd := proxmoxcommands.NewAssignVMCustomerHandler(vmRepo, pub)

	planRepo := proxmoxpersistence.NewGormVMPlanRepository(gdb)
	createVMPlanCmd := proxmoxcommands.NewCreateVMPlanHandler(planRepo, pub)
	updateVMPlanCmd := proxmoxcommands.NewUpdateVMPlanHandler(planRepo, pub)
	deleteVMPlanCmd := proxmoxcommands.NewDeleteVMPlanHandler(planRepo, pub)

	listNodesQuery := proxmoxqueries.NewListNodesHandler(nodeRepo)
	getNodeQuery := proxmoxqueries.NewGetNodeHandler(nodeRepo)
	listVMsQuery := proxmoxqueries.NewListVMsHandler(vmRepo, nodeRepo)
	getVMQuery := proxmoxqueries.NewGetVMHandler(vmRepo, nodeRepo)
	listVMsForCustomer := proxmoxqueries.NewListVMsForCustomerHandler(vmRepo, nodeRepo)
	listVMPlansQuery := proxmoxqueries.NewListVMPlansHandler(planRepo)
	getVMPlanQuery := proxmoxqueries.NewGetVMPlanHandler(planRepo)

	var routes infrahttp.ModuleRoutes
	if cfg.IsEnabled("proxmox") {
		if cust.routes != nil {
			cust.routes.WithVMsForCustomerQuery(listVMsForCustomer)
		}
		routes = proxmoxhttp.NewHandlers(
			createNodeCmd, updateNodeCmd, deleteNodeCmd,
			createVMCmd, suspendVMCmd, resumeVMCmd, restartVMCmd, deleteVMCmd, assignVMCustomerCmd,
			createVMPlanCmd, updateVMPlanCmd, deleteVMPlanCmd,
			listNodesQuery, getNodeQuery, listVMsQuery, getVMQuery, listVMsForCustomer,
			listVMPlansQuery, getVMPlanQuery,
			sub,
		)
		log.Printf("module enabled: proxmox")
	}

	return &proxmoxWired{
		listVMsForCustomer: listVMsForCustomer,
		routes:             routes,
	}
}
