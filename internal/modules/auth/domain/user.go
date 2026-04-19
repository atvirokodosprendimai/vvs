package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUsernameRequired = errors.New("username is required")
	ErrPasswordRequired = errors.New("password is required")
	ErrInvalidRole      = errors.New("role must be admin or operator")
	ErrUserNotFound     = errors.New("user not found")
	ErrUsernameTaken    = errors.New("username already taken")
	ErrInvalidPassword  = errors.New("invalid username or password")
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

func ValidRole(r Role) bool {
	return r == RoleAdmin || r == RoleOperator || r == RoleViewer
}

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         Role
	FullName     string
	Division     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewUser(username, plainPassword string, role Role) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrUsernameRequired
	}
	if plainPassword == "" {
		return nil, ErrPasswordRequired
	}
	if !ValidRole(role) {
		return nil, ErrInvalidRole
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), 12)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &User{
		ID:           uuid.Must(uuid.NewV7()).String(),
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (u *User) VerifyPassword(plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plain)) == nil
}

func (u *User) ChangePassword(plain string) error {
	if plain == "" {
		return ErrPasswordRequired
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateProfile sets display name and division fields. Division may only be
// changed by an admin — callers are responsible for enforcing that guard.
func (u *User) UpdateProfile(fullName, division string) {
	u.FullName = strings.TrimSpace(fullName)
	u.Division = strings.TrimSpace(division)
	u.UpdatedAt = time.Now().UTC()
}

// ChangeRole updates the user's role. Callers must ensure the actor is admin.
func (u *User) ChangeRole(r Role) error {
	if !ValidRole(r) {
		return ErrInvalidRole
	}
	u.Role = r
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// CanWrite returns true for roles allowed to make mutations (admin + operator).
// Viewers are read-only and must be blocked at the API layer.
func (u *User) CanWrite() bool {
	return u.Role == RoleAdmin || u.Role == RoleOperator
}
