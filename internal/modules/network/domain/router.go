package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNameRequired   = errors.New("router name is required")
	ErrHostRequired   = errors.New("router host is required")
	ErrRouterNotFound = errors.New("router not found")
)

// Router stores connection details for a managed network device.
// Passwords are stored in plaintext — add AES-GCM encryption before production.
type Router struct {
	ID        string
	Name      string // human label, e.g. "Edge-01"
	Host      string // IP or hostname, e.g. "192.168.88.1"
	Port      int    // default 8728 (RouterOS API)
	Username  string
	Password  string
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewRouter(name, host string, port int, username, password, notes string) (*Router, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNameRequired
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, ErrHostRequired
	}
	if port <= 0 {
		port = 8728
	}

	now := time.Now().UTC()
	return &Router{
		ID:        uuid.Must(uuid.NewV7()).String(),
		Name:      name,
		Host:      host,
		Port:      port,
		Username:  strings.TrimSpace(username),
		Password:  password,
		Notes:     strings.TrimSpace(notes),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *Router) Update(name, host string, port int, username, password, notes string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameRequired
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ErrHostRequired
	}
	if port <= 0 {
		port = 8728
	}

	r.Name = name
	r.Host = host
	r.Port = port
	r.Username = strings.TrimSpace(username)
	if password != "" {
		r.Password = password
	}
	r.Notes = strings.TrimSpace(notes)
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// ToConn returns a RouterConn ready to pass to RouterProvisioner.
func (r *Router) ToConn() RouterConn {
	return RouterConn{
		RouterID: r.ID,
		Host:     r.Host,
		Port:     r.Port,
		Username: r.Username,
		Password: r.Password,
	}
}
