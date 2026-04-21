// Package rpc implements a NATS request/reply server that exposes all ISP
// management commands and queries on isp.rpc.{module}.{action} subjects.
//
// Callers send a JSON-encoded request payload and receive a JSON response:
//
//	{"data": <result>}          on success
//	{"error": "<message>"}      on failure
//
// Example subjects:
//
//	isp.rpc.customer.list
//	isp.rpc.customer.create
//	isp.rpc.product.get
//	isp.rpc.service.assign
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	authcommands "github.com/atvirokodosprendimai/vvs/internal/modules/auth/app/commands"
	authqueries "github.com/atvirokodosprendimai/vvs/internal/modules/auth/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	customercommands "github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/commands"
	customerqueries "github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/queries"
	customerdomain "github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
	networkcommands "github.com/atvirokodosprendimai/vvs/internal/modules/network/app/commands"
	networkqueries "github.com/atvirokodosprendimai/vvs/internal/modules/network/app/queries"
	networkdomain "github.com/atvirokodosprendimai/vvs/internal/modules/network/domain"
	productcommands "github.com/atvirokodosprendimai/vvs/internal/modules/product/app/commands"
	productqueries "github.com/atvirokodosprendimai/vvs/internal/modules/product/app/queries"
	productdomain "github.com/atvirokodosprendimai/vvs/internal/modules/product/domain"
	servicecommands "github.com/atvirokodosprendimai/vvs/internal/modules/service/app/commands"
	servicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/service/app/queries"
	servicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/service/domain"

	devicecommands "github.com/atvirokodosprendimai/vvs/internal/modules/device/app/commands"
	devicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/device/app/queries"
	devicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/device/domain"

	croncommands "github.com/atvirokodosprendimai/vvs/internal/modules/cron/app/commands"
	cronqueries "github.com/atvirokodosprendimai/vvs/internal/modules/cron/app/queries"
	crondomain "github.com/atvirokodosprendimai/vvs/internal/modules/cron/domain"

	invoicecommands "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/commands"
	invoicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/queries"
	invoicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"
)

type envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// Server subscribes to isp.rpc.* and dispatches to the correct handler.
type Server struct {
	nc   *nats.Conn
	subs []*nats.Subscription
	mu   sync.Mutex

	// auth
	listUsers      *authqueries.ListUsersHandler
	createUser     *authcommands.CreateUserHandler
	deleteUser     *authcommands.DeleteUserHandler
	changePassword *authcommands.ChangePasswordHandler

	// customer
	listCustomers   *customerqueries.ListCustomersHandler
	getCustomer     *customerqueries.GetCustomerHandler
	createCustomer  *customercommands.CreateCustomerHandler
	updateCustomer  *customercommands.UpdateCustomerHandler
	deleteCustomer  *customercommands.DeleteCustomerHandler

	// product
	listProducts   *productqueries.ListProductsHandler
	getProduct     *productqueries.GetProductHandler
	createProduct  *productcommands.CreateProductHandler
	updateProduct  *productcommands.UpdateProductHandler
	deleteProduct  *productcommands.DeleteProductHandler

	// network
	listRouters *networkqueries.ListRoutersHandler
	getRouter   *networkqueries.GetRouterHandler
	createRouter *networkcommands.CreateRouterHandler
	updateRouter *networkcommands.UpdateRouterHandler
	deleteRouter *networkcommands.DeleteRouterHandler
	syncARP      *networkcommands.SyncCustomerARPHandler

	// service
	listServices  *servicequeries.ListServicesForCustomerHandler
	assignService *servicecommands.AssignServiceHandler
	suspendService *servicecommands.SuspendServiceHandler
	reactivateService *servicecommands.ReactivateServiceHandler
	cancelService *servicecommands.CancelServiceHandler

	// device
	listDevices      *devicequeries.ListDevicesHandler
	getDevice        *devicequeries.GetDeviceHandler
	registerDevice   *devicecommands.RegisterDeviceHandler
	deployDevice     *devicecommands.DeployDeviceHandler
	returnDevice     *devicecommands.ReturnDeviceHandler
	decommissionDevice *devicecommands.DecommissionDeviceHandler
	updateDevice     *devicecommands.UpdateDeviceHandler

	// cron
	listJobs   *cronqueries.ListJobsHandler
	getJob     *cronqueries.GetJobHandler
	addJob     *croncommands.AddJobHandler
	pauseJob   *croncommands.PauseJobHandler
	resumeJob  *croncommands.ResumeJobHandler
	deleteJob  *croncommands.DeleteJobHandler

	// invoice
	listAllInvoices        *invoicequeries.ListAllInvoicesHandler
	getInvoice             *invoicequeries.GetInvoiceHandler
	listInvoicesForCust    *invoicequeries.ListInvoicesForCustomerHandler
	createInvoice          *invoicecommands.CreateInvoiceHandler
	finalizeInvoice        *invoicecommands.FinalizeInvoiceHandler
	markPaidInvoice        *invoicecommands.MarkPaidHandler
	voidInvoice            *invoicecommands.VoidInvoiceHandler
	generateInvoice        *invoicecommands.GenerateFromSubscriptionsHandler
	addInvoiceLine         *invoicecommands.AddLineItemHandler
	updateInvoiceLine      *invoicecommands.UpdateLineItemHandler
	removeInvoiceLine      *invoicecommands.RemoveLineItemHandler
}

type Config struct {
	ListUsers      *authqueries.ListUsersHandler
	CreateUser     *authcommands.CreateUserHandler
	DeleteUser     *authcommands.DeleteUserHandler
	ChangePassword *authcommands.ChangePasswordHandler

	ListCustomers  *customerqueries.ListCustomersHandler
	GetCustomer    *customerqueries.GetCustomerHandler
	CreateCustomer *customercommands.CreateCustomerHandler
	UpdateCustomer *customercommands.UpdateCustomerHandler
	DeleteCustomer *customercommands.DeleteCustomerHandler

	ListProducts  *productqueries.ListProductsHandler
	GetProduct    *productqueries.GetProductHandler
	CreateProduct *productcommands.CreateProductHandler
	UpdateProduct *productcommands.UpdateProductHandler
	DeleteProduct *productcommands.DeleteProductHandler

	ListRouters  *networkqueries.ListRoutersHandler
	GetRouter    *networkqueries.GetRouterHandler
	CreateRouter *networkcommands.CreateRouterHandler
	UpdateRouter *networkcommands.UpdateRouterHandler
	DeleteRouter *networkcommands.DeleteRouterHandler
	SyncARP      *networkcommands.SyncCustomerARPHandler

	ListServices      *servicequeries.ListServicesForCustomerHandler
	AssignService     *servicecommands.AssignServiceHandler
	SuspendService    *servicecommands.SuspendServiceHandler
	ReactivateService *servicecommands.ReactivateServiceHandler
	CancelService     *servicecommands.CancelServiceHandler

	ListDevices        *devicequeries.ListDevicesHandler
	GetDevice          *devicequeries.GetDeviceHandler
	RegisterDevice     *devicecommands.RegisterDeviceHandler
	DeployDevice       *devicecommands.DeployDeviceHandler
	ReturnDevice       *devicecommands.ReturnDeviceHandler
	DecommissionDevice *devicecommands.DecommissionDeviceHandler
	UpdateDevice       *devicecommands.UpdateDeviceHandler

	ListJobs  *cronqueries.ListJobsHandler
	GetJob    *cronqueries.GetJobHandler
	AddJob    *croncommands.AddJobHandler
	PauseJob  *croncommands.PauseJobHandler
	ResumeJob *croncommands.ResumeJobHandler
	DeleteJob *croncommands.DeleteJobHandler

	// invoice
	ListAllInvoices        *invoicequeries.ListAllInvoicesHandler
	GetInvoice             *invoicequeries.GetInvoiceHandler
	ListInvoicesForCust    *invoicequeries.ListInvoicesForCustomerHandler
	CreateInvoice          *invoicecommands.CreateInvoiceHandler
	FinalizeInvoice        *invoicecommands.FinalizeInvoiceHandler
	MarkPaidInvoice        *invoicecommands.MarkPaidHandler
	VoidInvoice            *invoicecommands.VoidInvoiceHandler
	GenerateInvoice        *invoicecommands.GenerateFromSubscriptionsHandler
	AddInvoiceLine         *invoicecommands.AddLineItemHandler
	UpdateInvoiceLine      *invoicecommands.UpdateLineItemHandler
	RemoveInvoiceLine      *invoicecommands.RemoveLineItemHandler
}

func New(nc *nats.Conn, cfg Config) *Server {
	return &Server{
		nc:                nc,
		listUsers:      cfg.ListUsers,
		createUser:     cfg.CreateUser,
		deleteUser:     cfg.DeleteUser,
		changePassword: cfg.ChangePassword,
		listCustomers:     cfg.ListCustomers,
		getCustomer:       cfg.GetCustomer,
		createCustomer:    cfg.CreateCustomer,
		updateCustomer:    cfg.UpdateCustomer,
		deleteCustomer:    cfg.DeleteCustomer,
		listProducts:      cfg.ListProducts,
		getProduct:        cfg.GetProduct,
		createProduct:     cfg.CreateProduct,
		updateProduct:     cfg.UpdateProduct,
		deleteProduct:     cfg.DeleteProduct,
		listRouters:       cfg.ListRouters,
		getRouter:         cfg.GetRouter,
		createRouter:      cfg.CreateRouter,
		updateRouter:      cfg.UpdateRouter,
		deleteRouter:      cfg.DeleteRouter,
		syncARP:           cfg.SyncARP,
		listServices:      cfg.ListServices,
		assignService:     cfg.AssignService,
		suspendService:    cfg.SuspendService,
		reactivateService: cfg.ReactivateService,
		cancelService:     cfg.CancelService,

		listDevices:        cfg.ListDevices,
		getDevice:          cfg.GetDevice,
		registerDevice:     cfg.RegisterDevice,
		deployDevice:       cfg.DeployDevice,
		returnDevice:       cfg.ReturnDevice,
		decommissionDevice: cfg.DecommissionDevice,
		updateDevice:       cfg.UpdateDevice,

		listJobs:  cfg.ListJobs,
		getJob:    cfg.GetJob,
		addJob:    cfg.AddJob,
		pauseJob:  cfg.PauseJob,
		resumeJob: cfg.ResumeJob,
		deleteJob: cfg.DeleteJob,

		listAllInvoices:     cfg.ListAllInvoices,
		getInvoice:          cfg.GetInvoice,
		listInvoicesForCust: cfg.ListInvoicesForCust,
		createInvoice:       cfg.CreateInvoice,
		finalizeInvoice:     cfg.FinalizeInvoice,
		markPaidInvoice:     cfg.MarkPaidInvoice,
		voidInvoice:         cfg.VoidInvoice,
		generateInvoice:     cfg.GenerateInvoice,
		addInvoiceLine:      cfg.AddInvoiceLine,
		updateInvoiceLine:   cfg.UpdateInvoiceLine,
		removeInvoiceLine:   cfg.RemoveInvoiceLine,
	}
}

// handlers returns the full subject → handler dispatch table.
func (s *Server) handlers() map[string]func(context.Context, []byte) (any, error) {
	return map[string]func(context.Context, []byte) (any, error){
		// auth
		"isp.rpc.user.list":            s.handleUserList,
		"isp.rpc.user.create":          s.handleUserCreate,
		"isp.rpc.user.delete":          s.handleUserDelete,
		"isp.rpc.user.change-password": s.handleUserChangePassword,
		// customer
		"isp.rpc.customer.list":   s.handleCustomerList,
		"isp.rpc.customer.get":    s.handleCustomerGet,
		"isp.rpc.customer.create": s.handleCustomerCreate,
		"isp.rpc.customer.update": s.handleCustomerUpdate,
		"isp.rpc.customer.delete": s.handleCustomerDelete,
		// product
		"isp.rpc.product.list":   s.handleProductList,
		"isp.rpc.product.get":    s.handleProductGet,
		"isp.rpc.product.create": s.handleProductCreate,
		"isp.rpc.product.update": s.handleProductUpdate,
		"isp.rpc.product.delete": s.handleProductDelete,
		// network
		"isp.rpc.router.list":     s.handleRouterList,
		"isp.rpc.router.get":      s.handleRouterGet,
		"isp.rpc.router.create":   s.handleRouterCreate,
		"isp.rpc.router.update":   s.handleRouterUpdate,
		"isp.rpc.router.delete":   s.handleRouterDelete,
		"isp.rpc.router.sync-arp": s.handleRouterSyncARP,
		// service
		"isp.rpc.service.list":       s.handleServiceList,
		"isp.rpc.service.assign":     s.handleServiceAssign,
		"isp.rpc.service.suspend":    s.handleServiceSuspend,
		"isp.rpc.service.reactivate": s.handleServiceReactivate,
		"isp.rpc.service.cancel":     s.handleServiceCancel,
		// device
		"isp.rpc.device.list":         s.handleDeviceList,
		"isp.rpc.device.get":          s.handleDeviceGet,
		"isp.rpc.device.register":     s.handleDeviceRegister,
		"isp.rpc.device.deploy":       s.handleDeviceDeploy,
		"isp.rpc.device.return":       s.handleDeviceReturn,
		"isp.rpc.device.decommission": s.handleDeviceDecommission,
		"isp.rpc.device.update":       s.handleDeviceUpdate,
		// cron
		"isp.rpc.cron.list":   s.handleCronList,
		"isp.rpc.cron.get":    s.handleCronGet,
		"isp.rpc.cron.add":    s.handleCronAdd,
		"isp.rpc.cron.pause":  s.handleCronPause,
		"isp.rpc.cron.resume": s.handleCronResume,
		"isp.rpc.cron.delete": s.handleCronDelete,
		// invoice
		"isp.rpc.invoice.list":              s.handleInvoiceList,
		"isp.rpc.invoice.get":               s.handleInvoiceGet,
		"isp.rpc.invoice.list-for-customer": s.handleInvoiceListForCustomer,
		"isp.rpc.invoice.create":            s.handleInvoiceCreate,
		"isp.rpc.invoice.finalize":          s.handleInvoiceFinalize,
		"isp.rpc.invoice.mark-paid":         s.handleInvoiceMarkPaid,
		"isp.rpc.invoice.void":              s.handleInvoiceVoid,
		"isp.rpc.invoice.generate":          s.handleInvoiceGenerate,
		"isp.rpc.invoice.add-line":          s.handleInvoiceAddLine,
		"isp.rpc.invoice.update-line":       s.handleInvoiceUpdateLine,
		"isp.rpc.invoice.remove-line":       s.handleInvoiceRemoveLine,
	}
}

// Dispatch executes the handler for a subject directly (no NATS round-trip).
// Used by the HTTP RPC endpoint for CLI fallback transport.
func (s *Server) Dispatch(ctx context.Context, subject string, payload []byte) (any, error) {
	h, ok := s.handlers()[subject]
	if !ok {
		return nil, fmt.Errorf("unknown subject: %s", subject)
	}
	return h(ctx, payload)
}

// Register subscribes all RPC subjects. Call once on startup.
func (s *Server) Register() error {
	routes := s.handlers()

	s.mu.Lock()
	defer s.mu.Unlock()

	for subject, handler := range routes {
		h := handler // capture
		sub, err := s.nc.Subscribe(subject, func(msg *nats.Msg) {
			if msg.Reply == "" {
				return // not a request — ignore
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			result, err := h(ctx, msg.Data)
			reply(msg, result, err)
		})
		if err != nil {
			return err
		}
		s.subs = append(s.subs, sub)
	}
	log.Printf("NATS RPC: registered %d subjects", len(routes))
	return nil
}

// Close unsubscribes all RPC subjects.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		_ = sub.Unsubscribe()
	}
	s.subs = nil
}

func reply(msg *nats.Msg, data any, err error) {
	var env envelope
	if err != nil {
		env = envelope{Error: err.Error()}
	} else {
		env = envelope{Data: data}
	}
	b, _ := json.Marshal(env)
	_ = msg.Respond(b)
}

// ── auth ─────────────────────────────────────────────────────────────────────

func (s *Server) handleUserList(ctx context.Context, _ []byte) (any, error) {
	return s.listUsers.Handle(ctx)
}

func (s *Server) handleUserCreate(ctx context.Context, payload []byte) (any, error) {
	var req struct {
		Username string      `json:"username"`
		Password string      `json:"password"`
		Role     domain.Role `json:"role"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	u, err := s.createUser.Handle(ctx, authcommands.CreateUserCommand{
		Username: req.Username, Password: req.Password, Role: req.Role,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": u.ID, "username": u.Username, "role": u.Role}, nil
}

func (s *Server) handleUserDelete(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return nil, s.deleteUser.Handle(ctx, authcommands.DeleteUserCommand{ID: req.ID})
}

func (s *Server) handleUserChangePassword(ctx context.Context, payload []byte) (any, error) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	// Resolve username → ID
	users, err := s.listUsers.Handle(ctx)
	if err != nil {
		return nil, err
	}
	var userID string
	for _, u := range users {
		if u.Username == req.Username {
			userID = u.ID
			break
		}
	}
	if userID == "" {
		return nil, domain.ErrUserNotFound
	}
	return nil, s.changePassword.Handle(ctx, authcommands.ChangePasswordCommand{
		UserID:      userID,
		NewPassword: req.Password,
	})
}

// ── customer ─────────────────────────────────────────────────────────────────

func (s *Server) handleCustomerList(ctx context.Context, payload []byte) (any, error) {
	var req customerqueries.ListCustomersQuery
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &req)
	}
	if req.PageSize == 0 {
		req.PageSize = 25
	}
	return s.listCustomers.Handle(ctx, req)
}

func (s *Server) handleCustomerGet(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	c, err := s.getCustomer.Handle(ctx, customerqueries.GetCustomerQuery{ID: req.ID})
	if err != nil {
		if errors.Is(err, customerdomain.ErrCustomerNotFound) {
			return nil, errors.New("not found")
		}
		return nil, err
	}
	return c, nil
}

func (s *Server) handleCustomerCreate(ctx context.Context, payload []byte) (any, error) {
	var req customercommands.CreateCustomerCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.createCustomer.Handle(ctx, req)
}

func (s *Server) handleCustomerUpdate(ctx context.Context, payload []byte) (any, error) {
	var req customercommands.UpdateCustomerCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return nil, s.updateCustomer.Handle(ctx, req)
}

func (s *Server) handleCustomerDelete(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return nil, s.deleteCustomer.Handle(ctx, customercommands.DeleteCustomerCommand{ID: req.ID})
}

// ── product ──────────────────────────────────────────────────────────────────

func (s *Server) handleProductList(ctx context.Context, payload []byte) (any, error) {
	var req productqueries.ListProductsQuery
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &req)
	}
	if req.PageSize == 0 {
		req.PageSize = 25
	}
	return s.listProducts.Handle(ctx, req)
}

func (s *Server) handleProductGet(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	p, err := s.getProduct.Handle(ctx, productqueries.GetProductQuery{ID: req.ID})
	if err != nil {
		if errors.Is(err, productdomain.ErrProductNotFound) {
			return nil, errors.New("not found")
		}
		return nil, err
	}
	return p, nil
}

func (s *Server) handleProductCreate(ctx context.Context, payload []byte) (any, error) {
	var req productcommands.CreateProductCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.createProduct.Handle(ctx, req)
}

func (s *Server) handleProductUpdate(ctx context.Context, payload []byte) (any, error) {
	var req productcommands.UpdateProductCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return nil, s.updateProduct.Handle(ctx, req)
}

func (s *Server) handleProductDelete(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return nil, s.deleteProduct.Handle(ctx, productcommands.DeleteProductCommand{ID: req.ID})
}

// ── network ──────────────────────────────────────────────────────────────────

func (s *Server) handleRouterList(ctx context.Context, _ []byte) (any, error) {
	return s.listRouters.Handle(ctx)
}

func (s *Server) handleRouterGet(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	rm, err := s.getRouter.Handle(ctx, req.ID)
	if err != nil {
		if errors.Is(err, networkdomain.ErrRouterNotFound) {
			return nil, errors.New("not found")
		}
		return nil, err
	}
	return rm, nil
}

func (s *Server) handleRouterCreate(ctx context.Context, payload []byte) (any, error) {
	var req networkcommands.CreateRouterCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	router, err := s.createRouter.Handle(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.getRouter.Handle(ctx, router.ID)
}

func (s *Server) handleRouterUpdate(ctx context.Context, payload []byte) (any, error) {
	var req networkcommands.UpdateRouterCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	_, err := s.updateRouter.Handle(ctx, req)
	return nil, err
}

func (s *Server) handleRouterDelete(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return nil, s.deleteRouter.Handle(ctx, req.ID)
}

func (s *Server) handleRouterSyncARP(ctx context.Context, payload []byte) (any, error) {
	var req networkcommands.SyncCustomerARPCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return nil, s.syncARP.Handle(ctx, req)
}

// ── service ──────────────────────────────────────────────────────────────────

func (s *Server) handleServiceList(ctx context.Context, payload []byte) (any, error) {
	var req servicequeries.ListServicesForCustomerQuery
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.listServices.Handle(ctx, req)
}

func (s *Server) handleServiceAssign(ctx context.Context, payload []byte) (any, error) {
	var req servicecommands.AssignServiceCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.assignService.Handle(ctx, req)
}

func (s *Server) handleServiceSuspend(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	err := s.suspendService.Handle(ctx, servicecommands.SuspendServiceCommand{ID: req.ID})
	if errors.Is(err, servicedomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	return nil, err
}

func (s *Server) handleServiceReactivate(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	err := s.reactivateService.Handle(ctx, servicecommands.ReactivateServiceCommand{ID: req.ID})
	if errors.Is(err, servicedomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	return nil, err
}

func (s *Server) handleServiceCancel(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	err := s.cancelService.Handle(ctx, servicecommands.CancelServiceCommand{ID: req.ID})
	if errors.Is(err, servicedomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	return nil, err
}

// ── device ───────────────────────────────────────────────────────────────────

func (s *Server) handleDeviceList(ctx context.Context, payload []byte) (any, error) {
	var req devicequeries.ListDevicesQuery
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &req)
	}
	if req.PageSize == 0 {
		req.PageSize = 25
	}
	return s.listDevices.Handle(ctx, req)
}

func (s *Server) handleDeviceGet(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	d, err := s.getDevice.Handle(ctx, devicequeries.GetDeviceQuery{ID: req.ID})
	if errors.Is(err, devicedomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	return d, err
}

func (s *Server) handleDeviceRegister(ctx context.Context, payload []byte) (any, error) {
	var req devicecommands.RegisterDeviceCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.registerDevice.Handle(ctx, req)
}

func (s *Server) handleDeviceDeploy(ctx context.Context, payload []byte) (any, error) {
	var req devicecommands.DeployDeviceCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	d, err := s.deployDevice.Handle(ctx, req)
	if errors.Is(err, devicedomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	if errors.Is(err, devicedomain.ErrInvalidTransition) {
		return nil, errors.New("invalid status transition")
	}
	return d, err
}

func (s *Server) handleDeviceReturn(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	err := s.returnDevice.Handle(ctx, devicecommands.ReturnDeviceCommand{ID: req.ID})
	if errors.Is(err, devicedomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	if errors.Is(err, devicedomain.ErrInvalidTransition) {
		return nil, errors.New("invalid status transition")
	}
	return nil, err
}

func (s *Server) handleDeviceDecommission(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	err := s.decommissionDevice.Handle(ctx, devicecommands.DecommissionDeviceCommand{ID: req.ID})
	if errors.Is(err, devicedomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	if errors.Is(err, devicedomain.ErrInvalidTransition) {
		return nil, errors.New("invalid status transition")
	}
	return nil, err
}

func (s *Server) handleDeviceUpdate(ctx context.Context, payload []byte) (any, error) {
	var req devicecommands.UpdateDeviceCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	d, err := s.updateDevice.Handle(ctx, req)
	if errors.Is(err, devicedomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	return d, err
}

// ── cron ─────────────────────────────────────────────────────────────────────

func (s *Server) handleCronList(ctx context.Context, _ []byte) (any, error) {
	return s.listJobs.Handle(ctx)
}

func (s *Server) handleCronGet(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	j, err := s.getJob.Handle(ctx, req.ID)
	if errors.Is(err, crondomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	return j, err
}

func (s *Server) handleCronAdd(ctx context.Context, payload []byte) (any, error) {
	var req croncommands.AddJobCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.addJob.Handle(ctx, req)
}

func (s *Server) handleCronPause(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	err := s.pauseJob.Handle(ctx, croncommands.PauseJobCommand{ID: req.ID})
	if errors.Is(err, crondomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	if errors.Is(err, crondomain.ErrInvalidTransition) {
		return nil, errors.New("invalid status transition")
	}
	return nil, err
}

func (s *Server) handleCronResume(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	err := s.resumeJob.Handle(ctx, croncommands.ResumeJobCommand{ID: req.ID})
	if errors.Is(err, crondomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	if errors.Is(err, crondomain.ErrInvalidTransition) {
		return nil, errors.New("invalid status transition")
	}
	return nil, err
}

func (s *Server) handleCronDelete(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	err := s.deleteJob.Handle(ctx, croncommands.DeleteJobCommand{ID: req.ID})
	if errors.Is(err, crondomain.ErrNotFound) {
		return nil, errors.New("not found")
	}
	return nil, err
}

// ── invoice ──────────────────────────────────────────────────────────────────

func (s *Server) handleInvoiceList(ctx context.Context, payload []byte) (any, error) {
	var req invoicequeries.ListAllInvoicesQuery
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &req)
	}
	return s.listAllInvoices.Handle(ctx, req)
}

func (s *Server) handleInvoiceGet(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.getInvoice.Handle(ctx, req.ID)
}

func (s *Server) handleInvoiceListForCustomer(ctx context.Context, payload []byte) (any, error) {
	var req invoicequeries.ListInvoicesForCustomerQuery
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.listInvoicesForCust.Handle(ctx, req)
}

func (s *Server) handleInvoiceCreate(ctx context.Context, payload []byte) (any, error) {
	var req invoicecommands.CreateInvoiceCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.createInvoice.Handle(ctx, req)
}

func (s *Server) handleInvoiceFinalize(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	inv, err := s.finalizeInvoice.Handle(ctx, invoicecommands.FinalizeInvoiceCommand{InvoiceID: req.ID})
	if errors.Is(err, invoicedomain.ErrInvalidTransition) {
		return nil, errors.New("invalid status transition")
	}
	return inv, err
}

func (s *Server) handleInvoiceMarkPaid(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	inv, err := s.markPaidInvoice.Handle(ctx, invoicecommands.MarkPaidCommand{InvoiceID: req.ID})
	if errors.Is(err, invoicedomain.ErrInvalidTransition) {
		return nil, errors.New("invalid status transition")
	}
	return inv, err
}

func (s *Server) handleInvoiceVoid(ctx context.Context, payload []byte) (any, error) {
	var req struct{ ID string `json:"id"` }
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	inv, err := s.voidInvoice.Handle(ctx, invoicecommands.VoidInvoiceCommand{InvoiceID: req.ID})
	if errors.Is(err, invoicedomain.ErrInvalidTransition) {
		return nil, errors.New("invalid status transition")
	}
	return inv, err
}

func (s *Server) handleInvoiceGenerate(ctx context.Context, payload []byte) (any, error) {
	var req invoicecommands.GenerateFromSubscriptionsCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.generateInvoice.Handle(ctx, req)
}

func (s *Server) handleInvoiceAddLine(ctx context.Context, payload []byte) (any, error) {
	var req invoicecommands.AddLineItemCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.addInvoiceLine.Handle(ctx, req)
}

func (s *Server) handleInvoiceUpdateLine(ctx context.Context, payload []byte) (any, error) {
	var req invoicecommands.UpdateLineItemCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.updateInvoiceLine.Handle(ctx, req)
}

func (s *Server) handleInvoiceRemoveLine(ctx context.Context, payload []byte) (any, error) {
	var req invoicecommands.RemoveLineItemCommand
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return s.removeInvoiceLine.Handle(ctx, req)
}
