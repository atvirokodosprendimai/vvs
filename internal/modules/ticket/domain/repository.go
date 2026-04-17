package domain

import "context"

// TicketRepository is the port for ticket persistence.
type TicketRepository interface {
	Save(ctx context.Context, t *Ticket) error
	FindByID(ctx context.Context, id string) (*Ticket, error)
	ListAll(ctx context.Context) ([]*Ticket, error)
	ListForCustomer(ctx context.Context, customerID string) ([]*Ticket, error)
	Delete(ctx context.Context, id string) error
	SaveComment(ctx context.Context, c *TicketComment) error
	ListComments(ctx context.Context, ticketID string) ([]*TicketComment, error)
}
