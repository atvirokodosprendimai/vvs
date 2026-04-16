package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/email/domain"
)

type SearchAttachmentsQuery struct {
	AccountID string
	Query     string
}

type SearchAttachmentsHandler struct {
	attachments domain.EmailAttachmentRepository
}

func NewSearchAttachmentsHandler(attachments domain.EmailAttachmentRepository) *SearchAttachmentsHandler {
	return &SearchAttachmentsHandler{attachments: attachments}
}

func (h *SearchAttachmentsHandler) Handle(ctx context.Context, q SearchAttachmentsQuery) ([]AttachmentSearchRow, error) {
	if q.Query == "" {
		return nil, nil
	}
	rows, err := h.attachments.SearchByFilename(ctx, q.AccountID, q.Query)
	if err != nil {
		return nil, err
	}
	out := make([]AttachmentSearchRow, len(rows))
	for i, r := range rows {
		out[i] = AttachmentSearchRow{
			ID:            r.ID,
			Filename:      r.Filename,
			MIMEType:      r.MIMEType,
			Size:          r.Size,
			ThreadID:      r.ThreadID,
			ThreadSubject: r.ThreadSubject,
			FromAddr:      r.FromAddr,
			ReceivedAt:    r.ReceivedAt,
		}
	}
	return out, nil
}
