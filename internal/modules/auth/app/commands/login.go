package commands

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/vvs/isp/internal/modules/auth/domain"
)

const SessionTTL = 24 * time.Hour

type LoginCommand struct {
	Username string
	Password string
}

type LoginResult struct {
	Token   string // raw token — goes into the cookie
	Session *domain.Session
	User    *domain.User
}

type LoginHandler struct {
	users    domain.UserRepository
	sessions domain.SessionRepository
}

func NewLoginHandler(users domain.UserRepository, sessions domain.SessionRepository) *LoginHandler {
	return &LoginHandler{users: users, sessions: sessions}
}

func (h *LoginHandler) Handle(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	u, err := h.users.FindByUsername(ctx, cmd.Username)
	if err != nil {
		return nil, domain.ErrInvalidPassword
	}
	if !u.VerifyPassword(cmd.Password) {
		return nil, domain.ErrInvalidPassword
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(raw)

	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])

	sess := domain.NewSession(u.ID, hash, SessionTTL)
	if err := h.sessions.Save(ctx, sess); err != nil {
		return nil, err
	}

	return &LoginResult{Token: token, Session: sess, User: u}, nil
}
