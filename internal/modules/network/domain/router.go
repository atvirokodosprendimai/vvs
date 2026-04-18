package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	RouterTypeMikroTik = "mikrotik"
	RouterTypeArista   = "arista"
)

var (
	ErrNameRequired       = errors.New("router name is required")
	ErrHostRequired       = errors.New("router host is required")
	ErrRouterNotFound     = errors.New("router not found")
	ErrInvalidRouterType  = errors.New("router type must be 'mikrotik' or 'arista'")
)

// Router stores connection details for a managed network device.
// Password is stored encrypted at rest via AES-256-GCM (see persistence layer).
type Router struct {
	ID         string
	Name       string // human label, e.g. "Edge-01"
	RouterType string // "mikrotik" | "arista"
	Host       string // IP or hostname
	Port       int    // default: 8728 (MikroTik) | 443 (Arista)
	Username   string
	Password   string
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewRouter(name, routerType, host string, port int, username, password, notes string) (*Router, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNameRequired
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, ErrHostRequired
	}
	routerType = normaliseRouterType(routerType)
	if routerType != RouterTypeMikroTik && routerType != RouterTypeArista {
		return nil, ErrInvalidRouterType
	}
	if port <= 0 {
		port = defaultPort(routerType)
	}

	now := time.Now().UTC()
	return &Router{
		ID:         uuid.Must(uuid.NewV7()).String(),
		Name:       name,
		RouterType: routerType,
		Host:       host,
		Port:       port,
		Username:   strings.TrimSpace(username),
		Password:   password,
		Notes:      strings.TrimSpace(notes),
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (r *Router) Update(name, routerType, host string, port int, username, password, notes string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameRequired
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ErrHostRequired
	}
	routerType = normaliseRouterType(routerType)
	if routerType != RouterTypeMikroTik && routerType != RouterTypeArista {
		return ErrInvalidRouterType
	}
	if port <= 0 {
		port = defaultPort(routerType)
	}

	r.Name = name
	r.RouterType = routerType
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
		RouterID:   r.ID,
		RouterType: r.RouterType,
		Host:       r.Host,
		Port:       r.Port,
		Username:   r.Username,
		Password:   r.Password,
	}
}

func normaliseRouterType(t string) string {
	t = strings.TrimSpace(strings.ToLower(t))
	if t == "" {
		return RouterTypeMikroTik
	}
	return t
}

func defaultPort(routerType string) int {
	if routerType == RouterTypeArista {
		return 443
	}
	return 8728
}
