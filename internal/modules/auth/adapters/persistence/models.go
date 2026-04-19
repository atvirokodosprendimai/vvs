package persistence

import (
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

type RoleModel struct {
	Name        string `gorm:"primaryKey"`
	DisplayName string `gorm:"not null;default:''"`
	IsBuiltin   bool   `gorm:"not null;default:0"`
	CanWrite    bool   `gorm:"not null;default:1"`
}

func (RoleModel) TableName() string { return "roles" }

func roleToModel(r *domain.RoleDefinition) *RoleModel {
	return &RoleModel{
		Name:        string(r.Name),
		DisplayName: r.DisplayName,
		IsBuiltin:   r.IsBuiltin,
		CanWrite:    r.CanWrite,
	}
}

func roleToDomain(m *RoleModel) *domain.RoleDefinition {
	return &domain.RoleDefinition{
		Name:        domain.Role(m.Name),
		DisplayName: m.DisplayName,
		IsBuiltin:   m.IsBuiltin,
		CanWrite:    m.CanWrite,
	}
}

type UserModel struct {
	ID           string    `gorm:"primaryKey"`
	Username     string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	Role         string    `gorm:"not null"`
	FullName     string    `gorm:"not null;default:''"`
	Division     string    `gorm:"not null;default:''"`
	TOTPSecret   string    `gorm:"column:totp_secret;not null;default:''"`
	TOTPEnabled  bool      `gorm:"column:totp_enabled;not null;default:0"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (UserModel) TableName() string { return "users" }

type SessionModel struct {
	ID        string    `gorm:"primaryKey"`
	UserID    string    `gorm:"not null;index"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
}

func (SessionModel) TableName() string { return "sessions" }

func userToModel(u *domain.User) *UserModel {
	return &UserModel{
		ID:           u.ID,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		Role:         string(u.Role),
		FullName:     u.FullName,
		Division:     u.Division,
		TOTPSecret:   u.TOTPSecret,
		TOTPEnabled:  u.TOTPEnabled,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func userToDomain(m *UserModel, isWriteRole bool) *domain.User {
	return &domain.User{
		ID:           m.ID,
		Username:     m.Username,
		PasswordHash: m.PasswordHash,
		Role:         domain.Role(m.Role),
		IsWriteRole:  isWriteRole,
		FullName:     m.FullName,
		Division:     m.Division,
		TOTPSecret:   m.TOTPSecret,
		TOTPEnabled:  m.TOTPEnabled,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

// userWithRoleRow is a scan target for user + roles JOIN.
type userWithRoleRow struct {
	UserModel
	RoleCanWrite bool `gorm:"column:role_can_write"`
}

func sessionToModel(s *domain.Session) *SessionModel {
	return &SessionModel{
		ID:        s.ID,
		UserID:    s.UserID,
		TokenHash: s.TokenHash,
		CreatedAt: s.CreatedAt,
		ExpiresAt: s.ExpiresAt,
	}
}

func sessionToDomain(m *SessionModel) *domain.Session {
	return &domain.Session{
		ID:        m.ID,
		UserID:    m.UserID,
		TokenHash: m.TokenHash,
		CreatedAt: m.CreatedAt,
		ExpiresAt: m.ExpiresAt,
	}
}
