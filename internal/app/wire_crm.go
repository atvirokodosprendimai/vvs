package app

import (
	"log"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
	"github.com/vvs/isp/internal/shared/events"

	contacthttp "github.com/vvs/isp/internal/modules/contact/adapters/http"
	contactpersistence "github.com/vvs/isp/internal/modules/contact/adapters/persistence"
	contactcommands "github.com/vvs/isp/internal/modules/contact/app/commands"
	contactqueries "github.com/vvs/isp/internal/modules/contact/app/queries"

	dealhttp "github.com/vvs/isp/internal/modules/deal/adapters/http"
	dealpersistence "github.com/vvs/isp/internal/modules/deal/adapters/persistence"
	dealcommands "github.com/vvs/isp/internal/modules/deal/app/commands"
	dealqueries "github.com/vvs/isp/internal/modules/deal/app/queries"

	tickethttp "github.com/vvs/isp/internal/modules/ticket/adapters/http"
	ticketpersistence "github.com/vvs/isp/internal/modules/ticket/adapters/persistence"
	ticketcommands "github.com/vvs/isp/internal/modules/ticket/app/commands"
	ticketqueries "github.com/vvs/isp/internal/modules/ticket/app/queries"

	taskhttp "github.com/vvs/isp/internal/modules/task/adapters/http"
	taskpersistence "github.com/vvs/isp/internal/modules/task/adapters/persistence"
	taskcommands "github.com/vvs/isp/internal/modules/task/app/commands"
	taskqueries "github.com/vvs/isp/internal/modules/task/app/queries"
)

type crmWired struct {
	listContacts         *contactqueries.ListContactsForCustomerHandler
	listDeals            *dealqueries.ListDealsForCustomerHandler
	listAllDeals         *dealqueries.ListAllDealsHandler
	listTickets          *ticketqueries.ListTicketsForCustomerHandler
	listAllTickets       *ticketqueries.ListAllTicketsHandler
	getTicket            *ticketqueries.GetTicketHandler
	listTasksForCustomer *taskqueries.ListTasksForCustomerHandler
	listAllTasks         *taskqueries.ListAllTasksHandler
	ticketRoutes         *tickethttp.Handlers
	routes               []infrahttp.ModuleRoutes
}

func wireCRM(
	gdb *gormsqlite.DB,
	pub events.EventPublisher,
	sub events.EventSubscriber,
	cust *customerWired,
) *crmWired {
	// ── Contact ───────────────────────────────────────────────────────────────
	contactRepo    := contactpersistence.NewGormContactRepository(gdb)
	addContactCmd  := contactcommands.NewAddContactHandler(contactRepo, pub)
	updateContactCmd := contactcommands.NewUpdateContactHandler(contactRepo, pub)
	deleteContactCmd := contactcommands.NewDeleteContactHandler(contactRepo, pub)
	listContactsQuery := contactqueries.NewListContactsForCustomerHandler(gdb)

	contactRoutes := contacthttp.NewHandlers(addContactCmd, updateContactCmd, deleteContactCmd, listContactsQuery, sub)
	if cust.routes != nil {
		cust.routes.WithContactsQuery(listContactsQuery)
	}
	log.Printf("module wired: contact")

	// ── Deal ──────────────────────────────────────────────────────────────────
	dealRepo     := dealpersistence.NewGormDealRepository(gdb)
	addDealCmd   := dealcommands.NewAddDealHandler(dealRepo, pub)
	updateDealCmd := dealcommands.NewUpdateDealHandler(dealRepo, pub)
	deleteDealCmd := dealcommands.NewDeleteDealHandler(dealRepo, pub)
	advanceDealCmd := dealcommands.NewAdvanceDealHandler(dealRepo, pub)
	listDealsQuery := dealqueries.NewListDealsForCustomerHandler(dealRepo)
	listAllDealsQuery := dealqueries.NewListAllDealsHandler(dealRepo)

	dealRoutes := dealhttp.NewHandlers(addDealCmd, updateDealCmd, deleteDealCmd, advanceDealCmd, listDealsQuery, sub)
	dealRoutes.WithListAll(listAllDealsQuery)
	dealRoutes.WithCustomerNames(&dealCustomerNameBridge{repo: cust.repo})
	if cust.routes != nil {
		cust.routes.WithDealsQuery(listDealsQuery)
	}
	log.Printf("module wired: deal")

	// ── Ticket ────────────────────────────────────────────────────────────────
	ticketRepo          := ticketpersistence.NewGormTicketRepository(gdb)
	openTicketCmd       := ticketcommands.NewOpenTicketHandler(ticketRepo, pub)
	updateTicketCmd     := ticketcommands.NewUpdateTicketHandler(ticketRepo, pub)
	deleteTicketCmd     := ticketcommands.NewDeleteTicketHandler(ticketRepo, pub)
	changeTicketStatusCmd := ticketcommands.NewChangeTicketStatusHandler(ticketRepo, pub)
	addCommentCmd       := ticketcommands.NewAddCommentHandler(ticketRepo, pub)
	listTicketsQuery    := ticketqueries.NewListTicketsForCustomerHandler(ticketRepo)
	listCommentsQuery   := ticketqueries.NewListCommentsHandler(ticketRepo)

	ticketNameResolver  := &ticketCustomerNameBridge{repo: cust.repo}
	listAllTicketsQuery := ticketqueries.NewListAllTicketsHandler(ticketRepo, ticketNameResolver)
	getTicketQuery      := ticketqueries.NewGetTicketHandler(ticketRepo, ticketNameResolver)

	ticketRoutes := tickethttp.NewHandlers(
		openTicketCmd, updateTicketCmd, deleteTicketCmd,
		changeTicketStatusCmd, addCommentCmd,
		listTicketsQuery, listCommentsQuery,
		sub, pub,
	)
	ticketRoutes.WithListAll(listAllTicketsQuery)
	ticketRoutes.WithGetTicket(getTicketQuery)
	ticketRoutes.WithCustomerSearch(&ticketCustomerSearchBridge{handler: cust.listQuery})
	if cust.routes != nil {
		cust.routes.WithTicketsQuery(listTicketsQuery)
	}
	log.Printf("module wired: ticket")

	// ── Task ──────────────────────────────────────────────────────────────────
	taskRepo              := taskpersistence.NewGormTaskRepository(gdb)
	createTaskCmd         := taskcommands.NewCreateTaskHandler(taskRepo, pub)
	updateTaskCmd         := taskcommands.NewUpdateTaskHandler(taskRepo, pub)
	deleteTaskCmd         := taskcommands.NewDeleteTaskHandler(taskRepo, pub)
	changeTaskStatusCmd   := taskcommands.NewChangeTaskStatusHandler(taskRepo, pub)
	listTasksForCustomerQ := taskqueries.NewListTasksForCustomerHandler(taskRepo)
	listAllTasksQuery     := taskqueries.NewListAllTasksHandler(taskRepo)

	taskRoutes := taskhttp.NewHandlers(
		createTaskCmd, updateTaskCmd, deleteTaskCmd, changeTaskStatusCmd,
		listTasksForCustomerQ, listAllTasksQuery,
		sub, pub,
	)
	if cust.routes != nil {
		cust.routes.WithTasksQuery(listTasksForCustomerQ)
	}
	log.Printf("module wired: task")

	return &crmWired{
		listContacts:         listContactsQuery,
		listDeals:            listDealsQuery,
		listAllDeals:         listAllDealsQuery,
		listTickets:          listTicketsQuery,
		listAllTickets:       listAllTicketsQuery,
		getTicket:            getTicketQuery,
		listTasksForCustomer: listTasksForCustomerQ,
		listAllTasks:         listAllTasksQuery,
		ticketRoutes:         ticketRoutes,
		routes:               []infrahttp.ModuleRoutes{contactRoutes, dealRoutes, ticketRoutes, taskRoutes},
	}
}
