package app

import (
	"log"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/shared/events"

	servicehttp "github.com/vvs/isp/internal/modules/service/adapters/http"
	servicepersistence "github.com/vvs/isp/internal/modules/service/adapters/persistence"
	servicecommands "github.com/vvs/isp/internal/modules/service/app/commands"
	servicequeries "github.com/vvs/isp/internal/modules/service/app/queries"
	servicedomain "github.com/vvs/isp/internal/modules/service/domain"
)

type serviceWired struct {
	repo          servicedomain.ServiceRepository
	listServices  *servicequeries.ListServicesForCustomerHandler
	assignCmd     *servicecommands.AssignServiceHandler
	suspendCmd    *servicecommands.SuspendServiceHandler
	reactivateCmd *servicecommands.ReactivateServiceHandler
	cancelCmd     *servicecommands.CancelServiceHandler
	routes        *servicehttp.ServiceHandlers // nil when module disabled
}

func wireService(gdb *gormsqlite.DB, pub events.EventPublisher, sub events.EventSubscriber, cfg Config) *serviceWired {
	serviceRepo      := servicepersistence.NewGormServiceRepository(gdb)
	listServicesQuery := servicequeries.NewListServicesForCustomerHandler(serviceRepo)

	assignServiceCmd     := servicecommands.NewAssignServiceHandler(serviceRepo, pub)
	suspendServiceCmd    := servicecommands.NewSuspendServiceHandler(serviceRepo, pub)
	reactivateServiceCmd := servicecommands.NewReactivateServiceHandler(serviceRepo, pub)
	cancelServiceCmd     := servicecommands.NewCancelServiceHandler(serviceRepo, pub)

	var routes *servicehttp.ServiceHandlers
	if cfg.IsEnabled("service") {
		routes = servicehttp.NewServiceHandlers(
			assignServiceCmd, suspendServiceCmd, reactivateServiceCmd, cancelServiceCmd,
			listServicesQuery, sub, pub,
		)
		log.Printf("module enabled: service")
	}

	return &serviceWired{
		repo:          serviceRepo,
		listServices:  listServicesQuery,
		assignCmd:     assignServiceCmd,
		suspendCmd:    suspendServiceCmd,
		reactivateCmd: reactivateServiceCmd,
		cancelCmd:     cancelServiceCmd,
		routes:        routes,
	}
}
