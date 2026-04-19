// Package notifications provides persistent per-user notification storage.
package notifications

import (
	"context"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
)

// Notification is the read view for a single notification entry.
type Notification struct {
	ID        string
	Title     string
	URL       string
	CreatedAt time.Time
	Read      bool // true if the current user has a notification_reads row for this
}

// notifModel mirrors the notifications table.
type notifModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Title     string    `gorm:"column:title"`
	URL       string    `gorm:"column:url"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (notifModel) TableName() string { return "notifications" }

// Store handles all notification DB operations.
type Store struct {
	db *gormsqlite.DB
}

// NewStore creates a Store backed by the shared gormsqlite DB.
func NewStore(db *gormsqlite.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new notification row.
func (s *Store) Create(ctx context.Context, id, title, url string) error {
	return s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Create(&notifModel{
			ID:        id,
			Title:     title,
			URL:       url,
			CreatedAt: time.Now().UTC(),
		}).Error
	})
}

// List returns the most recent notifications (up to limit) for userID with
// the Read flag set when a notification_reads row exists for that user.
func (s *Store) List(ctx context.Context, userID string, limit int) ([]Notification, error) {
	type row struct {
		ID        string    `gorm:"column:id"`
		Title     string    `gorm:"column:title"`
		URL       string    `gorm:"column:url"`
		CreatedAt time.Time `gorm:"column:created_at"`
		IsRead    int       `gorm:"column:is_read"`
	}
	var rows []row
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT n.id, n.title, n.url, n.created_at,
			       CASE WHEN nr.notification_id IS NOT NULL THEN 1 ELSE 0 END AS is_read
			FROM notifications n
			LEFT JOIN notification_reads nr
			       ON nr.notification_id = n.id AND nr.user_id = ?
			ORDER BY n.created_at DESC
			LIMIT ?
		`, userID, limit).Scan(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	notifs := make([]Notification, len(rows))
	for i, r := range rows {
		notifs[i] = Notification{
			ID: r.ID, Title: r.Title, URL: r.URL,
			CreatedAt: r.CreatedAt, Read: r.IsRead == 1,
		}
	}
	return notifs, nil
}

// UnreadCount returns the number of notifications the user has not yet read.
func (s *Store) UnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT COUNT(*) FROM notifications n
			WHERE NOT EXISTS (
				SELECT 1 FROM notification_reads nr
				WHERE nr.notification_id = n.id AND nr.user_id = ?
			)
		`, userID).Scan(&count).Error
	})
	return count, err
}

// MarkAllRead inserts notification_reads rows for every notification the user
// has not already acknowledged. Idempotent (INSERT OR IGNORE).
func (s *Store) MarkAllRead(ctx context.Context, userID string) error {
	return s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Exec(`
			INSERT OR IGNORE INTO notification_reads (user_id, notification_id)
			SELECT ?, id FROM notifications
		`, userID).Error
	})
}
