package domain

import "context"

type UserRepository interface {
	Save(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	ListAll(ctx context.Context) ([]*User, error)
	Delete(ctx context.Context, id string) error
}

type SessionRepository interface {
	Save(ctx context.Context, s *Session) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	DeleteByID(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
	PruneExpired(ctx context.Context) error
}
