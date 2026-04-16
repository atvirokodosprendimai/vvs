package persistence

import (
	"context"
	"strings"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/email/domain"
	"gorm.io/gorm"
)

// --- mapping helpers ---

func toAccountModel(a *domain.EmailAccount) accountModel {
	return accountModel{
		ID:          a.ID,
		Name:        a.Name,
		Host:        a.Host,
		Port:        a.Port,
		Username:    a.Username,
		PasswordEnc: a.PasswordEnc,
		TLS:         a.TLS,
		Folder:      a.Folder,
		Status:      a.Status,
		LastError:   a.LastError,
		LastSyncAt:  a.LastSyncAt,
		LastUID:     a.LastUID,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}

func (m *accountModel) toDomain() *domain.EmailAccount {
	return &domain.EmailAccount{
		ID:          m.ID,
		Name:        m.Name,
		Host:        m.Host,
		Port:        m.Port,
		Username:    m.Username,
		PasswordEnc: m.PasswordEnc,
		TLS:         m.TLS,
		Folder:      m.Folder,
		Status:      m.Status,
		LastError:   m.LastError,
		LastSyncAt:  m.LastSyncAt,
		LastUID:     m.LastUID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func toThreadModel(t *domain.EmailThread) threadModel {
	return threadModel{
		ID:                   t.ID,
		AccountID:            t.AccountID,
		Subject:              t.Subject,
		ParticipantAddresses: t.ParticipantAddresses,
		CustomerID:           t.CustomerID,
		MessageCount:         t.MessageCount,
		LastMessageAt:        t.LastMessageAt,
		CreatedAt:            t.CreatedAt,
	}
}

func (m *threadModel) toDomain() *domain.EmailThread {
	return &domain.EmailThread{
		ID:                   m.ID,
		AccountID:            m.AccountID,
		Subject:              m.Subject,
		ParticipantAddresses: m.ParticipantAddresses,
		CustomerID:           m.CustomerID,
		MessageCount:         m.MessageCount,
		LastMessageAt:        m.LastMessageAt,
		CreatedAt:            m.CreatedAt,
	}
}

func toMessageModel(m *domain.EmailMessage) messageModel {
	return messageModel{
		ID:         m.ID,
		AccountID:  m.AccountID,
		ThreadID:   m.ThreadID,
		UID:        m.UID,
		Folder:     m.Folder,
		MessageID:  m.MessageID,
		References: m.References,
		InReplyTo:  m.InReplyTo,
		Subject:    m.Subject,
		FromAddr:   m.FromAddr,
		FromName:   m.FromName,
		ToAddrs:    m.ToAddrs,
		TextBody:   m.TextBody,
		HTMLBody:   m.HTMLBody,
		ReceivedAt: m.ReceivedAt,
		FetchedAt:  m.FetchedAt,
	}
}

func (m *messageModel) toDomain() *domain.EmailMessage {
	return &domain.EmailMessage{
		ID:         m.ID,
		AccountID:  m.AccountID,
		ThreadID:   m.ThreadID,
		UID:        m.UID,
		Folder:     m.Folder,
		MessageID:  m.MessageID,
		References: m.References,
		InReplyTo:  m.InReplyTo,
		Subject:    m.Subject,
		FromAddr:   m.FromAddr,
		FromName:   m.FromName,
		ToAddrs:    m.ToAddrs,
		TextBody:   m.TextBody,
		HTMLBody:   m.HTMLBody,
		ReceivedAt: m.ReceivedAt,
		FetchedAt:  m.FetchedAt,
	}
}

func toAttachmentModel(a *domain.EmailAttachment) attachmentModel {
	return attachmentModel{
		ID:        a.ID,
		MessageID: a.MessageID,
		Filename:  a.Filename,
		MIMEType:  a.MIMEType,
		Size:      a.Size,
		Data:      a.Data,
		CreatedAt: a.CreatedAt,
	}
}

func (m *attachmentModel) toDomain() *domain.EmailAttachment {
	return &domain.EmailAttachment{
		ID:        m.ID,
		MessageID: m.MessageID,
		Filename:  m.Filename,
		MIMEType:  m.MIMEType,
		Size:      m.Size,
		Data:      m.Data,
		CreatedAt: m.CreatedAt,
	}
}

func toTagModel(t *domain.EmailTag) tagModel {
	sys := 0
	if t.System {
		sys = 1
	}
	return tagModel{
		ID:        t.ID,
		AccountID: t.AccountID,
		Name:      t.Name,
		Color:     t.Color,
		System:    sys,
		CreatedAt: t.CreatedAt,
	}
}

func (m *tagModel) toDomain() *domain.EmailTag {
	return &domain.EmailTag{
		ID:        m.ID,
		AccountID: m.AccountID,
		Name:      m.Name,
		Color:     m.Color,
		System:    m.System != 0,
		CreatedAt: m.CreatedAt,
	}
}

// --- GormEmailAccountRepository ---

type GormEmailAccountRepository struct{ db *gormsqlite.DB }

func NewGormEmailAccountRepository(db *gormsqlite.DB) *GormEmailAccountRepository {
	return &GormEmailAccountRepository{db: db}
}

func (r *GormEmailAccountRepository) Save(ctx context.Context, a *domain.EmailAccount) error {
	m := toAccountModel(a)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}

func (r *GormEmailAccountRepository) FindByID(ctx context.Context, id string) (*domain.EmailAccount, error) {
	var m accountModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormEmailAccountRepository) ListActive(ctx context.Context) ([]*domain.EmailAccount, error) {
	return r.listWhere(ctx, "status = ?", domain.AccountStatusActive)
}

func (r *GormEmailAccountRepository) List(ctx context.Context) ([]*domain.EmailAccount, error) {
	return r.listWhere(ctx, "1=1")
}

func (r *GormEmailAccountRepository) listWhere(ctx context.Context, cond string, args ...any) ([]*domain.EmailAccount, error) {
	var models []accountModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where(cond, args...).Order("created_at ASC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EmailAccount, len(models))
	for i := range models {
		out[i] = models[i].toDomain()
	}
	return out, nil
}

func (r *GormEmailAccountRepository) Delete(ctx context.Context, id string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).Delete(&accountModel{}).Error
	})
}

// --- GormEmailThreadRepository ---

type GormEmailThreadRepository struct{ db *gormsqlite.DB }

func NewGormEmailThreadRepository(db *gormsqlite.DB) *GormEmailThreadRepository {
	return &GormEmailThreadRepository{db: db}
}

func (r *GormEmailThreadRepository) Save(ctx context.Context, t *domain.EmailThread) error {
	m := toThreadModel(t)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}

func (r *GormEmailThreadRepository) FindByID(ctx context.Context, id string) (*domain.EmailThread, error) {
	var m threadModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrThreadNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

// FindByMessageID finds the thread containing a message with the given RFC 2822 Message-ID.
func (r *GormEmailThreadRepository) FindByMessageID(ctx context.Context, accountID, messageID string) (*domain.EmailThread, error) {
	var m threadModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT t.* FROM email_threads t
			JOIN email_messages msg ON msg.thread_id = t.id
			WHERE t.account_id = ? AND msg.message_id = ?
			LIMIT 1
		`, accountID, messageID).Scan(&m).Error
	})
	if err != nil {
		return nil, err
	}
	if m.ID == "" {
		return nil, domain.ErrThreadNotFound
	}
	return m.toDomain(), nil
}

// FindBySubject finds a thread with matching normalized subject for the account.
func (r *GormEmailThreadRepository) FindBySubject(ctx context.Context, accountID, normalizedSubject string) (*domain.EmailThread, error) {
	var m threadModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("account_id = ? AND subject = ?", accountID, normalizedSubject).
			Order("last_message_at DESC").
			First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrThreadNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormEmailThreadRepository) ListForAccount(ctx context.Context, accountID string) ([]*domain.EmailThread, error) {
	var models []threadModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("account_id = ?", accountID).Order("last_message_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EmailThread, len(models))
	for i := range models {
		out[i] = models[i].toDomain()
	}
	return out, nil
}

func (r *GormEmailThreadRepository) ListForCustomer(ctx context.Context, customerID string) ([]*domain.EmailThread, error) {
	var models []threadModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("customer_id = ?", customerID).Order("last_message_at DESC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EmailThread, len(models))
	for i := range models {
		out[i] = models[i].toDomain()
	}
	return out, nil
}

// --- GormEmailMessageRepository ---

type GormEmailMessageRepository struct{ db *gormsqlite.DB }

func NewGormEmailMessageRepository(db *gormsqlite.DB) *GormEmailMessageRepository {
	return &GormEmailMessageRepository{db: db}
}

func (r *GormEmailMessageRepository) Save(ctx context.Context, m *domain.EmailMessage) error {
	model := toMessageModel(m)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&model).Error
	})
}

func (r *GormEmailMessageRepository) FindByUID(ctx context.Context, accountID string, uid uint32) (*domain.EmailMessage, error) {
	var m messageModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("account_id = ? AND uid = ?", accountID, uid).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormEmailMessageRepository) FindByMessageID(ctx context.Context, accountID, messageID string) (*domain.EmailMessage, error) {
	var m messageModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("account_id = ? AND message_id = ?", accountID, messageID).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormEmailMessageRepository) ListForThread(ctx context.Context, threadID string) ([]*domain.EmailMessage, error) {
	var models []messageModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("thread_id = ?", threadID).Order("received_at ASC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EmailMessage, len(models))
	for i := range models {
		out[i] = models[i].toDomain()
	}
	return out, nil
}

// --- GormEmailAttachmentRepository ---

type GormEmailAttachmentRepository struct{ db *gormsqlite.DB }

func NewGormEmailAttachmentRepository(db *gormsqlite.DB) *GormEmailAttachmentRepository {
	return &GormEmailAttachmentRepository{db: db}
}

func (r *GormEmailAttachmentRepository) Save(ctx context.Context, a *domain.EmailAttachment) error {
	m := toAttachmentModel(a)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}

func (r *GormEmailAttachmentRepository) FindByID(ctx context.Context, id string) (*domain.EmailAttachment, error) {
	var m attachmentModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrMessageNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormEmailAttachmentRepository) ListForMessage(ctx context.Context, messageID string) ([]*domain.EmailAttachment, error) {
	var models []attachmentModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("message_id = ?", messageID).Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EmailAttachment, len(models))
	for i := range models {
		out[i] = models[i].toDomain()
	}
	return out, nil
}

// --- GormEmailTagRepository ---

type GormEmailTagRepository struct{ db *gormsqlite.DB }

func NewGormEmailTagRepository(db *gormsqlite.DB) *GormEmailTagRepository {
	return &GormEmailTagRepository{db: db}
}

func (r *GormEmailTagRepository) Save(ctx context.Context, t *domain.EmailTag) error {
	m := toTagModel(t)
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Save(&m).Error
	})
}

func (r *GormEmailTagRepository) FindByID(ctx context.Context, id string) (*domain.EmailTag, error) {
	var m tagModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("id = ?", id).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrTagNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

func (r *GormEmailTagRepository) ListAll(ctx context.Context) ([]*domain.EmailTag, error) {
	var models []tagModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Order("system DESC, name ASC").Find(&models).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EmailTag, len(models))
	for i := range models {
		out[i] = models[i].toDomain()
	}
	return out, nil
}

func (r *GormEmailTagRepository) ListForThread(ctx context.Context, threadID string) ([]*domain.EmailTag, error) {
	var models []tagModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT t.* FROM email_tags t
			JOIN email_thread_tags tt ON tt.tag_id = t.id
			WHERE tt.thread_id = ?
			ORDER BY t.system DESC, t.name ASC
		`, threadID).Scan(&models).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.EmailTag, len(models))
	for i := range models {
		out[i] = models[i].toDomain()
	}
	return out, nil
}

func (r *GormEmailTagRepository) ApplyToThread(ctx context.Context, tt domain.EmailThreadTag) error {
	m := threadTagModel{ThreadID: tt.ThreadID, TagID: tt.TagID}
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Exec(
			`INSERT OR IGNORE INTO email_thread_tags (thread_id, tag_id) VALUES (?, ?)`,
			m.ThreadID, m.TagID,
		).Error
	})
}

func (r *GormEmailTagRepository) RemoveFromThread(ctx context.Context, threadID, tagID string) error {
	return r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("thread_id = ? AND tag_id = ?", threadID, tagID).Delete(&threadTagModel{}).Error
	})
}

// FindSystemTag returns the seeded system tag by name.
func (r *GormEmailTagRepository) FindSystemTag(ctx context.Context, name string) (*domain.EmailTag, error) {
	var m tagModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("system = 1 AND name = ?", strings.ToLower(name)).First(&m).Error
	})
	if err == gorm.ErrRecordNotFound {
		return nil, domain.ErrTagNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDomain(), nil
}

// UpdateThreadStats recalculates message_count and last_message_at for a thread.
func UpdateThreadStats(ctx context.Context, db *gormsqlite.DB, threadID string) error {
	return db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Exec(`
			UPDATE email_threads
			SET message_count = (SELECT COUNT(*) FROM email_messages WHERE thread_id = ?),
			    last_message_at = COALESCE(
			        (SELECT MAX(received_at) FROM email_messages WHERE thread_id = ?),
			        last_message_at
			    )
			WHERE id = ?
		`, threadID, threadID, threadID).Error
	})
}

// AddParticipant appends an address to thread.participant_addresses if not already present.
func AddParticipant(ctx context.Context, db *gormsqlite.DB, threadID, addr string) error {
	if addr == "" {
		return nil
	}
	return db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		var current string
		if err := tx.Raw(
			`SELECT participant_addresses FROM email_threads WHERE id = ?`, threadID,
		).Scan(&current).Error; err != nil {
			return err
		}
		if strings.Contains(current, addr) {
			return nil
		}
		updated := addr
		if current != "" {
			updated = current + "," + addr
		}
		return tx.Exec(
			`UPDATE email_threads SET participant_addresses = ? WHERE id = ?`,
			updated, threadID,
		).Error
	})
}

// NowUTC is a testable time source.
var NowUTC = func() time.Time { return time.Now().UTC() }
