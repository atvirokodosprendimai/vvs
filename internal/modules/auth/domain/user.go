package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUsernameRequired = errors.New("username is required")
	ErrPasswordRequired = errors.New("password is required")
	ErrUserNotFound     = errors.New("user not found")
	ErrUsernameTaken    = errors.New("username already taken")
	ErrInvalidPassword  = errors.New("invalid username or password")
)

type Role string

// Built-in role names — always present, cannot be deleted.
const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         Role
	IsWriteRole  bool // populated by persistence layer via JOIN with roles table
	FullName     string
	Division     string
	TOTPSecret   string
	TOTPEnabled  bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// EnableTOTP sets the TOTP secret and marks 2FA as active.
func (u *User) EnableTOTP(secret string) {
	u.TOTPSecret = secret
	u.TOTPEnabled = true
	u.UpdatedAt = time.Now().UTC()
}

// DisableTOTP clears the TOTP secret and turns off 2FA.
func (u *User) DisableTOTP() {
	u.TOTPSecret = ""
	u.TOTPEnabled = false
	u.UpdatedAt = time.Now().UTC()
}

// VerifyTOTP checks a 6-digit code against the user's TOTP secret.
// Returns false if TOTP is not enabled or the code is invalid.
func (u *User) VerifyTOTP(code string) bool {
	if !u.TOTPEnabled || u.TOTPSecret == "" {
		return false
	}
	return totp.Validate(code, u.TOTPSecret)
}

func NewUser(username, plainPassword string, role Role) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrUsernameRequired
	}
	if plainPassword == "" {
		return nil, ErrPasswordRequired
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
	u.Role = r
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// CanWrite returns true when the user's role permits mutations.
// Populated from the roles table via persistence JOIN — do not hardcode role names here.
func (u *User) CanWrite() bool {
	return u.IsWriteRole
}
