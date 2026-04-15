// Package chat provides persistent chat message storage.
package chat

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
)

// Message is the canonical chat message type used by both store and HTTP layer.
type Message struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type msgModel struct {
	ID        string    `gorm:"primaryKey;column:id"`
	UserID    string    `gorm:"column:user_id"`
	Username  string    `gorm:"column:username"`
	Body      string    `gorm:"column:body"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (msgModel) TableName() string { return "chat_messages" }

// Store handles chat_messages DB operations.
type Store struct {
	db *gormsqlite.DB
}

// NewStore creates a Store.
func NewStore(db *gormsqlite.DB) *Store { return &Store{db: db} }

// Save inserts a new chat message.
func (s *Store) Save(ctx context.Context, msg Message) error {
	return s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Create(&msgModel{
			ID:        msg.ID,
			UserID:    msg.UserID,
			Username:  msg.Username,
			Body:      msg.Body,
			CreatedAt: msg.CreatedAt,
		}).Error
	})
}

// Recent returns the most recent limit messages in ascending chronological order
// (oldest first so the chat renders top-to-bottom correctly).
func (s *Store) Recent(ctx context.Context, limit int) ([]Message, error) {
	var rows []msgModel
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Raw(`
			SELECT * FROM (
				SELECT id, user_id, username, body, created_at
				FROM chat_messages
				ORDER BY created_at DESC
				LIMIT ?
			) sub ORDER BY created_at ASC
		`, limit).Scan(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	msgs := make([]Message, len(rows))
	for i, r := range rows {
		msgs[i] = Message{ID: r.ID, UserID: r.UserID, Username: r.Username, Body: r.Body, CreatedAt: r.CreatedAt}
	}
	return msgs, nil
}
