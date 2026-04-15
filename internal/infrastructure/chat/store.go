// Package chat provides persistent chat storage for threads, DMs, and channels.
package chat

import (
	"context"
	"errors"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
)

// ErrNotFound is returned when a thread or member is not found.
var ErrNotFound = errors.New("chat: not found")

// Thread represents a DM or named channel.
type Thread struct {
	ID        string
	Type      string // "direct" | "channel"
	Name      string // empty for direct threads
	IsPrivate bool
	CreatedBy string
	CreatedAt time.Time
}

// ThreadSummary is a Thread with read-model extras for the thread list UI.
type ThreadSummary struct {
	Thread
	LastMessage string
	LastAt      time.Time
	UnreadCount int
	Members     []string // userIDs — populated for direct threads to derive display name
}

// Message is the canonical chat message used by store and HTTP layer.
type Message struct {
	ID        string    `json:"id"`
	ThreadID  string    `json:"thread_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// --- GORM models ---

type threadModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Type      string    `gorm:"column:type"`
	Name      string    `gorm:"column:name"`
	IsPrivate int       `gorm:"column:is_private"`
	CreatedBy string    `gorm:"column:created_by"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (threadModel) TableName() string { return "chat_threads" }

type memberModel struct {
	ThreadID string    `gorm:"column:thread_id"`
	UserID   string    `gorm:"column:user_id"`
	JoinedAt time.Time `gorm:"column:joined_at"`
}

func (memberModel) TableName() string { return "chat_thread_members" }

type msgModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	ThreadID  string    `gorm:"column:thread_id"`
	UserID    string    `gorm:"column:user_id"`
	Username  string    `gorm:"column:username"`
	Body      string    `gorm:"column:body"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (msgModel) TableName() string { return "chat_messages" }

type readModel struct {
	ThreadID   string    `gorm:"column:thread_id"`
	UserID     string    `gorm:"column:user_id"`
	LastReadAt time.Time `gorm:"column:last_read_at"`
}

func (readModel) TableName() string { return "chat_thread_reads" }

// Store handles all chat DB operations.
type Store struct {
	db *gormsqlite.DB
}

// NewStore creates a Store.
func NewStore(db *gormsqlite.DB) *Store { return &Store{db: db} }

// CreateThread inserts a new thread.
func (s *Store) CreateThread(ctx context.Context, t Thread) error {
	return s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Create(&threadModel{
			ID:        t.ID,
			Type:      t.Type,
			Name:      t.Name,
			IsPrivate: boolToInt(t.IsPrivate),
			CreatedBy: t.CreatedBy,
			CreatedAt: t.CreatedAt,
		}).Error
	})
}

// ThreadExists returns true if a thread with the given id exists.
func (s *Store) ThreadExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Model(&threadModel{}).Where("id = ?", id).Count(&count).Error
	})
	return count > 0, err
}

// AddMember adds a user to a thread. Idempotent.
func (s *Store) AddMember(ctx context.Context, threadID, userID string) error {
	return s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Exec(
			`INSERT OR IGNORE INTO chat_thread_members (thread_id, user_id, joined_at) VALUES (?, ?, ?)`,
			threadID, userID, time.Now().UTC(),
		).Error
	})
}

// IsMember returns true if userID is a member of threadID.
func (s *Store) IsMember(ctx context.Context, threadID, userID string) (bool, error) {
	var count int64
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Model(&memberModel{}).
			Where("thread_id = ? AND user_id = ?", threadID, userID).
			Count(&count).Error
	})
	return count > 0, err
}

// FindDirectThread returns the existing DM thread between two users, or ErrNotFound.
func (s *Store) FindDirectThread(ctx context.Context, userA, userB string) (Thread, error) {
	var row threadModel
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT t.* FROM chat_threads t
			JOIN chat_thread_members ma ON ma.thread_id = t.id AND ma.user_id = ?
			JOIN chat_thread_members mb ON mb.thread_id = t.id AND mb.user_id = ?
			WHERE t.type = 'direct'
			LIMIT 1
		`, userA, userB).Scan(&row).Error
	})
	if err != nil {
		return Thread{}, err
	}
	if row.ID == "" {
		return Thread{}, ErrNotFound
	}
	return toThread(row), nil
}

// ListThreadsForUser returns all threads the user is a member of with read-model extras.
func (s *Store) ListThreadsForUser(ctx context.Context, userID string) ([]ThreadSummary, error) {
	type summaryRow struct {
		ID          string    `gorm:"column:id"`
		Type        string    `gorm:"column:type"`
		Name        string    `gorm:"column:name"`
		IsPrivate   int       `gorm:"column:is_private"`
		CreatedBy   string    `gorm:"column:created_by"`
		CreatedAt   time.Time `gorm:"column:created_at"`
		LastMessage string    `gorm:"column:last_message"`
		LastAt      time.Time `gorm:"column:last_at"`
		UnreadCount int       `gorm:"column:unread_count"`
	}

	var rows []summaryRow
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT
				t.id, t.type, t.name, t.is_private, t.created_by, t.created_at,
				COALESCE(last_msg.body, '') AS last_message,
				COALESCE(last_msg.created_at, t.created_at) AS last_at,
				COALESCE(unread.cnt, 0) AS unread_count
			FROM chat_threads t
			JOIN chat_thread_members m ON m.thread_id = t.id AND m.user_id = ?
			LEFT JOIN (
				SELECT thread_id, body, created_at,
					ROW_NUMBER() OVER (PARTITION BY thread_id ORDER BY created_at DESC) AS rn
				FROM chat_messages
			) last_msg ON last_msg.thread_id = t.id AND last_msg.rn = 1
			LEFT JOIN (
				SELECT cm.thread_id, COUNT(*) AS cnt
				FROM chat_messages cm
				LEFT JOIN chat_thread_reads r ON r.thread_id = cm.thread_id AND r.user_id = ?
				WHERE r.last_read_at IS NULL OR cm.created_at > r.last_read_at
				GROUP BY cm.thread_id
			) unread ON unread.thread_id = t.id
			ORDER BY last_at DESC
		`, userID, userID).Scan(&rows).Error
	})
	if err != nil {
		return nil, err
	}

	// Fetch member userIDs for direct threads (for display name derivation)
	summaries := make([]ThreadSummary, len(rows))
	for i, r := range rows {
		summaries[i] = ThreadSummary{
			Thread: Thread{
				ID:        r.ID,
				Type:      r.Type,
				Name:      r.Name,
				IsPrivate: r.IsPrivate != 0,
				CreatedBy: r.CreatedBy,
				CreatedAt: r.CreatedAt,
			},
			LastMessage: r.LastMessage,
			LastAt:      r.LastAt,
			UnreadCount: r.UnreadCount,
		}
		if r.Type == "direct" {
			members, _ := s.threadMemberIDs(ctx, r.ID)
			summaries[i].Members = members
		}
	}
	return summaries, nil
}

func (s *Store) threadMemberIDs(ctx context.Context, threadID string) ([]string, error) {
	var ids []string
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(
			`SELECT user_id FROM chat_thread_members WHERE thread_id = ?`, threadID,
		).Scan(&ids).Error
	})
	return ids, err
}

// EnsurePublicMembership adds the user to every public channel they are not yet a member of.
// Called on chat page load so users automatically see all public channels.
func (s *Store) EnsurePublicMembership(ctx context.Context, userID string) error {
	return s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Exec(`
			INSERT OR IGNORE INTO chat_thread_members (thread_id, user_id, joined_at)
			SELECT id, ?, CURRENT_TIMESTAMP
			FROM chat_threads
			WHERE type = 'channel' AND is_private = 0
		`, userID).Error
	})
}

// ListPublicChannels returns all public channels (for the join/discover UI).
func (s *Store) ListPublicChannels(ctx context.Context) ([]Thread, error) {
	var rows []threadModel
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("type = 'channel' AND is_private = 0").Find(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	threads := make([]Thread, len(rows))
	for i, r := range rows {
		threads[i] = toThread(r)
	}
	return threads, nil
}

// Save inserts a new chat message.
func (s *Store) Save(ctx context.Context, msg Message) error {
	return s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Create(&msgModel{
			ID:        msg.ID,
			ThreadID:  msg.ThreadID,
			UserID:    msg.UserID,
			Username:  msg.Username,
			Body:      msg.Body,
			CreatedAt: msg.CreatedAt,
		}).Error
	})
}

// Recent returns the most recent limit messages for a thread in ascending order.
func (s *Store) Recent(ctx context.Context, threadID string, limit int) ([]Message, error) {
	var rows []msgModel
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT * FROM (
				SELECT id, thread_id, user_id, username, body, created_at
				FROM chat_messages
				WHERE thread_id = ?
				ORDER BY created_at DESC
				LIMIT ?
			) sub ORDER BY created_at ASC
		`, threadID, limit).Scan(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	msgs := make([]Message, len(rows))
	for i, r := range rows {
		msgs[i] = Message{ID: r.ID, ThreadID: r.ThreadID, UserID: r.UserID, Username: r.Username, Body: r.Body, CreatedAt: r.CreatedAt}
	}
	return msgs, nil
}

// MarkRead updates the last-read timestamp for a user in a thread.
func (s *Store) MarkRead(ctx context.Context, threadID, userID string) error {
	return s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Exec(
			`INSERT INTO chat_thread_reads (thread_id, user_id, last_read_at)
			 VALUES (?, ?, ?)
			 ON CONFLICT(thread_id, user_id) DO UPDATE SET last_read_at = excluded.last_read_at`,
			threadID, userID, time.Now().UTC(),
		).Error
	})
}

// TotalUnread returns total unread message count for a user across all threads.
func (s *Store) TotalUnread(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT COUNT(*) FROM chat_messages cm
			JOIN chat_thread_members m ON m.thread_id = cm.thread_id AND m.user_id = ?
			LEFT JOIN chat_thread_reads r ON r.thread_id = cm.thread_id AND r.user_id = ?
			WHERE r.last_read_at IS NULL OR cm.created_at > r.last_read_at
		`, userID, userID).Scan(&count).Error
	})
	return count, err
}

func toThread(r threadModel) Thread {
	return Thread{
		ID:        r.ID,
		Type:      r.Type,
		Name:      r.Name,
		IsPrivate: r.IsPrivate != 0,
		CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt,
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
