package commands

import (
	"context"

	"github.com/vvs/isp/internal/modules/email/domain"
)

type LinkCustomerCommand struct {
	ThreadID   string
	CustomerID string // empty = unlink
}

type LinkCustomerHandler struct {
	threads domain.EmailThreadRepository
}

func NewLinkCustomerHandler(threads domain.EmailThreadRepository) *LinkCustomerHandler {
	return &LinkCustomerHandler{threads: threads}
}

func (h *LinkCustomerHandler) Handle(ctx context.Context, cmd LinkCustomerCommand) error {
	t, err := h.threads.FindByID(ctx, cmd.ThreadID)
	if err != nil {
		return err
	}
	t.CustomerID = cmd.CustomerID
	return h.threads.Save(ctx, t)
}
