package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/email/domain"
)

type GetThreadHandler struct {
	threads     domain.EmailThreadRepository
	messages    domain.EmailMessageRepository
	attachments domain.EmailAttachmentRepository
	tags        domain.EmailTagRepository
}

func NewGetThreadHandler(
	threads domain.EmailThreadRepository,
	messages domain.EmailMessageRepository,
	attachments domain.EmailAttachmentRepository,
	tags domain.EmailTagRepository,
) *GetThreadHandler {
	return &GetThreadHandler{
		threads:     threads,
		messages:    messages,
		attachments: attachments,
		tags:        tags,
	}
}

func (h *GetThreadHandler) Handle(ctx context.Context, threadID string) (*ThreadDetailReadModel, error) {
	t, err := h.threads.FindByID(ctx, threadID)
	if err != nil {
		return nil, err
	}

	tags, _ := h.tags.ListForThread(ctx, threadID)
	base := threadToReadModel(t, tags)

	msgs, err := h.messages.ListForThread(ctx, threadID)
	if err != nil {
		return nil, err
	}

	msgModels := make([]MessageReadModel, len(msgs))
	for i, m := range msgs {
		atts, _ := h.attachments.ListForMessage(ctx, m.ID)
		attModels := make([]AttachmentReadModel, len(atts))
		for j, a := range atts {
			attModels[j] = AttachmentReadModel{
				ID:       a.ID,
				Filename: a.Filename,
				MIMEType: a.MIMEType,
				Size:     a.Size,
			}
		}
		msgModels[i] = MessageReadModel{
			ID:          m.ID,
			ThreadID:    m.ThreadID,
			MessageID:   m.MessageID,
			Subject:     m.Subject,
			FromAddr:    m.FromAddr,
			FromName:    m.FromName,
			ToAddrs:     m.ToAddrs,
			TextBody:    m.TextBody,
			HTMLBody:    m.HTMLBody,
			ReceivedAt:  m.ReceivedAt,
			Attachments: attModels,
		}
	}

	return &ThreadDetailReadModel{
		ThreadReadModel: base,
		Messages:        msgModels,
	}, nil
}
