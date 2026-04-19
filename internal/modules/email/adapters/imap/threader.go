package imap

import (
	"context"
	"errors"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
)

// ThreadRepositoryReader is the subset of EmailThreadRepository needed by Assign.
type ThreadRepositoryReader interface {
	FindByMessageID(ctx context.Context, accountID, messageID string) (*domain.EmailThread, error)
	FindBySubject(ctx context.Context, accountID, normalizedSubject string) (*domain.EmailThread, error)
	Save(ctx context.Context, t *domain.EmailThread) error
}

// MessageRepositoryReader is the subset of EmailMessageRepository needed by Assign.
type MessageRepositoryReader interface {
	FindByMessageID(ctx context.Context, accountID, messageID string) (*domain.EmailMessage, error)
}

// Assign determines which thread the message belongs to and returns its ID.
// Creates a new thread if no match is found.
// Priority: References/In-Reply-To match → subject match → new thread.
func Assign(
	ctx context.Context,
	msg *domain.EmailMessage,
	threads ThreadRepositoryReader,
	messages MessageRepositoryReader,
	newID func() string,
) (threadID string, err error) {
	// 1. Check References and In-Reply-To for existing thread.
	refIDs := msg.ReferenceIDs()
	if msg.InReplyTo != "" {
		refIDs = append(refIDs, msg.InReplyTo)
	}
	for _, refMsgID := range refIDs {
		t, err := threads.FindByMessageID(ctx, msg.AccountID, refMsgID)
		if err == nil {
			return t.ID, nil
		}
		if !errors.Is(err, domain.ErrThreadNotFound) {
			return "", err
		}
		// Try finding the message itself and get its thread.
		m, merr := messages.FindByMessageID(ctx, msg.AccountID, refMsgID)
		if merr == nil {
			return m.ThreadID, nil
		}
	}

	// 2. Fallback: normalized subject match.
	normalized := domain.NormalizeSubject(msg.Subject)
	if normalized != "" {
		t, err := threads.FindBySubject(ctx, msg.AccountID, normalized)
		if err == nil {
			return t.ID, nil
		}
		if !errors.Is(err, domain.ErrThreadNotFound) {
			return "", err
		}
	}

	// 3. Create new thread.
	now := time.Now().UTC()
	t := &domain.EmailThread{
		ID:            newID(),
		AccountID:     msg.AccountID,
		Subject:       normalized,
		LastMessageAt: msg.ReceivedAt,
		CreatedAt:     now,
	}
	if err := threads.Save(ctx, t); err != nil {
		return "", err
	}
	return t.ID, nil
}
