package events

import "fmt"

// Subject is a typed NATS subject string.
// Static subjects use String(); dynamic subjects use Format(args...).
type Subject string

// String returns the raw subject string. Use for static subjects.
func (s Subject) String() string { return string(s) }

// Format replaces %s/%d placeholders (like fmt.Sprintf). Use for dynamic subjects.
func (s Subject) Format(args ...any) string { return fmt.Sprintf(string(s), args...) }

// ────────────────────────────────────────────────────────────────────
// Wildcard subscriptions
// ────────────────────────────────────────────────────────────────────

// Everything matches all ISP domain events.
// Used by: notification worker, customer CRM SSE (catch-all).
var Everything Subject = "isp.>"

// ────────────────────────────────────────────────────────────────────
// Customer
// ────────────────────────────────────────────────────────────────────

var CustomerAll Subject = "isp.customer.*" // wildcard for all customer events

// CustomerCreated — new customer added.
// Pub: customer/app/commands/create_customer.go
// Sub: customer list SSE, notification worker, ARP worker
var CustomerCreated Subject = "isp.customer.created"

// CustomerUpdated — customer fields changed.
// Pub: customer/app/commands/update_customer.go
var CustomerUpdated Subject = "isp.customer.updated"

// CustomerDeleted — customer removed.
// Pub: customer/app/commands/delete_customer.go
var CustomerDeleted Subject = "isp.customer.deleted"

// CustomerStatusChanged — status transition (qualified/converted/suspended/activated/churned).
// Pub: customer/app/commands/change_status.go — use Format(action)
// Example: CustomerStatusChanged.Format("activated")
var CustomerStatusChanged Subject = "isp.customer.%s"

// ────────────────────────────────────────────────────────────────────
// Contact
// ────────────────────────────────────────────────────────────────────

var ContactAll Subject = "isp.contact.*"

// Pub: contact/app/commands/add.go
var ContactAdded Subject = "isp.contact.added"

// Pub: contact/app/commands/update.go
var ContactUpdated Subject = "isp.contact.updated"

// Pub: contact/app/commands/delete.go
var ContactDeleted Subject = "isp.contact.deleted"

// ────────────────────────────────────────────────────────────────────
// Product
// ────────────────────────────────────────────────────────────────────

var ProductAll Subject = "isp.product.*"

// Pub: product/app/commands/create_product.go
var ProductCreated Subject = "isp.product.created"

// Pub: product/app/commands/update_product.go
var ProductUpdated Subject = "isp.product.updated"

// Pub: product/app/commands/delete_product.go
var ProductDeleted Subject = "isp.product.deleted"

// ────────────────────────────────────────────────────────────────────
// Service
// ────────────────────────────────────────────────────────────────────

var ServiceAll Subject = "isp.service.*"

// Pub: service/app/commands/assign.go
var ServiceAssigned Subject = "isp.service.assigned"

// Pub: service/app/commands/transitions.go
var ServiceSuspended Subject = "isp.service.suspended"

// Pub: service/app/commands/transitions.go
var ServiceReactivated Subject = "isp.service.reactivated"

// Pub: service/app/commands/transitions.go
var ServiceCancelled Subject = "isp.service.cancelled"

// ────────────────────────────────────────────────────────────────────
// Device
// ────────────────────────────────────────────────────────────────────

var DeviceAll Subject = "isp.device.*"

// Pub: device/app/commands/register.go
var DeviceRegistered Subject = "isp.device.registered"

// Pub: device/app/commands/deploy.go
var DeviceDeployed Subject = "isp.device.deployed"

// Pub: device/app/commands/update.go
var DeviceUpdated Subject = "isp.device.updated"

// Pub: device/app/commands/decommission.go
var DeviceDecommissioned Subject = "isp.device.decommissioned"

// Pub: device/app/commands/return.go
var DeviceReturned Subject = "isp.device.returned"

// ────────────────────────────────────────────────────────────────────
// Network
// ────────────────────────────────────────────────────────────────────

var NetworkRouterAll Subject = "isp.network.router.*"

// Pub: network/app/commands/create_router.go
var NetworkRouterCreated Subject = "isp.network.router.created"

// Pub: network/app/commands/update_router.go
var NetworkRouterUpdated Subject = "isp.network.router.updated"

// Pub: network/app/commands/delete_router.go
var NetworkRouterDeleted Subject = "isp.network.router.deleted"

// Pub: network/app/commands/sync_customer_arp.go
// Sub: ARP worker
var NetworkARPChanged Subject = "isp.network.arp_changed"

// Pub: customer/adapters/http/handlers.go (ARP enable/disable button)
// Sub: network/app/subscribers/arp_worker.go
var NetworkARPRequested Subject = "isp.network.arp_requested"

// ────────────────────────────────────────────────────────────────────
// Deal
// ────────────────────────────────────────────────────────────────────

var DealAll Subject = "isp.deal.*"

// Pub: deal/app/commands/add.go
var DealAdded Subject = "isp.deal.added"

// Pub: deal/app/commands/update.go
var DealUpdated Subject = "isp.deal.updated"

// Pub: deal/app/commands/delete.go
var DealDeleted Subject = "isp.deal.deleted"

// Pub: deal/app/commands/advance.go
var DealAdvanced Subject = "isp.deal.advanced"

// ────────────────────────────────────────────────────────────────────
// Ticket
// ────────────────────────────────────────────────────────────────────

var TicketAll Subject = "isp.ticket.*"

// Pub: ticket/app/commands/open.go
var TicketOpened Subject = "isp.ticket.opened"

// Pub: ticket/app/commands/update.go
var TicketUpdated Subject = "isp.ticket.updated"

// Pub: ticket/app/commands/delete.go
var TicketDeleted Subject = "isp.ticket.deleted"

// Pub: ticket/app/commands/change_status.go
var TicketStatusChanged Subject = "isp.ticket.status_changed"

// Pub: ticket/app/commands/add_comment.go
// Sub: ticket comment SSE (exact match)
var TicketCommentAdded Subject = "isp.ticket.comment_added"

// ────────────────────────────────────────────────────────────────────
// Task
// ────────────────────────────────────────────────────────────────────

var TaskAll Subject = "isp.task.*"

// Pub: task/app/commands/create.go
var TaskCreated Subject = "isp.task.created"

// Pub: task/app/commands/update.go
var TaskUpdated Subject = "isp.task.updated"

// Pub: task/app/commands/delete.go
var TaskDeleted Subject = "isp.task.deleted"

// Pub: task/app/commands/change_status.go
var TaskStatusChanged Subject = "isp.task.status_changed"

// ────────────────────────────────────────────────────────────────────
// Email
// ────────────────────────────────────────────────────────────────────

var EmailAll Subject = "isp.email.*"

// Pub: email/app/commands/configure_account.go
var EmailAccountConfigured Subject = "isp.email.account_configured"

// Pub: email/app/commands/manage_account.go
var EmailAccountPaused Subject = "isp.email.account_paused"

// Pub: email/app/commands/manage_account.go
var EmailAccountResumed Subject = "isp.email.account_resumed"

// Pub: email/app/commands/delete_account.go
var EmailAccountDeleted Subject = "isp.email.account_deleted"

// Pub: email/app/commands/send_reply.go, email/app/commands/compose.go
var EmailThreadUpdated Subject = "isp.email.thread_updated"

// Pub: email/app/commands/tag_thread.go
var EmailThreadTagged Subject = "isp.email.thread_tagged"

// Pub: email/app/commands/tag_thread.go
var EmailThreadUntagged Subject = "isp.email.thread_untagged"

// Pub: email/app/commands/mark_read.go
var EmailRead Subject = "isp.email.read"

// Pub: email/app/commands/link_customer.go, email/worker/sync_worker.go, email/adapters/http/handlers.go
var EmailCustomerLinked Subject = "isp.email.customer_linked"

// Pub: email/adapters/imap/fetcher.go
var EmailSynced Subject = "isp.email.synced"

// Pub: email/adapters/http/handlers.go — use Format(accountID)
// Sub: email/worker/sync_worker.go (isp.email.sync_requested.*)
var EmailSyncRequested Subject = "isp.email.sync_requested.%s"

// EmailSyncRequestedAll — wildcard for per-account sync triggers.
var EmailSyncRequestedAll Subject = "isp.email.sync_requested.*"

// ────────────────────────────────────────────────────────────────────
// Chat
// ────────────────────────────────────────────────────────────────────

var ChatAll Subject = "isp.chat.>"

// Pub: infrastructure/http/chat.go — use Format(threadID)
// Sub: chat page SSE (per thread), global SSE (isp.chat.message.general)
var ChatMessage Subject = "isp.chat.message.%s"

// ChatMessageGeneral — hardcoded general channel subscription.
var ChatMessageGeneral Subject = "isp.chat.message.general"

// Pub: infrastructure/http/chat.go — use Format(threadID)
// Sub: chat page SSE (isp.chat.> wildcard)
var ChatRead Subject = "isp.chat.read.%s"

// Pub: infrastructure/http/chat.go
var ChatThreadCreated Subject = "isp.chat.thread.created"

// ────────────────────────────────────────────────────────────────────
// Invoice
// ────────────────────────────────────────────────────────────────────

var InvoiceAll Subject = "isp.invoice.*"

// InvoiceCreated is published when a new invoice is created.
var InvoiceCreated Subject = "isp.invoice.created"

// InvoiceUpdated is published when invoice details are modified.
var InvoiceUpdated Subject = "isp.invoice.updated"

// InvoiceFinalized is published when an invoice is finalized (locked).
var InvoiceFinalized Subject = "isp.invoice.finalized"

// InvoicePaid is published when an invoice is marked as paid.
var InvoicePaid Subject = "isp.invoice.paid"

// InvoiceVoided is published when an invoice is voided/cancelled.
var InvoiceVoided Subject = "isp.invoice.voided"

// ────────────────────────────────────────────────────────────────────
// Proxmox
// ────────────────────────────────────────────────────────────────────

var ProxmoxVMAll Subject = "isp.proxmox.vm.*"

// Pub: proxmox/app/commands/create_vm.go
var ProxmoxVMCreated Subject = "isp.proxmox.vm.created"

// Pub: proxmox/app/commands/suspend_vm.go
var ProxmoxVMSuspended Subject = "isp.proxmox.vm.suspended"

// Pub: proxmox/app/commands/resume_vm.go
var ProxmoxVMResumed Subject = "isp.proxmox.vm.resumed"

// Pub: proxmox/app/commands/restart_vm.go
var ProxmoxVMRestarted Subject = "isp.proxmox.vm.restarted"

// Pub: proxmox/app/commands/delete_vm.go
var ProxmoxVMDeleted Subject = "isp.proxmox.vm.deleted"

// Pub: proxmox/app/commands/create_vm.go, suspend_vm.go, resume_vm.go, restart_vm.go
// Carries updated VM status for SSE refresh.
var ProxmoxVMStatusChanged Subject = "isp.proxmox.vm.status_changed"

var ProxmoxNodeAll Subject = "isp.proxmox.node.*"

// Pub: proxmox/app/commands/create_node.go
var ProxmoxNodeCreated Subject = "isp.proxmox.node.created"

// Pub: proxmox/app/commands/update_node.go
var ProxmoxNodeUpdated Subject = "isp.proxmox.node.updated"

// Pub: proxmox/app/commands/delete_node.go
var ProxmoxNodeDeleted Subject = "isp.proxmox.node.deleted"

// ────────────────────────────────────────────────────────────────────
// Notifications
// ────────────────────────────────────────────────────────────────────

// Pub: infrastructure/notifications/worker.go
// Sub: notification SSE, global SSE
var Notifications Subject = "isp.notifications"
